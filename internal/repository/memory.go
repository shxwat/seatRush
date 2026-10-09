package repository

import (
	"seatrush/internal/domain"
	"sync"
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
