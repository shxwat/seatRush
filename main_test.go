package main

import (
	"sync"
	"testing"
)

func TestConcurrentReservationAllowsOnlyOneHold(t *testing.T) {
	seats := []Seat{{ID: 1, EventID: 101, Status: StatusAvailable}}
	reservations := make([]Reservation, 0, 1)
	nextReservationID := uint(501)
	var mu sync.RWMutex

	const requestCount = 100
	var wg sync.WaitGroup
	var successCount int
	var successMu sync.Mutex

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			found, reservation := holdSeat(&mu, seats, &reservations, &nextReservationID, CreateReservationRequest{
				UserID:  1,
				EventID: 101,
				SeatID:  1,
			})
			if found && reservation.ID != 0 {
				successMu.Lock()
				successCount++
				successMu.Unlock()
			}
		}()
	}

	wg.Wait()
	if successCount != 1 {
		t.Fatalf("expected 1 successful hold, got %d", successCount)
	}
}
