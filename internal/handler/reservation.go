package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CreateReservationRequest struct {
	UserID  uint `json:"user_id"`
	EventID uint `json:"event_id"`
	SeatID  uint `json:"seat_id"`
}

func (h *Handler) CreateReservation(c *gin.Context) {
	var input CreateReservationRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	seatFound, reservation := h.service.ReserveSeat(input.UserID, input.EventID, input.SeatID)

	if !seatFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "Seat not found"})
		return
	}

	if reservation.ID == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Seat unavailable"})
		return
	}
	c.JSON(http.StatusCreated, reservation)
}
func (h *Handler) GetReservation(c *gin.Context) {
	idText := c.Param("id")

	id, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}
	reservation, found := h.service.GetReservationByID(uint(id))

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
		return
	}
	c.JSON(http.StatusOK, reservation)
}
