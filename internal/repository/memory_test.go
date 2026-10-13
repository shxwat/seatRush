package repository

import (
	"sync"
	"testing"

	"seatrush/internal/domain"
)

func TestConcurrentReservationAllowsOnlyOneHold(t *testing.T) {
	repo := NewMemoryRepository(
		[]domain.Event{{ID: 101, Name: "Test Event"}},
		[]domain.Seat{{ID: 1, EventID: 101, Status: domain.StatusAvailable}},
		nil, 102, 2, 501,
	)

	const requestCount = 100
	var wg sync.WaitGroup
	var successCount int
	var successMu sync.Mutex
	start := make(chan struct{})

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			_, reservation := repo.HoldSeat(1, 101, 1)
			if reservation.ID != 0 {
				successMu.Lock()
				successCount++
				successMu.Unlock()
			}
		}()
	}

	close(start)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected 1 successful hold, got %d", successCount)
	}

	seats := repo.ListSeatsByEvent(101)
	reservation, found := repo.GetReservationByID(501)

	if !found ||
		reservation.SeatID != 1 ||
		reservation.Status != domain.ReservationStatusPending ||
		len(seats) != 1 ||
		seats[0].Status != domain.StatusHeld {
		t.Fatalf(
			"unexpected state: seats=%v, reservation=%v, found=%v",
			seats,
			reservation,
			found,
		)
	}
}
