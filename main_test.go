package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentReservationAllowsOnlyOneHold(t *testing.T) {
	seats := []Seat{{ID: 1, EventID: 101, Status: StatusAvailable}}
	reservations := make([]Reservation, 0, 1)
	nextReservationID := uint(501)
	var mu sync.Mutex

	const requestCount = 100
	var wg sync.WaitGroup
	var successCount int
	var successMu sync.Mutex

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			found, reservation := holdSeat(&mu, &seats, &reservations, &nextReservationID, CreateReservationRequest{
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
	if len(reservations) != 1 || seats[0].Status != StatusHeld || nextReservationID != 502 {
		t.Fatalf("unexpected state: seats=%v, reservations=%v, next ID=%d", seats, reservations, nextReservationID)
	}
}

func testRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestAPIRoutes(t *testing.T) {
	router := setupRouter([]Event{{ID: 101, Name: "Test Event"}}, nil, nil, 102, 1, 501)
	cases := []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/", "", 200},
		{"GET", "/events", "", 200},
		{"GET", "/events/101", "", 200},
		{"GET", "/events/abc", "", 400},
		{"GET", "/events/999", "", 404},
		{"POST", "/events", "{", 400},
		{"POST", "/events", `{}`, 400},
		{"POST", "/events", `{"name":"New Event"}`, 201},
		{"GET", "/events/102", "", 200},
		{"GET", "/events/101/seats", "", 200},
		{"POST", "/events/abc/seats", "", 400},
		{"POST", "/events/999/seats", "", 404},
		{"GET", "/events/abc/seats", "", 400},
		{"GET", "/events/999/seats", "", 404},
		{"POST", "/events/101/seats", "", 201},
		{"POST", "/reservations", "{", 400},
		{"POST", "/reservations", `{"user_id":1,"event_id":102,"seat_id":1}`, 404},
		{"POST", "/reservations", `{"user_id":1,"event_id":101,"seat_id":999}`, 404},
		{"POST", "/reservations", `{"user_id":1,"event_id":101,"seat_id":1}`, 201},
		{"POST", "/reservations", `{"user_id":2,"event_id":101,"seat_id":1}`, 409},
		{"GET", "/reservations/501", "", 200},
		{"GET", "/reservations/abc", "", 400},
		{"GET", "/reservations/999", "", 404},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := testRequest(router, tc.method, tc.path, tc.body)
			if response.Code != tc.status {
				t.Fatalf("expected status %d, got %d: %s", tc.status, response.Code, response.Body.String())
			}
		})
	}
	var reservation Reservation
	response := testRequest(router, "GET", "/reservations/501", "")
	if err := json.Unmarshal(response.Body.Bytes(), &reservation); err != nil {
		t.Fatal(err)
	}
	if reservation.ID != 501 || reservation.UserID != 1 || reservation.EventID != 101 || reservation.SeatID != 1 || reservation.Status != ReservationStatusPending {
		t.Fatalf("unexpected reservation: %+v", reservation)
	}
}

func TestConcurrentHTTPReservations(t *testing.T) {
	router := setupRouter(
		[]Event{{ID: 101, Name: "Test Event"}},
		[]Seat{{ID: 1, EventID: 101, Status: StatusAvailable}}, nil, 102, 2, 501,
	)
	const requestCount = 100
	statuses := make(chan int, requestCount)
	var wg sync.WaitGroup
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := testRequest(router, "POST", "/reservations", `{"user_id":1,"event_id":101,"seat_id":1}`)
			statuses <- response.Code
			// Exercise reads and slice growth alongside reservation requests.
			for _, request := range []struct{ method, path, body string }{
				{"POST", "/events", `{"name":"Concurrent Event"}`},
				{"GET", "/events", ""},
				{"POST", "/events/101/seats", ""},
				{"GET", "/events/101/seats", ""},
				{"GET", "/events/101", ""},
				{"GET", "/reservations/501", ""},
			} {
				response := testRequest(router, request.method, request.path, request.body)
				if response.Code != http.StatusOK && response.Code != http.StatusCreated {
					t.Errorf("%s %s returned %d", request.method, request.path, response.Code)
				}
			}
		}()
	}
	wg.Wait()
	close(statuses)
	created, conflicts := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("unexpected reservation status: %d", status)
		}
	}
	if created != 1 || conflicts != requestCount-1 {
		t.Fatalf("expected 1 success and 99 conflicts, got %d and %d", created, conflicts)
	}
}
