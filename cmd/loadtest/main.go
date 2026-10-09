// Command loadtest fires N reservation requests for the same seat at the same
// instant and checks the HTTP results against SeatRush's own state.
//
//	go run ./cmd/loadtest -n 20
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type result struct {
	user          int
	status        int
	requestID     string
	reservationID uint
	latency       time.Duration
	err           error
}

func main() {
	base := flag.String("url", "http://localhost:8080", "SeatRush API base URL")
	n := flag.Int("n", 20, "concurrent buyers")
	eventID := flag.Uint("event", 101, "event ID")
	seatID := flag.Uint("seat", 0, "seat ID to contend for (0 = create a fresh seat first)")
	flag.Parse()

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{MaxIdleConns: *n * 2, MaxIdleConnsPerHost: *n * 2, MaxConnsPerHost: *n * 2},
	}

	if *seatID == 0 {
		var seat struct{ ID uint }
		if code, err := doJSON(client, "POST", fmt.Sprintf("%s/events/%d/seats", *base, *eventID), nil, &seat); err != nil || code != http.StatusCreated {
			fail("could not create a seat (status %d): %v", code, err)
		}
		*seatID = seat.ID
	}
	fmt.Printf("Target: event %d, seat %d — %d buyers\n", *eventID, *seatID, *n)

	// Warm one keep-alive connection per buyer so the burst is not serialized by dialing.
	var warm sync.WaitGroup
	for i := 0; i < *n; i++ {
		warm.Add(1)
		go func() {
			defer warm.Done()
			if resp, err := client.Get(*base + "/"); err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}
	warm.Wait()
	time.Sleep(200 * time.Millisecond) // let the warm-up settle so it never merges into the burst

	// Every buyer builds its request, then waits at the barrier.
	start := make(chan struct{})
	var ready, done sync.WaitGroup
	results := make([]result, *n)
	for i := 0; i < *n; i++ {
		ready.Add(1)
		done.Add(1)
		go func(i int) {
			defer done.Done()
			body, _ := json.Marshal(map[string]uint{"user_id": uint(1000 + i), "event_id": *eventID, "seat_id": *seatID})
			req, _ := http.NewRequest("POST", *base+"/reservations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			ready.Done()
			<-start

			t0 := time.Now()
			resp, err := client.Do(req)
			r := result{user: 1000 + i, err: err, latency: time.Since(t0)}
			if err == nil {
				r.status = resp.StatusCode
				r.requestID = resp.Header.Get("X-Request-ID")
				var res struct{ ID uint }
				if resp.StatusCode == http.StatusCreated {
					json.NewDecoder(resp.Body).Decode(&res)
					r.reservationID = res.ID
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			results[i] = r
		}(i)
	}
	ready.Wait()
	close(start) // release every buyer at once
	done.Wait()

	success, conflict, other := 0, 0, 0
	var winners []result
	for _, r := range results {
		switch {
		case r.err != nil:
			other++
		case r.status == http.StatusCreated:
			success++
			winners = append(winners, r)
		case r.status == http.StatusConflict:
			conflict++
		default:
			other++
		}
	}
	sort.Slice(winners, func(i, j int) bool { return winners[i].reservationID < winners[j].reservationID })

	fmt.Printf("\nRequests: %d\nSuccess:  %d\nConflict: %d\nOther:    %d\n", *n, success, conflict, other)
	for _, w := range winners {
		fmt.Printf("  winner %s (user %d) → reservation %d in %v\n", w.requestID, w.user, w.reservationID, w.latency.Round(time.Microsecond))
	}

	// Verify against the backend: the source of truth.
	var seats []struct {
		ID     uint
		Status string
	}
	if _, err := doJSON(client, "GET", fmt.Sprintf("%s/events/%d/seats", *base, *eventID), nil, &seats); err != nil {
		fail("could not read seats: %v", err)
	}
	seatStatus := "?"
	for _, s := range seats {
		if s.ID == *seatID {
			seatStatus = s.Status
		}
	}
	confirmed := 0
	for _, w := range winners {
		var res struct {
			SeatID uint
			UserID uint
		}
		if code, err := doJSON(client, "GET", fmt.Sprintf("%s/reservations/%d", *base, w.reservationID), nil, &res); err == nil && code == 200 && res.SeatID == *seatID && res.UserID == uint(w.user) {
			confirmed++
		}
	}
	fmt.Printf("\nBackend: seat %d is %s; %d reservation(s) on record for it\n", *seatID, seatStatus, confirmed)
	switch {
	case success == 1 && seatStatus == "HELD" && confirmed == 1:
		fmt.Println("Verdict: CORRECT — exactly one buyer holds the seat")
	case success > 1:
		fmt.Printf("Verdict: DOUBLE BOOKING — %d buyers were told they own seat %d\n", success, *seatID)
	default:
		fmt.Println("Verdict: UNEXPECTED — inspect the server")
		os.Exit(1)
	}
}

func doJSON(c *http.Client, method, url string, body any, out any) (int, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "loadtest: "+format+"\n", args...)
	os.Exit(1)
}
