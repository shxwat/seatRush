// Package instrument records meaningful concurrency transitions of a request
// (arrival, lock contention, seat reads/writes, response) as plain events.
//
// It knows nothing about who consumes them. Business code holds a *Span and
// calls its methods; a nil *Span is valid and every method is a no-op, so
// instrumentation costs nothing when disabled.
package instrument

import (
	"sync/atomic"
	"time"
)

type Kind string

const (
	RequestStarted     Kind = "REQUEST_STARTED"
	ReservationAttempt Kind = "RESERVATION_ATTEMPT"
	LockWait           Kind = "LOCK_WAIT"
	LockAcquired       Kind = "LOCK_ACQUIRED"
	SeatRead           Kind = "SEAT_READ"
	SeatHeld           Kind = "SEAT_HELD"
	LockReleased       Kind = "LOCK_RELEASED"
	RequestFinished    Kind = "REQUEST_FINISHED"
)

// Event is one observed transition. Seq is a process-wide total order
// assigned at the moment of emission.
type Event struct {
	Seq           uint64 `json:"seq"`
	Kind          Kind   `json:"kind"`
	TimeUnixNano  int64  `json:"ts"`
	RequestID     string `json:"request_id"`
	RequestNum    uint64 `json:"request_num"`
	Method        string `json:"method,omitempty"`
	Path          string `json:"path,omitempty"`
	Lock          string `json:"lock,omitempty"`
	EventID       uint   `json:"event_id,omitempty"`
	SeatID        uint   `json:"seat_id,omitempty"`
	UserID        uint   `json:"user_id,omitempty"`
	SeatStatus    string `json:"seat_status,omitempty"`
	ReservationID uint   `json:"reservation_id,omitempty"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	LatencyMicros int64  `json:"latency_us,omitempty"`
}

// Sink receives events. Implementations must not block: Emit may be called
// while a request holds a lock.
type Sink interface {
	Emit(Event)
}

var seq atomic.Uint64

// Span is the per-request handle passed through business code.
type Span struct {
	sink  Sink
	id    string
	num   uint64
	start time.Time
}

func NewSpan(sink Sink, id string, num uint64) *Span {
	return &Span{sink: sink, id: id, num: num, start: time.Now()}
}

func (s *Span) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

func (s *Span) emit(e Event) {
	if s == nil || s.sink == nil {
		return
	}
	e.Seq = seq.Add(1)
	e.TimeUnixNano = time.Now().UnixNano()
	e.RequestID = s.id
	e.RequestNum = s.num
	s.sink.Emit(e)
}

func (s *Span) Started(method, path string) {
	s.emit(Event{Kind: RequestStarted, Method: method, Path: path})
}

func (s *Span) Attempt(eventID, seatID, userID uint) {
	s.emit(Event{Kind: ReservationAttempt, EventID: eventID, SeatID: seatID, UserID: userID})
}

func (s *Span) LockWait(lock string)     { s.emit(Event{Kind: LockWait, Lock: lock}) }
func (s *Span) LockAcquired(lock string) { s.emit(Event{Kind: LockAcquired, Lock: lock}) }
func (s *Span) LockReleased(lock string) { s.emit(Event{Kind: LockReleased, Lock: lock}) }

func (s *Span) SeatRead(eventID, seatID uint, status string) {
	s.emit(Event{Kind: SeatRead, EventID: eventID, SeatID: seatID, SeatStatus: status})
}

func (s *Span) SeatHeld(eventID, seatID, reservationID uint) {
	s.emit(Event{Kind: SeatHeld, EventID: eventID, SeatID: seatID, ReservationID: reservationID, SeatStatus: "HELD"})
}

func (s *Span) Finished(httpStatus int) {
	if s == nil {
		return
	}
	s.emit(Event{Kind: RequestFinished, HTTPStatus: httpStatus, LatencyMicros: time.Since(s.start).Microseconds()})
}
