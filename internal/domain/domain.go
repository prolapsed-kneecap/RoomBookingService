package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrConflict       = errors.New("conflict")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidRequest = errors.New("invalid request")
	ErrInternal       = errors.New("internal server error")
)

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Room struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Capacity  int       `json:"capacity"`
	CreatedAt time.Time `json:"created_at"`
}

type Booking struct {
	ID        int       `json:"id"`
	RoomID    int       `json:"room_id"`
	UserID    int       `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type CreateRoomRequest struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
}

type CreateBookingRequest struct {
	RoomID    int       `json:"room_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}
