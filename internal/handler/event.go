package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CreateEventRequest struct {
	Name string `json:"name"`
}

func (h *Handler) CreateEvent(c *gin.Context) {
	var input CreateEventRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	event, ok := h.service.CreateEvent(input.Name)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}
	c.JSON(http.StatusCreated, event)
}
func (h *Handler) GetEvent(c *gin.Context) {
	idText := c.Param("id")

	id, err := strconv.ParseUint(idText, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}
	event, found := h.service.GetEventByID(uint(id))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}
	c.JSON(http.StatusOK, event)
}
func (h *Handler) ListEvents(c *gin.Context) {
	events := h.service.ListEvents()
	c.JSON(http.StatusOK, events)
}
