package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"booking-service/internal/domain"
	"booking-service/internal/handler"
	"booking-service/internal/middleware"
)

type mockService struct {
	registerFn      func(ctx context.Context, req domain.RegisterRequest) (*domain.User, error)
	loginFn         func(ctx context.Context, req domain.LoginRequest) (string, error)
	createRoomFn    func(ctx context.Context, req domain.CreateRoomRequest) (*domain.Room, error)
	getRoomsFn      func(ctx context.Context) ([]domain.Room, error)
	createBookingFn func(ctx context.Context, userID int, req domain.CreateBookingRequest) (*domain.Booking, error)
	deleteBookingFn func(ctx context.Context, bookingID, userID int) error
}

func (m *mockService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, req)
	}
	return nil, nil
}

func (m *mockService) Login(ctx context.Context, req domain.LoginRequest) (string, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, req)
	}
	return "", nil
}

func (m *mockService) CreateRoom(ctx context.Context, req domain.CreateRoomRequest) (*domain.Room, error) {
	if m.createRoomFn != nil {
		return m.createRoomFn(ctx, req)
	}
	return nil, nil
}

func (m *mockService) GetRooms(ctx context.Context) ([]domain.Room, error) {
	if m.getRoomsFn != nil {
		return m.getRoomsFn(ctx)
	}
	return []domain.Room{}, nil
}

func (m *mockService) CreateBooking(ctx context.Context, userID int, req domain.CreateBookingRequest) (*domain.Booking, error) {
	if m.createBookingFn != nil {
		return m.createBookingFn(ctx, userID, req)
	}
	return nil, nil
}

func (m *mockService) GetRoomSchedule(ctx context.Context, roomID int, date time.Time) ([]domain.Booking, error) {
	return []domain.Booking{}, nil
}

func (m *mockService) GetUserBookings(ctx context.Context, userID int) ([]domain.Booking, error) {
	return []domain.Booking{}, nil
}

func (m *mockService) DeleteBooking(ctx context.Context, bookingID, userID int) error {
	if m.deleteBookingFn != nil {
		return m.deleteBookingFn(ctx, bookingID, userID)
	}
	return nil
}

func TestHandler_Register(t *testing.T) {
	svc := &mockService{
		registerFn: func(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
			if req.Email == "duplicate@example.com" {
				return nil, domain.ErrConflict
			}
			return &domain.User{ID: 1, Email: req.Email}, nil
		},
	}
	h := handler.New(svc)

	t.Run("status created", func(t *testing.T) {
		body, _ := json.Marshal(domain.RegisterRequest{Email: "test@example.com", Password: "password123"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
		}
	})

	t.Run("status conflict", func(t *testing.T) {
		body, _ := json.Marshal(domain.RegisterRequest{Email: "duplicate@example.com", Password: "password123"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
		}
	})
}

func TestHandler_Login(t *testing.T) {
	svc := &mockService{
		loginFn: func(ctx context.Context, req domain.LoginRequest) (string, error) {
			if req.Email == "test@example.com" && req.Password == "correct" {
				return "jwt-token-xyz", nil
			}
			return "", domain.ErrUnauthorized
		},
	}
	h := handler.New(svc)

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(domain.LoginRequest{Email: "test@example.com", Password: "correct"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		body, _ := json.Marshal(domain.LoginRequest{Email: "test@example.com", Password: "wrong"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

func TestHandler_CreateBooking(t *testing.T) {
	svc := &mockService{
		createBookingFn: func(ctx context.Context, userID int, req domain.CreateBookingRequest) (*domain.Booking, error) {
			if req.RoomID == 409 {
				return nil, domain.ErrConflict
			}
			return &domain.Booking{ID: 1, RoomID: req.RoomID, UserID: userID}, nil
		},
	}
	h := handler.New(svc)

	t.Run("unauthorized without context", func(t *testing.T) {
		body, _ := json.Marshal(domain.CreateBookingRequest{RoomID: 1})
		req := httptest.NewRequest(http.MethodPost, "/api/bookings", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.CreateBooking(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("conflict error", func(t *testing.T) {
		body, _ := json.Marshal(domain.CreateBookingRequest{RoomID: 409})
		req := httptest.NewRequest(http.MethodPost, "/api/bookings", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 10)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.CreateBooking(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
		}
	})

	t.Run("booking created", func(t *testing.T) {
		body, _ := json.Marshal(domain.CreateBookingRequest{RoomID: 1})
		req := httptest.NewRequest(http.MethodPost, "/api/bookings", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 10)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.CreateBooking(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
		}
	})
}
