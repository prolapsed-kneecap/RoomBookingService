package router

import (
	"net/http"

	"booking-service/internal/handler"
	"booking-service/internal/middleware"
)

func New(h *handler.Handler, jwtSecret string) http.Handler {
	mux := http.NewServeMux()

	authMiddleware := middleware.Auth(jwtSecret)

	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/rooms", h.GetRooms)
	mux.HandleFunc("GET /api/rooms/{id}/schedule", h.GetRoomSchedule)

	mux.Handle("POST /api/rooms", authMiddleware(http.HandlerFunc(h.CreateRoom)))
	mux.Handle("POST /api/bookings", authMiddleware(http.HandlerFunc(h.CreateBooking)))
	mux.Handle("GET /api/user/bookings", authMiddleware(http.HandlerFunc(h.GetUserBookings)))
	mux.Handle("DELETE /api/bookings/{id}", authMiddleware(http.HandlerFunc(h.DeleteBooking)))

	var root http.Handler = mux
	root = middleware.Logger(root)
	root = middleware.Recover(root)

	return root
}
