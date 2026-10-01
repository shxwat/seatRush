package main

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrentReservationRace(t *testing.T) {
	seat := Seat{
		ID:      1,
		EventID: 101,
		Status:  StatusAvailable,
	}
	var wg sync.WaitGroup

	success := make(chan struct{}, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if seat.Status == StatusAvailable {
				time.Sleep(time.Millisecond)
				seat.Status = StatusHeld
				success <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(success)
	successfulHolds := len(success)

	if successfulHolds != 1 {
		t.Fatalf("expected 1 successful hold, got %d", successfulHolds)
	}
	t.Log("successful holds:", len(success))
}
