package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	StatusAvailable = "AVAILABLE"
	StatusHeld      = "HELD"
	StatusBooked    = "BOOKED"
)

const ReservationStatusPending = "PENDING"
const ReservationStatusConfirmed = "CONFIRMED"

type Seat struct {
	ID      uint
	EventID uint
	Status  string
}

type Event struct {
	ID   uint
	Name string
}
type User struct {
	ID    uint
	Name  string
	Email string
}
type Reservation struct {
	ID      uint
	UserID  uint
	EventID uint
	SeatID  uint
	Status  string
}
type CreateEventRequest struct {
	Name string `json:"name"`
}

type CreateReservationRequest struct {
	UserID  uint `json:"user_id"`
	EventID uint `json:"event_id"`
	SeatID  uint `json:"seat_id"`
}

type App struct {
	events            []Event
	seats             []Seat
	reservations      []Reservation
	nextEventID       uint
	nextSeatID        uint
	nextReservationID uint
	mu                sync.RWMutex
}

func (app *App) setupRouter() *gin.Engine {
	router := gin.New()

	router.POST("/reservations", func(c *gin.Context) {
		var input CreateReservationRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON",
			})
			return
		}
		seatFound := false
		seatHeld := false

		app.mu.Lock()
		for i := range app.seats {
			if app.seats[i].ID == input.SeatID && app.seats[i].EventID == input.EventID {
				seatFound = true

				if app.seats[i].Status == StatusAvailable {
					app.seats[i].Status = StatusHeld
					seatHeld = true
				}
				break
			}
		}
		var reservation Reservation
		if seatHeld {
			reservation = Reservation{
				ID:      app.nextReservationID,
				UserID:  input.UserID,
				EventID: input.EventID,
				SeatID:  input.SeatID,
				Status:  ReservationStatusPending,
			}
			app.nextReservationID++
			app.reservations = append(app.reservations, reservation)
		}
		app.mu.Unlock()
		fmt.Println("Seat found", seatFound)
		fmt.Println("Seat held", seatHeld)

		if !seatFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Seat not found",
			})
			return
		}
		if !seatHeld {
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
		app.mu.RLock()
		defer app.mu.RUnlock()
		for _, reservation := range app.reservations {
			if reservation.ID == uint(id) {
				c.JSON(http.StatusOK, reservation)
				return
			}
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

		app.mu.Lock()
		defer app.mu.Unlock()

		newEvent := Event{
			ID:   app.nextEventID,
			Name: input.Name,
		}
		app.nextEventID++
		app.events = append(app.events, newEvent)
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
		app.mu.RLock()
		defer app.mu.RUnlock()
		for _, event := range app.events {
			if event.ID == uint(id) {
				c.JSON(http.StatusOK, event)
				return
			}
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
		app.mu.Lock()
		defer app.mu.Unlock()
		eventFound := false

		for _, event := range app.events {
			if event.ID == uint(eventID) {
				eventFound = true
				break
			}
		}
		if !eventFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		newSeat := Seat{
			ID:      app.nextSeatID,
			EventID: uint(eventID),
			Status:  StatusAvailable,
		}

		app.nextSeatID++
		app.seats = append(app.seats, newSeat)

		c.JSON(http.StatusCreated, newSeat)

	})
	router.GET("/events/:id/seats", func(c *gin.Context) {
		idText := c.Param("id")

		eventID, err := strconv.ParseUint(idText, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event id"})
			return
		}
		app.mu.RLock()
		defer app.mu.RUnlock()
		eventFound := false

		for _, event := range app.events {
			if event.ID == uint(eventID) {
				eventFound = true
				break
			}
		}
		if !eventFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		eventSeats := []Seat{}

		for _, seat := range app.seats {
			if seat.EventID == uint(eventID) {
				eventSeats = append(eventSeats, seat)
			}
		}
		c.JSON(http.StatusOK, eventSeats)
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "SeatRush api is running",
		})
	})
	router.GET("/events", func(c *gin.Context) {
		app.mu.RLock()
		defer app.mu.RUnlock()

		c.JSON(http.StatusOK, app.events)
	})
	return router
}

func main() {
	app := &App{
		events: []Event{
			{ID: 101, Name: "Tech Conference"},
		},
		seats: []Seat{
			{ID: 1, EventID: 101, Status: StatusAvailable},
			{ID: 2, EventID: 101, Status: StatusAvailable},
		},
		reservations:      []Reservation{},
		nextEventID:       102,
		nextSeatID:        3,
		nextReservationID: 501,
	}

	if err := app.setupRouter().Run(":8080"); err != nil {
		panic(err)
	}
}
