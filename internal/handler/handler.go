package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"seatrush/internal/domain"
)

type Handler struct {
	service Service
}
type Service interface {
	ReserveSeat(userID, eventID, seatID uint) (bool, domain.Reservation)
	GetReservationByID(id uint) (domain.Reservation, bool)
	CreateEvent(name string) (domain.Event, bool)
	GetEventByID(id uint) (domain.Event, bool)
	ListEvents() []domain.Event
	AddSeat(eventID uint) (domain.Seat, bool)
	ListSeatsByEvent(eventID uint) ([]domain.Seat, bool)
}

func New(svc Service) *Handler {
	return &Handler{

		service: svc,
	}
}
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "SeatRush is running"})
}
