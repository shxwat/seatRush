package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"seatrush/internal/colonybridge"
	"seatrush/internal/domain"
	"seatrush/internal/instrument"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
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

func holdSeat(mu *sync.Mutex, seats *[]Seat, reservations *[]Reservation, nextReservationID *uint, input CreateReservationRequest) (bool, Reservation) {
	return holdSeatTraced(mu, seats, reservations, nextReservationID, input, nil)
}

// holdSeatTraced is holdSeat with optional instrumentation. A nil span records
// nothing; the locking and the check-and-hold are identical either way.
func holdSeatTraced(mu *sync.Mutex, seats *[]Seat, reservations *[]Reservation, nextReservationID *uint, input CreateReservationRequest, span *instrument.Span) (bool, Reservation) {
	span.LockWait("mu")
	mu.Lock()
	span.LockAcquired("mu")
	defer func() {
		span.LockReleased("mu")
		mu.Unlock()
	}()

	for i := range *seats {
		if (*seats)[i].ID != input.SeatID || (*seats)[i].EventID != input.EventID {
			continue
		}
		span.SeatRead(input.EventID, input.SeatID, (*seats)[i].Status)
		if (*seats)[i].Status != StatusAvailable {
			return true, Reservation{}
		}

		(*seats)[i].Status = StatusHeld
		reservation := Reservation{
			ID:      *nextReservationID,
			UserID:  input.UserID,
			EventID: input.EventID,
			SeatID:  input.SeatID,
			Status:  ReservationStatusPending,
		}
		*nextReservationID = *nextReservationID + 1
		*reservations = append(*reservations, reservation)
		span.SeatHeld(input.EventID, input.SeatID, reservation.ID)
		return true, reservation
	}

	return false, Reservation{}
}

// routerOptions are opt-in, demo-only switches. The zero value is the normal API.
type routerOptions struct {
	// sink receives request instrumentation; nil disables it entirely.
	sink instrument.Sink
	// unsafeCheckThenAct selects the deliberately broken reservation path
	// (see demo_unsafe.go). Never enabled outside SEATRUSH_DEMO_UNSAFE.
	unsafeCheckThenAct bool
	// snapshot, when non-nil, is set to a function that reads current state.
	snapshot *func() colonybridge.Snapshot
}

func setupRouter(events []Event, seats []Seat, reservations []Reservation, nextEventID, nextSeatID, nextReservationID uint) *gin.Engine {
	return newRouter(routerOptions{}, events, seats, reservations, nextEventID, nextSeatID, nextReservationID)
}

func newRouter(opts routerOptions, events []Event, seats []Seat, reservations []Reservation, nextEventID, nextSeatID, nextReservationID uint) *gin.Engine {
	var mu sync.Mutex
	router := gin.New()
	if opts.sink != nil {
		router.Use(instrument.Middleware(opts.sink))
	}
	if opts.snapshot != nil {
		*opts.snapshot = func() colonybridge.Snapshot {
			mu.Lock()
			defer mu.Unlock()
			snap := colonybridge.Snapshot{}
			for _, e := range events {
				snap.Events = append(snap.Events, colonybridge.EventInfo{ID: e.ID, Name: e.Name})
			}
			for _, s := range seats {
				snap.Seats = append(snap.Seats, colonybridge.SeatInfo{ID: s.ID, EventID: s.EventID, Status: s.Status})
			}
			return snap
		}
	}

	router.POST("/reservations", func(c *gin.Context) {
		var input CreateReservationRequest

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON",
			})
			return
		}
		span := instrument.SpanFrom(c)
		span.Attempt(input.EventID, input.SeatID, input.UserID)
		var seatFound bool
		var reservation Reservation
		if opts.unsafeCheckThenAct {
			seatFound, reservation = holdSeatCheckThenAct(&mu, &seats, &reservations, &nextReservationID, input, span)
		} else {
			seatFound, reservation = holdSeatTraced(&mu, &seats, &reservations, &nextReservationID, input, span)
		}
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
		mu.Lock()
		var foundReservation Reservation
		for _, reservation := range reservations {
			if reservation.ID == uint(id) {
				foundReservation = reservation
				break
			}
		}
		mu.Unlock()
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
		mu.Lock()
		var foundEvent Event
		for _, event := range events {
			if event.ID == uint(id) {
				foundEvent = event
				break
			}
		}
		mu.Unlock()
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
		eventSeats := []Seat{}

		for _, seat := range seats {
			if seat.EventID == uint(eventID) {
				eventSeats = append(eventSeats, seat)
			}
		}
		mu.Unlock()
		c.JSON(http.StatusOK, eventSeats)
	})

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "SeatRush api is running",
		})
	})
	router.GET("/events", func(c *gin.Context) {
		mu.Lock()
		currentEvents := append([]Event(nil), events...)
		mu.Unlock()
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
	opts := routerOptions{unsafeCheckThenAct: os.Getenv("SEATRUSH_DEMO_UNSAFE") == "true"}
	if opts.unsafeCheckThenAct {
		fmt.Println("WARNING: SEATRUSH_DEMO_UNSAFE=true — reservations use a deliberately broken check-then-act path. Demo only.")
	}
	// Goroutine Colony live stream (development/demo only).
	var bridge *colonybridge.Bridge
	var bridgeAddr string
	if addr := os.Getenv("SEATRUSH_COLONY_ADDR"); addr != "" {
		var snapshot func() colonybridge.Snapshot
		mode := "SAFE"
		if opts.unsafeCheckThenAct {
			mode = "UNSAFE"
		}
		bridge = colonybridge.New(colonybridge.Options{Mode: mode, Snapshot: func() colonybridge.Snapshot { return snapshot() }})
		bridgeAddr = addr
		opts.sink = bridge
		opts.snapshot = &snapshot
	}
	router := newRouter(opts, events, seats, reservations, nextEventID, nextSeatID, nextReservationID)
	if bridge != nil {
		// started only after newRouter has bound the snapshot reader
		go func() {
			if err := bridge.Run(context.Background(), bridgeAddr); err != nil {
				fmt.Println("colony bridge stopped:", err)
			}
		}()
	}

	fmt.Println("Gin server is running on :8080")
	router.Run(":8080")
}
