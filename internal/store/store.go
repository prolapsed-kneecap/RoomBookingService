package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"booking-service/internal/config"
	"booking-service/internal/domain"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func New(cfg *config.DBConfig) (*Store, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS rooms (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		capacity INT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS bookings (
		id SERIAL PRIMARY KEY,
		room_id INT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
		user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		start_time TIMESTAMPTZ NOT NULL,
		end_time TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_bookings_room_time ON bookings (room_id, start_time, end_time);
	`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*domain.User, error) {
	query := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at`

	u := &domain.User{}
	err := s.db.QueryRowContext(ctx, query, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, created_at FROM users WHERE email = $1`

	u := &domain.User{}
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) CreateRoom(ctx context.Context, name string, capacity int) (*domain.Room, error) {
	query := `
		INSERT INTO rooms (name, capacity)
		VALUES ($1, $2)
		RETURNING id, name, capacity, created_at`

	r := &domain.Room{}
	err := s.db.QueryRowContext(ctx, query, name, capacity).Scan(
		&r.ID, &r.Name, &r.Capacity, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) GetRooms(ctx context.Context) ([]domain.Room, error) {
	query := `SELECT id, name, capacity, created_at FROM rooms ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []domain.Room
	for rows.Next() {
		var r domain.Room
		if err := rows.Scan(&r.ID, &r.Name, &r.Capacity, &r.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rooms, nil
}

func (s *Store) GetRoomByID(ctx context.Context, id int) (*domain.Room, error) {
	query := `SELECT id, name, capacity, created_at FROM rooms WHERE id = $1`

	r := &domain.Room{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID, &r.Name, &r.Capacity, &r.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) CreateBookingTx(ctx context.Context, roomID, userID int, start, end time.Time) (*domain.Booking, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var dummy int
	err = tx.QueryRowContext(ctx, `SELECT id FROM rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock room: %w", err)
	}

	var count int
	overlapQuery := `
		SELECT COUNT(*)
		FROM bookings
		WHERE room_id = $1
		  AND start_time < $2
		  AND end_time > $3`

	if err := tx.QueryRowContext(ctx, overlapQuery, roomID, end, start).Scan(&count); err != nil {
		return nil, fmt.Errorf("check overlap: %w", err)
	}
	if count > 0 {
		return nil, domain.ErrConflict
	}

	insertQuery := `
		INSERT INTO bookings (room_id, user_id, start_time, end_time)
		VALUES ($1, $2, $3, $4)
		RETURNING id, room_id, user_id, start_time, end_time, created_at`

	b := &domain.Booking{}
	err = tx.QueryRowContext(ctx, insertQuery, roomID, userID, start, end).Scan(
		&b.ID, &b.RoomID, &b.UserID, &b.StartTime, &b.EndTime, &b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert booking: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return b, nil
}

func (s *Store) GetRoomSchedule(ctx context.Context, roomID int, dayStart, dayEnd time.Time) ([]domain.Booking, error) {
	query := `
		SELECT id, room_id, user_id, start_time, end_time, created_at
		FROM bookings
		WHERE room_id = $1
		  AND start_time < $3
		  AND end_time > $2
		ORDER BY start_time ASC`

	rows, err := s.db.QueryContext(ctx, query, roomID, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []domain.Booking
	for rows.Next() {
		var b domain.Booking
		if err := rows.Scan(&b.ID, &b.RoomID, &b.UserID, &b.StartTime, &b.EndTime, &b.CreatedAt); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bookings, nil
}

func (s *Store) GetUserBookings(ctx context.Context, userID int) ([]domain.Booking, error) {
	query := `
		SELECT id, room_id, user_id, start_time, end_time, created_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY start_time ASC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []domain.Booking
	for rows.Next() {
		var b domain.Booking
		if err := rows.Scan(&b.ID, &b.RoomID, &b.UserID, &b.StartTime, &b.EndTime, &b.CreatedAt); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bookings, nil
}

func (s *Store) DeleteBooking(ctx context.Context, bookingID, userID int) error {
	var ownerID int
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM bookings WHERE id = $1`, bookingID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}

	if ownerID != userID {
		return domain.ErrUnauthorized
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM bookings WHERE id = $1`, bookingID)
	return err
}
