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

func holdSeat(mu *sync.RWMutex, seats []Seat, reservations *[]Reservation, nextReservationID *uint, input CreateReservationRequest) (bool, Reservation) {
	mu.Lock()
	defer mu.Unlock()

	for i := range seats {
		if seats[i].ID != input.SeatID || seats[i].EventID != input.EventID {
			continue
		}
		if seats[i].Status != StatusAvailable {
			return true, Reservation{}
		}

		seats[i].Status = StatusHeld
		reservation := Reservation{
			ID:      *nextReservationID,
			UserID:  input.UserID,
			EventID: input.EventID,
			SeatID:  input.SeatID,
			Status:  ReservationStatusPending,
		}
		*nextReservationID = *nextReservationID + 1
		*reservations = append(*reservations, reservation)
		return true, reservation
	}

	return false, Reservation{}
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
	var mu sync.RWMutex

	//routes

	router := gin.New()

	router.POST("/reservations", func(c *gin.Context) {
		var input CreateReservationRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON",
			})
			return
		}
		seatFound, reservation := holdSeat(&mu, seats, &reservations, &nextReservationID, input)
		seatHeld := reservation.ID != 0
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
		mu.RLock()
		var foundReservation Reservation
		for _, reservation := range reservations {
			if reservation.ID == uint(id) {
				foundReservation = reservation
				break
			}
		}
		mu.RUnlock()
		if foundReservation.ID != 0 {
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

		mu.Lock()
		newEvent := Event{
			ID:   nextEventID,
			Name: input.Name,
		}
		nextEventID++
		events = append(events, newEvent)
		mu.Unlock()
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
		mu.RLock()
		var foundEvent Event
		for _, event := range events {
			if event.ID == uint(id) {
				foundEvent = event
				break
			}
		}
		mu.RUnlock()
		if foundEvent.ID != 0 {
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
		mu.Lock()
		eventFound := false

		for _, event := range events {
			if event.ID == uint(eventID) {
				eventFound = true
				break
			}
		}
		if !eventFound {
			mu.Unlock()
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		newSeat := Seat{
			ID:      nextSeatID,
			EventID: uint(eventID),
			Status:  StatusAvailable,
		}

		nextSeatID++
		seats = append(seats, newSeat)
		mu.Unlock()

		c.JSON(http.StatusCreated, newSeat)

	})
	router.GET("/events/:id/seats", func(c *gin.Context) {
		idText := c.Param("id")

		eventID, err := strconv.ParseUint(idText, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event id"})
			return
		}
		mu.RLock()
		eventFound := false

		for _, event := range events {
			if event.ID == uint(eventID) {
				eventFound = true
				break
			}
		}
		if !eventFound {
			mu.RUnlock()
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		eventSeats := []Seat{}

		for _, seat := range seats {
			if seat.EventID == uint(eventID) {
				eventSeats = append(eventSeats, seat)
			}
		}
		mu.RUnlock()
		c.JSON(http.StatusOK, eventSeats)
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "SeatRush api is running",
		})
	})
	router.GET("/events", func(c *gin.Context) {
		mu.RLock()
		currentEvents := append([]Event(nil), events...)
		mu.RUnlock()
		c.JSON(http.StatusOK, currentEvents)
	})
	fmt.Println("Gin server is running on :8080")
	router.Run(":8080")
}
