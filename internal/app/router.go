package app

import (
	"github.com/gin-gonic/gin"

	"seatrush/internal/handler"
)

func NewRouter(h *handler.Handler) *gin.Engine {
	router := gin.New()

	router.POST("/reservations", h.CreateReservation)
	router.GET("/reservations/:id", h.GetReservation)

	router.POST("/events", h.CreateEvent)
	router.GET("/events/:id", h.GetEvent)
	router.GET("/events", h.ListEvents)

	router.POST("/events/:id/seats", h.AddSeat)
	router.GET("/events/:id/seats", h.ListSeatsByEvent)

	router.GET("/", h.Health)

	return router
}
