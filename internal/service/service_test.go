package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"booking-service/internal/auth"
	"booking-service/internal/domain"
	"booking-service/internal/service"
)

type mockStore struct {
	createUserFn      func(ctx context.Context, email, hash string) (*domain.User, error)
	getUserByEmailFn  func(ctx context.Context, email string) (*domain.User, error)
	getRoomByIDFn     func(ctx context.Context, id int) (*domain.Room, error)
	createBookingTxFn func(ctx context.Context, roomID, userID int, start, end time.Time) (*domain.Booking, error)
}

func (m *mockStore) CreateUser(ctx context.Context, email, hash string) (*domain.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, email, hash)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return nil, domain.ErrNotFound
}

func (m *mockStore) CreateRoom(ctx context.Context, name string, capacity int) (*domain.Room, error) {
	return nil, nil
}

func (m *mockStore) GetRooms(ctx context.Context) ([]domain.Room, error) {
	return nil, nil
}

func (m *mockStore) GetRoomByID(ctx context.Context, id int) (*domain.Room, error) {
	if m.getRoomByIDFn != nil {
		return m.getRoomByIDFn(ctx, id)
	}
	return &domain.Room{ID: id, Name: "Main", Capacity: 10}, nil
}

func (m *mockStore) CreateBookingTx(ctx context.Context, roomID, userID int, start, end time.Time) (*domain.Booking, error) {
	if m.createBookingTxFn != nil {
		return m.createBookingTxFn(ctx, roomID, userID, start, end)
	}
	return &domain.Booking{ID: 1, RoomID: roomID, UserID: userID, StartTime: start, EndTime: end}, nil
}

func (m *mockStore) GetRoomSchedule(ctx context.Context, roomID int, start, end time.Time) ([]domain.Booking, error) {
	return nil, nil
}

func (m *mockStore) GetUserBookings(ctx context.Context, userID int) ([]domain.Booking, error) {
	return nil, nil
}

func (m *mockStore) DeleteBooking(ctx context.Context, bookingID, userID int) error {
	return nil
}

func TestService_Register(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.RegisterRequest
		mockFn  func(ctx context.Context, email, hash string) (*domain.User, error)
		wantErr error
	}{
		{
			name: "success",
			req: domain.RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockFn: func(ctx context.Context, email, hash string) (*domain.User, error) {
				return &domain.User{ID: 1, Email: email}, nil
			},
			wantErr: nil,
		},
		{
			name: "short password",
			req: domain.RegisterRequest{
				Email:    "test@example.com",
				Password: "123",
			},
			wantErr: domain.ErrInvalidRequest,
		},
		{
			name: "empty email",
			req: domain.RegisterRequest{
				Email:    "",
				Password: "password123",
			},
			wantErr: domain.ErrInvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockStore{createUserFn: tt.mockFn}
			svc := service.New(store, "secret")

			u, err := svc.Register(context.Background(), tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if tt.wantErr == nil && u == nil {
				t.Fatal("expected user, got nil")
			}
		})
	}
}

func TestService_Login(t *testing.T) {
	hash, err := auth.HashPassword("valid-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := &mockStore{
		getUserByEmailFn: func(ctx context.Context, email string) (*domain.User, error) {
			if email == "user@example.com" {
				return &domain.User{ID: 42, Email: email, PasswordHash: hash}, nil
			}
			return nil, domain.ErrNotFound
		},
	}

	svc := service.New(store, "secret")

	t.Run("success", func(t *testing.T) {
		token, err := svc.Login(context.Background(), domain.LoginRequest{
			Email:    "user@example.com",
			Password: "valid-password",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := svc.Login(context.Background(), domain.LoginRequest{
			Email:    "user@example.com",
			Password: "wrong-password",
		})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, err := svc.Login(context.Background(), domain.LoginRequest{
			Email:    "absent@example.com",
			Password: "valid-password",
		})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestService_CreateBooking(t *testing.T) {
	now := time.Now().Add(1 * time.Hour)

	store := &mockStore{
		createBookingTxFn: func(ctx context.Context, roomID, userID int, start, end time.Time) (*domain.Booking, error) {
			if roomID == 99 {
				return nil, domain.ErrConflict
			}
			return &domain.Booking{ID: 1, RoomID: roomID, UserID: userID, StartTime: start, EndTime: end}, nil
		},
	}

	svc := service.New(store, "secret")

	t.Run("invalid time interval", func(t *testing.T) {
		_, err := svc.CreateBooking(context.Background(), 1, domain.CreateBookingRequest{
			RoomID:    1,
			StartTime: now.Add(2 * time.Hour),
			EndTime:   now,
		})
		if !errors.Is(err, domain.ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})

	t.Run("booking in the past", func(t *testing.T) {
		_, err := svc.CreateBooking(context.Background(), 1, domain.CreateBookingRequest{
			RoomID:    1,
			StartTime: time.Now().Add(-2 * time.Hour),
			EndTime:   time.Now().Add(-1 * time.Hour),
		})
		if !errors.Is(err, domain.ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})

	t.Run("conflict error propagation", func(t *testing.T) {
		_, err := svc.CreateBooking(context.Background(), 1, domain.CreateBookingRequest{
			RoomID:    99,
			StartTime: now,
			EndTime:   now.Add(1 * time.Hour),
		})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("expected ErrConflict, got %v", err)
		}
	})
}
