package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"seatrush/internal/domain"
	"seatrush/internal/repository"
)

type Seat = domain.Seat
type Event = domain.Event
type User = domain.User
type Reservation = domain.Reservation

const (
	StatusAvailable = domain.StatusAvailable
	StatusHeld      = domain.StatusHeld
	StatusBooked    = domain.StatusBooked
)

const (
	ReservationStatusPending   = domain.ReservationStatusPending
	ReservationStatusConfirmed = domain.ReservationStatusConfirmed
)

type CreateEventRequest struct {
	Name string `json:"name"`
}

type CreateReservationRequest struct {
	UserID  uint `json:"user_id"`
	EventID uint `json:"event_id"`
	SeatID  uint `json:"seat_id"`
}

func setupRouter(events []Event, seats []Seat, reservations []Reservation, nextEventID, nextSeatID, nextReservationID uint) *gin.Engine {
	repo := repository.NewMemoryRepository(
		events,
		seats,
		reservations,
		nextEventID,
		nextSeatID,
		nextReservationID,
	)
	router := gin.New()

	router.POST("/reservations", func(c *gin.Context) {
		var input CreateReservationRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON",
			})
			return
		}
		seatFound, reservation := repo.HoldSeat(
			input.UserID,
			input.EventID,
			input.SeatID,
		)

		if !seatFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Seat not found",
			})
			return
		}
		if reservation.ID == 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Seat unavailable",
			})
			return
		}

		c.JSON(http.StatusCreated, reservation)
	})

	router.GET("/reservations/:id", func(c *gin.Context) {
		idText := c.Param("id")

		id, err := strconv.ParseUint(idText, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
			return
		}
		foundReservation, found := repo.GetReservationByID(uint(id))
		if found {
			c.JSON(http.StatusOK, foundReservation)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
	})

	router.POST("/events", func(c *gin.Context) {
		var input CreateEventRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON",
			})
			return
		}
		if input.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Name is required",
			})
			return
		}

		newEvent := repo.CreateEvent(input.Name)
		c.JSON(http.StatusCreated, newEvent)
	})
	router.GET("/events/:id", func(c *gin.Context) {
		idText := c.Param("id")

		id, err := strconv.ParseUint(idText, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid event ID",
			})
			return
		}
		foundEvent, found := repo.GetEventByID(uint(id))
		if found {
			c.JSON(http.StatusOK, foundEvent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Event not found",
		})
	})
	router.POST("/events/:id/seats", func(c *gin.Context) {
		idText := c.Param("id")

		eventID, err := strconv.ParseUint(idText, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
			return
		}
		_, eventFound := repo.GetEventByID(uint(eventID))
		if !eventFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		newSeat := repo.AddSeat(uint(eventID))

		c.JSON(http.StatusCreated, newSeat)

	})
	router.GET("/events/:id/seats", func(c *gin.Context) {
		idText := c.Param("id")

		eventID, err := strconv.ParseUint(idText, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event id"})
			return
		}
		_, eventFound := repo.GetEventByID(uint(eventID))
		if !eventFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		eventSeats := repo.ListSeatsByEvent(uint(eventID))
		c.JSON(http.StatusOK, eventSeats)
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "SeatRush api is running",
		})
	})
	router.GET("/events", func(c *gin.Context) {
		currentEvents := repo.ListEvents()
		c.JSON(http.StatusOK, currentEvents)
	})
	return router
}

func main() {
	fmt.Println("Welcome to SeatRush")

	event := Event{
		ID:   101,
		Name: "Tech Conference",
	}
	user := User{
		ID:    1,
		Name:  "John",
		Email: "John@gmail.com",
	}

	fmt.Println("Users:", user)
	fmt.Println(event)

	events := []Event{event}
	nextEventID := uint(102)

	seats := []Seat{
		{ID: 1, EventID: 101, Status: StatusAvailable},
		{ID: 2, EventID: 101, Status: StatusAvailable},
	}
	nextSeatID := uint(3)

	reservations := []Reservation{}
	nextReservationID := uint(501)
	router := setupRouter(events, seats, reservations, nextEventID, nextSeatID, nextReservationID)

	fmt.Println("Gin server is running on :8080")
	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
