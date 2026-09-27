package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Method:", r.Method)
	fmt.Println("Path", r.URL.Path)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]string{
		"message": "SeatRush API is running",
	})

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
	held := false
	found := false

	requestedSeatID := uint(1)
	for i := range seats {
		if seats[i].ID == requestedSeatID {
			found = true

			if seats[i].Status == StatusAvailable {
				seats[i].Status = StatusHeld
				held = true
			}

		}
	}
	fmt.Println("Found:", found)
	fmt.Println("Held:", held)
	reservations := []Reservation{}

	if !found {
		fmt.Println("Reservation failed: Seat not found")
	} else if !held {
		fmt.Println("Reservation Failed: Seat unavailable")
	} else {

		reservation := Reservation{
			ID:      501,
			UserID:  user.ID,
			EventID: event.ID,
			SeatID:  requestedSeatID,
			Status:  ReservationStatusPending,
		}
		reservations = append(reservations, reservation)

		fmt.Println("Reservation: ", reservation)

		for i := range seats {
			if seats[i].ID == reservation.SeatID && seats[i].Status == StatusHeld {
				seats[i].Status = StatusBooked
			}
		}
		reservations[len(reservations)-1].Status = ReservationStatusConfirmed

	}

	fmt.Println("Reservations:", reservations)
	fmt.Println("Seats:", seats)

	fmt.Println(seats)
	fmt.Println(len(seats))
	fmt.Println(seats[0].Status)

	mux := http.NewServeMux()

	mux.HandleFunc("/seats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(seats)
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(events)
		case http.MethodPost:
			var input CreateEventRequest

			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if input.Name == "" {
				http.Error(w, "Name is required", http.StatusBadRequest)
				return
			}

			newEvent := Event{
				ID:   nextEventID,
				Name: input.Name,
			}
			nextEventID++
			events = append(events, newEvent)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(newEvent)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	handler := http.HandlerFunc(rootHandler)
	mux.Handle("/", handler)

	fmt.Println("Server is Running on :8080")
	http.ListenAndServe(":8080", mux)
}
