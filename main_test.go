package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentReservationAllowsOnlyOneHold(t *testing.T) {
	app := &App{
		events:            []Event{{ID: 101, Name: "Tech Conference"}},
		seats:             []Seat{{ID: 1, EventID: 101, Status: StatusAvailable}},
		reservations:      []Reservation{},
		nextReservationID: 501,
	}
	router := app.setupRouter()

	const requestCount = 100
	statuses := make(chan int, requestCount)
	var wg sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			request := httptest.NewRequest(http.MethodPost, "/reservations", strings.NewReader(
				`{"user_id":1,"event_id":101,"seat_id":1}`,
			))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			statuses <- response.Code
		}()
	}

	wg.Wait()
	close(statuses)

	created, unavailable := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			unavailable++
		default:
			t.Errorf("unexpected response status: %d", status)
		}
	}

	if created != 1 || unavailable != requestCount-1 {
		t.Fatalf("expected 1 successful hold and %d unavailable responses; got %d and %d", requestCount-1, created, unavailable)
	}
}
