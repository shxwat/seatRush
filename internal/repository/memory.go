package repository

import (
	"sync"

	"seatrush/internal/domain"
)

type MemoryRepository struct {
	mu sync.Mutex

	events       []domain.Event
	seats        []domain.Seat
	reservations []domain.Reservation

	nextEventID       uint
	nextSeatID        uint
	nextReservationID uint
}

func NewMemoryRepository(
	events []domain.Event,
	seats []domain.Seat,
	reservations []domain.Reservation,
	nextEventID uint,
	nextSeatID uint,
	nextReservationID uint,
) *MemoryRepository {
	return &MemoryRepository{
		events:            events,
		seats:             seats,
		reservations:      reservations,
		nextEventID:       nextEventID,
		nextSeatID:        nextSeatID,
		nextReservationID: nextReservationID,
	}
}

// HoldSeat checks availability and creates the hold under the same lock.
// A false result means the seat does not exist in the requested event.
// A true result with a zero reservation ID means the seat is unavailable.
func (r *MemoryRepository) HoldSeat(
	userID uint,
	eventID uint,
	seatID uint,
) (bool, domain.Reservation) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.seats {
		if r.seats[i].ID != seatID || r.seats[i].EventID != eventID {
			continue
		}
		if r.seats[i].Status != domain.StatusAvailable {
			return true, domain.Reservation{}
		}

		r.seats[i].Status = domain.StatusHeld

		reservation := domain.Reservation{
			ID:      r.nextReservationID,
			UserID:  userID,
			EventID: eventID,
			SeatID:  seatID,
			Status:  domain.ReservationStatusPending,
		}
		r.nextReservationID++
		r.reservations = append(r.reservations, reservation)

		return true, reservation
	}
	return false, domain.Reservation{}
}

func (r *MemoryRepository) GetReservationByID(id uint) (domain.Reservation, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, reservation := range r.reservations {
		if reservation.ID == id {
			return reservation, true
		}
	}
	return domain.Reservation{}, false
}

func (r *MemoryRepository) CreateEvent(name string) domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	event := domain.Event{
		ID:   r.nextEventID,
		Name: name,
	}

	r.nextEventID++
	r.events = append(r.events, event)

	return event
}

func (r *MemoryRepository) GetEventByID(id uint) (domain.Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, event := range r.events {
		if event.ID == id {
			return event, true
		}
	}
	return domain.Event{}, false
}

func (r *MemoryRepository) ListEvents() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	events := append([]domain.Event(nil), r.events...)
	return events
}

func (r *MemoryRepository) AddSeat(eventID uint) domain.Seat {
	r.mu.Lock()
	defer r.mu.Unlock()

	seat := domain.Seat{
		ID:      r.nextSeatID,
		EventID: eventID,
		Status:  domain.StatusAvailable,
	}
	r.nextSeatID++
	r.seats = append(r.seats, seat)

	return seat
}

func (r *MemoryRepository) ListSeatsByEvent(eventID uint) []domain.Seat {
	r.mu.Lock()
	defer r.mu.Unlock()

	seats := []domain.Seat{}

	for _, seat := range r.seats {
		if seat.EventID == eventID {
			seats = append(seats, seat)
		}
	}
	return seats
}
