package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h *Handler) AddSeat(c *gin.Context) {
	idText := c.Param("id")

	eventID, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}
	seat, found := h.service.AddSeat(uint(eventID))

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}
	c.JSON(http.StatusCreated, seat)
}
func (h *Handler) ListSeatsByEvent(c *gin.Context) {
	idText := c.Param("id")

	eventID, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	seats, found := h.service.ListSeatsByEvent(uint(eventID))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event Not Found"})
		return
	}
	c.JSON(http.StatusOK, seats)
}
