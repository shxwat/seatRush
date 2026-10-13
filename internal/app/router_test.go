package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"seatrush/internal/domain"
	"seatrush/internal/handler"
	"seatrush/internal/repository"
	"seatrush/internal/service"
)

func newTestRouter() http.Handler {
	repo := repository.NewMemoryRepository(
		[]domain.Event{
			{ID: 101, Name: "Test Event"},
		},
		nil,
		nil,
		102,
		1,
		501,
	)
	svc := service.New(repo)
	h := handler.New(svc)

	return NewRouter(h)
}

func TestHealthRoute(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}
func testRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	return response
}
func TestEventGetRoutes(t *testing.T) {
	router := newTestRouter()

	cases := []struct {
		path   string
		status int
	}{
		{"/events", http.StatusOK},
		{"/events/101", http.StatusOK},
		{"/events/abc", http.StatusBadRequest},
		{"/events/999", http.StatusNotFound},
	}

	for _, tc := range cases {
		response := testRequest(router, http.MethodGet, tc.path, "")

		if response.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d", tc.path, tc.status, response.Code)
		}
	}
}
func TestCreateEventRoute(t *testing.T) {
	router := newTestRouter()

	cases := []struct {
		body   string
		status int
	}{
		{"{", http.StatusBadRequest},
		{`{}`, http.StatusBadRequest},
		{`{"name":"New Event"}`, http.StatusCreated},
	}

	for _, tc := range cases {
		response := testRequest(router, http.MethodPost, "/events", tc.body)

		if response.Code != tc.status {
			t.Fatalf("expected %d, got %d: %s", tc.status, response.Code, response.Body.String())
		}
	}

	response := testRequest(router, http.MethodGet, "/events/102", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected created event 102, got %d", response.Code)
	}
}

func TestSeatRoutes(t *testing.T) {
	router := newTestRouter()

	cases := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/events/101/seats", http.StatusOK},
		{http.MethodPost, "/events/abc/seats", http.StatusBadRequest},
		{http.MethodPost, "/events/999/seats", http.StatusNotFound},
		{http.MethodGet, "/events/abc/seats", http.StatusBadRequest},
		{http.MethodGet, "/events/999/seats", http.StatusNotFound},
		{http.MethodPost, "/events/101/seats", http.StatusCreated},
	}

	for _, tc := range cases {
		response := testRequest(router, tc.method, tc.path, "")

		if response.Code != tc.status {
			t.Fatalf("%s %s: expected %d, got %d",
				tc.method, tc.path, tc.status, response.Code)
		}
	}
}
func TestCreateReservationRoute(t *testing.T) {
	router := newTestRouter()

	response := testRequest(router, http.MethodPost, "/events/101/seats", "")
	if response.Code != http.StatusCreated {
		t.Fatalf("expected seat creation 201, got %d", response.Code)
	}

	cases := []struct {
		body   string
		status int
	}{
		{"{", http.StatusBadRequest},
		{`{"user_id":1,"event_id":102,"seat_id":1}`, http.StatusNotFound},
		{`{"user_id":1,"event_id":101,"seat_id":999}`, http.StatusNotFound},
		{`{"user_id":1,"event_id":101,"seat_id":1}`, http.StatusCreated},
		{`{"user_id":2,"event_id":101,"seat_id":1}`, http.StatusConflict},
	}

	for _, tc := range cases {
		response := testRequest(router, http.MethodPost, "/reservations", tc.body)

		if response.Code != tc.status {
			t.Fatalf("expected %d, got %d: %s",
				tc.status, response.Code, response.Body.String())
		}
	}
}
func TestGetReservationRoute(t *testing.T) {
	router := newTestRouter()

	testRequest(router, http.MethodPost, "/events/101/seats", "")
	testRequest(
		router,
		http.MethodPost,
		"/reservations",
		`{"user_id":1,"event_id":101,"seat_id":1}`,
	)

	cases := []struct {
		path   string
		status int
	}{
		{"/reservations/501", http.StatusOK},
		{"/reservations/abc", http.StatusBadRequest},
		{"/reservations/999", http.StatusNotFound},
	}

	for _, tc := range cases {
		response := testRequest(router, http.MethodGet, tc.path, "")

		if response.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d",
				tc.path, tc.status, response.Code)
		}
	}
	response := testRequest(router, http.MethodGet, "/reservations/501", "")

	var reservation domain.Reservation

	if err := json.Unmarshal(response.Body.Bytes(), &reservation); err != nil {
		t.Fatal(err)
	}

	if reservation.ID != 501 ||
		reservation.UserID != 1 ||
		reservation.EventID != 101 ||
		reservation.SeatID != 1 ||
		reservation.Status != domain.ReservationStatusPending {
		t.Fatalf("unexpected reservation: %+v", reservation)
	}

}
func TestConcurrentHTTPReservations(t *testing.T) {
	router := newTestRouter()

	response := testRequest(router, http.MethodPost, "/events/101/seats", "")
	if response.Code != http.StatusCreated {
		t.Fatalf("expected seat creation 201, got %d", response.Code)
	}

	const requestCount = 100
	statuses := make(chan int, requestCount)
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < requestCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-start

			response := testRequest(
				router,
				http.MethodPost,
				"/reservations",
				`{"user_id":1,"event_id":101,"seat_id":1}`,
			)

			statuses <- response.Code
			for _, req := range []struct {
				method string
				path   string
				body   string
			}{
				{http.MethodPost, "/events", `{"name":"Concurrent Event"}`},
				{http.MethodGet, "/events", ""},
				{http.MethodPost, "/events/101/seats", ""},
				{http.MethodGet, "/events/101/seats", ""},
				{http.MethodGet, "/events/101", ""},
				{http.MethodGet, "/reservations/501", ""},
			} {
				response := testRequest(router, req.method, req.path, req.body)

				if response.Code != http.StatusOK &&
					response.Code != http.StatusCreated {
					t.Errorf("%s %s returned %d", req.method, req.path, response.Code)
				}
			}
		}()
	}

	close(start)
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
		t.Fatalf(
			"expected 1 success and 99 conflicts, got %d and %d",
			created,
			conflicts,
		)
	}
}
