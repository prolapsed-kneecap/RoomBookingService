package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"booking-service/internal/domain"
	"booking-service/internal/middleware"
)

type Service interface {
	Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error)
	Login(ctx context.Context, req domain.LoginRequest) (string, error)
	CreateRoom(ctx context.Context, req domain.CreateRoomRequest) (*domain.Room, error)
	GetRooms(ctx context.Context) ([]domain.Room, error)
	CreateBooking(ctx context.Context, userID int, req domain.CreateBookingRequest) (*domain.Booking, error)
	GetRoomSchedule(ctx context.Context, roomID int, date time.Time) ([]domain.Booking, error)
	GetUserBookings(ctx context.Context, userID int) ([]domain.Booking, error)
	DeleteBooking(ctx context.Context, bookingID, userID int) error
}

type Handler struct {
	service Service
}

func New(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	user, err := h.service.Register(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	token, err := h.service.Login(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, domain.TokenResponse{Token: token})
}

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	room, err := h.service.CreateRoom(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, room)
}

func (h *Handler) GetRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.service.GetRooms(r.Context())
	if err != nil {
		h.handleError(w, err)
		return
	}

	if rooms == nil {
		rooms = []domain.Room{}
	}

	h.writeJSON(w, http.StatusOK, rooms)
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, domain.ErrUnauthorized)
		return
	}

	var req domain.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	booking, err := h.service.CreateBooking(r.Context(), userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, booking)
}

func (h *Handler) GetRoomSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	roomID, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	schedule, err := h.service.GetRoomSchedule(r.Context(), roomID, date)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if schedule == nil {
		schedule = []domain.Booking{}
	}

	h.writeJSON(w, http.StatusOK, schedule)
}

func (h *Handler) GetUserBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, domain.ErrUnauthorized)
		return
	}

	bookings, err := h.service.GetUserBookings(r.Context(), userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if bookings == nil {
		bookings = []domain.Booking{}
	}

	h.writeJSON(w, http.StatusOK, bookings)
}

func (h *Handler) DeleteBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, domain.ErrUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	bookingID, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, domain.ErrInvalidRequest)
		return
	}

	if err := h.service.DeleteBooking(r.Context(), bookingID, userID); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		h.writeError(w, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrConflict):
		h.writeError(w, http.StatusConflict, err)
	case errors.Is(err, domain.ErrUnauthorized):
		h.writeError(w, http.StatusUnauthorized, err)
	case errors.Is(err, domain.ErrInvalidRequest):
		h.writeError(w, http.StatusBadRequest, err)
	default:
		h.writeError(w, http.StatusInternalServerError, domain.ErrInternal)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, err error) {
	h.writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
