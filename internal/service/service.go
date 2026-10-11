package service

import (
	"context"
	"time"

	"booking-service/internal/auth"
	"booking-service/internal/domain"
)

type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	CreateRoom(ctx context.Context, name string, capacity int) (*domain.Room, error)
	GetRooms(ctx context.Context) ([]domain.Room, error)
	GetRoomByID(ctx context.Context, id int) (*domain.Room, error)
	CreateBookingTx(ctx context.Context, roomID, userID int, start, end time.Time) (*domain.Booking, error)
	GetRoomSchedule(ctx context.Context, roomID int, dayStart, dayEnd time.Time) ([]domain.Booking, error)
	GetUserBookings(ctx context.Context, userID int) ([]domain.Booking, error)
	DeleteBooking(ctx context.Context, bookingID, userID int) error
}

type Service struct {
	store     Store
	jwtSecret string
}

func New(store Store, jwtSecret string) *Service {
	return &Service{
		store:     store,
		jwtSecret: jwtSecret,
	}
}

func (s *Service) Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
	if req.Email == "" || len(req.Password) < 6 {
		return nil, domain.ErrInvalidRequest
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, domain.ErrInternal
	}

	u, err := s.store.CreateUser(ctx, req.Email, hash)
	if err != nil {
		return nil, domain.ErrConflict 
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, req domain.LoginRequest) (string, error) {
	u, err := s.store.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return "", domain.ErrUnauthorized
	}

	if !auth.CheckPasswordHash(req.Password, u.PasswordHash) {
		return "", domain.ErrUnauthorized
	}

	token, err := auth.GenerateToken(u.ID, s.jwtSecret)
	if err != nil {
		return "", domain.ErrInternal
	}

	return token, nil
}

func (s *Service) CreateRoom(ctx context.Context, req domain.CreateRoomRequest) (*domain.Room, error) {
	if req.Name == "" || req.Capacity <= 0 {
		return nil, domain.ErrInvalidRequest
	}

	return s.store.CreateRoom(ctx, req.Name, req.Capacity)
}

func (s *Service) GetRooms(ctx context.Context) ([]domain.Room, error) {
	return s.store.GetRooms(ctx)
}

func (s *Service) CreateBooking(ctx context.Context, userID int, req domain.CreateBookingRequest) (*domain.Booking, error) {
	if req.EndTime.Before(req.StartTime) || req.EndTime.Equal(req.StartTime) {
		return nil, domain.ErrInvalidRequest
	}

	if req.StartTime.Before(time.Now()) {
		return nil, domain.ErrInvalidRequest
	}

	_, err := s.store.GetRoomByID(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}

	return s.store.CreateBookingTx(ctx, req.RoomID, userID, req.StartTime, req.EndTime)
}

func (s *Service) GetRoomSchedule(ctx context.Context, roomID int, date time.Time) ([]domain.Booking, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	return s.store.GetRoomSchedule(ctx, roomID, dayStart, dayEnd)
}

func (s *Service) GetUserBookings(ctx context.Context, userID int) ([]domain.Booking, error) {
	return s.store.GetUserBookings(ctx, userID)
}

func (s *Service) DeleteBooking(ctx context.Context, bookingID, userID int) error {
	return s.store.DeleteBooking(ctx, bookingID, userID)
}
