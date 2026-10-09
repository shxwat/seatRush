package main

import (
	"sync"
	"time"

	"seatrush/internal/instrument"
)

// unsafeGap is the pause between checking and acting — the window that a
// database round trip or a slow service call would open in a real system.
const unsafeGap = 25 * time.Millisecond

// holdSeatCheckThenAct is a DELIBERATELY BROKEN reservation, used only when
// SEATRUSH_DEMO_UNSAFE=true to demonstrate a race condition.
//
// It is memory-safe (every access to the slices happens under mu, so the Go
// race detector stays quiet), but it is not atomic: it checks availability in
// one critical section and claims the seat in another, without re-checking.
// Every request that checks inside the gap sees AVAILABLE, and all of them
// "win" — a double booking. The correct path is holdSeatTraced.
func holdSeatCheckThenAct(mu *sync.Mutex, seats *[]Seat, reservations *[]Reservation, nextReservationID *uint, input CreateReservationRequest, span *instrument.Span) (bool, Reservation) {
	// check
	span.LockWait("mu")
	mu.Lock()
	span.LockAcquired("mu")
	found, status := false, ""
	for _, s := range *seats {
		if s.ID == input.SeatID && s.EventID == input.EventID {
			found, status = true, s.Status
			span.SeatRead(input.EventID, input.SeatID, status)
			break
		}
	}
	span.LockReleased("mu")
	mu.Unlock()

	if !found {
		return false, Reservation{}
	}
	if status != StatusAvailable {
		return true, Reservation{}
	}

	time.Sleep(unsafeGap)

	// act — trusting a status that may no longer be true
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
