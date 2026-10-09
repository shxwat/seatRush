// Package colonybridge streams SeatRush's instrumentation events to
// Goroutine Colony over WebSocket. It is a development/demo tool: it is only
// wired up when SEATRUSH_COLONY_ADDR is set, and it listens on its own port so
// the booking API is untouched.
package colonybridge

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"seatrush/internal/instrument"
)

// Snapshot is a read-only view of booking state, used to (re)build the world
// for a new run or a newly connected viewer.
type Snapshot struct {
	Events []EventInfo
	Seats  []SeatInfo
}

type EventInfo struct {
	ID   uint
	Name string
}

type SeatInfo struct {
	ID      uint
	EventID uint
	Status  string
}

type Options struct {
	// Snapshot reads current state. Called from the bridge goroutine only.
	Snapshot func() Snapshot
	// Mode is shown to viewers ("SAFE" or "UNSAFE").
	Mode string
	// RunGap is the idle time after which the next request starts a new run.
	RunGap time.Duration
}

// Bridge implements instrument.Sink.
type Bridge struct {
	opts    Options
	events  chan instrument.Event
	join    chan *client
	leave   chan *client
	dropped atomic.Uint64
}

type client struct {
	out chan []Frame
}

func New(opts Options) *Bridge {
	if opts.RunGap == 0 {
		opts.RunGap = 1500 * time.Millisecond
	}
	if opts.Mode == "" {
		opts.Mode = "SAFE"
	}
	return &Bridge{
		opts:   opts,
		events: make(chan instrument.Event, 1<<16),
		join:   make(chan *client),
		leave:  make(chan *client),
	}
}

// Emit never blocks; if the bridge falls behind, events are dropped and counted.
func (b *Bridge) Emit(e instrument.Event) {
	select {
	case b.events <- e:
	default:
		b.dropped.Add(1)
	}
}

// Run processes events and serves ws://addr/colony until ctx is done.
func (b *Bridge) Run(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/colony", b.serveWS)
	srv := &http.Server{Addr: addr, Handler: mux}
	go b.loop(ctx)
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	log.Printf("colony bridge: streaming on ws://localhost%s/colony (mode %s)", addr, b.opts.Mode)
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (b *Bridge) serveWS(w http.ResponseWriter, r *http.Request) {
	// Dev tool: the viewer runs on another localhost port.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := conn.CloseRead(r.Context())

	cl := &client{out: make(chan []Frame, 1024)}
	b.join <- cl
	defer func() { b.leave <- cl }()

	for {
		select {
		case <-ctx.Done():
			return
		case frames := <-cl.out:
			wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := wsjson.Write(wctx, conn, frames)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// ───────────────────────────────────────────────────────────── translation

type state struct {
	runStart  time.Time
	lastEvent time.Time
	focus     uint            // event whose seats are on screen
	shown     map[uint]string // seat id → last status shown
	heldBy    map[uint]uint64 // seat id → request that wrote HELD this run
	raced     map[uint]bool
	reqs      map[uint64]string // live reservation requests: num → id
	target    uint
}

func (b *Bridge) loop(ctx context.Context) {
	clients := map[*client]bool{}
	st := &state{}
	b.resetRun(st, time.Now())

	broadcast := func(frames []Frame) {
		if len(frames) == 0 {
			return
		}
		for cl := range clients {
			select {
			case cl.out <- frames:
			default: // a stalled viewer must never slow the server
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case cl := <-b.join:
			clients[cl] = true
			cl.out <- b.worldFrames(st, time.Now())
		case cl := <-b.leave:
			delete(clients, cl)
		case e := <-b.events:
			now := time.Unix(0, e.TimeUnixNano)
			var frames []Frame
			if e.Kind == instrument.RequestStarted && now.Sub(st.lastEvent) > b.opts.RunGap {
				// a burst after silence is a new run: rebuild the world from real state
				b.resetRun(st, now)
				frames = append(frames, b.worldFrames(st, now)...)
			}
			st.lastEvent = now
			frames = append(frames, b.translate(st, e)...)
			broadcast(frames)
		}
	}
}

func (b *Bridge) resetRun(st *state, now time.Time) {
	st.runStart = now
	st.shown = map[uint]string{}
	st.heldBy = map[uint]uint64{}
	st.raced = map[uint]bool{}
	st.reqs = map[uint64]string{}
}

func (b *Bridge) t(st *state, at time.Time) float64 {
	return at.Sub(st.runStart).Seconds()
}

// worldFrames: RUN_START + header + the lock + every seat of the focused event.
func (b *Bridge) worldFrames(st *state, now time.Time) []Frame {
	t := b.t(st, now)
	snap := b.opts.Snapshot()
	if st.focus == 0 && len(snap.Events) > 0 {
		st.focus = snap.Events[0].ID
	}
	st.shown = map[uint]string{}
	frames := []Frame{
		{Type: "RUN_START", T: t, Program: "seatrush"},
		b.info(st, snap, t),
		{Type: "MUTEX_MAKE", T: t, Mu: "mu"},
	}
	return append(frames, b.syncSeats(st, snap, t)...)
}

func (b *Bridge) info(st *state, snap Snapshot, t float64) Frame {
	name := fmt.Sprintf("event %d", st.focus)
	for _, ev := range snap.Events {
		if ev.ID == st.focus {
			name = ev.Name
		}
	}
	seat := "—"
	if st.target != 0 {
		seat = fmt.Sprintf("seat %d", st.target)
	}
	return Frame{
		Type:  "STREAM_INFO",
		T:     t,
		App:   "SeatRush",
		Title: "SEATRUSH — LIVE",
		Fields: []Field{
			{K: "Event", V: name},
			{K: "Seat", V: seat},
			{K: "Mode", V: b.opts.Mode},
		},
		Outcomes: map[string]string{"success": "Successful", "rejected": "Conflicts", "error": "Other"},
	}
}

// syncSeats allocates seats not yet on screen (e.g. created since the last sync).
func (b *Bridge) syncSeats(st *state, snap Snapshot, t float64) []Frame {
	var seats []SeatInfo
	for _, s := range snap.Seats {
		if s.EventID == st.focus {
			seats = append(seats, s)
		}
	}
	sort.Slice(seats, func(i, j int) bool { return seats[i].ID < seats[j].ID })
	var frames []Frame
	for _, s := range seats {
		if _, ok := st.shown[s.ID]; ok {
			continue
		}
		st.shown[s.ID] = s.Status
		frames = append(frames, Frame{
			Type: "MEMORY_ALLOC", T: t, GID: u64(0),
			Mem: seatMem(s.ID), Label: fmt.Sprintf("seat %d", s.ID), Group: "seats",
			Value: s.Status, Tone: tone(s.Status), Addr: fmt.Sprintf("event %d", s.EventID),
		})
	}
	return frames
}

func seatMem(id uint) string { return fmt.Sprintf("seat-%d", id) }

func (b *Bridge) translate(st *state, e instrument.Event) []Frame {
	t := b.t(st, time.Unix(0, e.TimeUnixNano))
	gid := u64(e.RequestNum)
	isReservation := e.Path == "/reservations" || st.reqs[e.RequestNum] != ""

	switch e.Kind {
	case instrument.RequestStarted:
		if e.Path != "/reservations" {
			return nil // admin/read traffic is not a buyer
		}
		st.reqs[e.RequestNum] = e.RequestID
		return []Frame{{Type: "GOROUTINE_SPAWN", T: t, GID: gid, Fn: "POST /reservations", Label: e.RequestID, At: "core"}}

	case instrument.ReservationAttempt:
		if !isReservation {
			return nil
		}
		var frames []Frame
		snap := Snapshot{}
		if e.EventID != st.focus || st.target != e.SeatID {
			snap = b.opts.Snapshot()
		}
		if e.EventID != st.focus {
			st.focus = e.EventID
			frames = append(frames, b.syncSeats(st, snap, t)...)
		}
		if st.target != e.SeatID {
			st.target = e.SeatID
			frames = append(frames, b.info(st, snap, t))
		}
		return frames

	case instrument.LockWait:
		if !isReservation {
			return nil
		}
		// every request really does call mu.Lock(); whether it had to wait, and
		// for how long, is decided by the LOCK_ACQUIRED order that follows
		return []Frame{{Type: "MUTEX_WAIT", T: t, GID: gid, Mu: e.Lock}}

	case instrument.LockAcquired:
		if !isReservation {
			return nil
		}
		return []Frame{{Type: "MUTEX_LOCK", T: t, GID: gid, Mu: e.Lock}}

	case instrument.LockReleased:
		if !isReservation {
			return nil
		}
		return []Frame{{Type: "MUTEX_UNLOCK", T: t, GID: gid, Mu: e.Lock}}

	case instrument.SeatRead:
		if !isReservation || e.EventID != st.focus {
			return nil
		}
		return []Frame{{Type: "MEMORY_READ", T: t, GID: gid, Mem: seatMem(e.SeatID), Value: e.SeatStatus, Tone: tone(e.SeatStatus)}}

	case instrument.SeatHeld:
		if !isReservation || e.EventID != st.focus {
			return nil
		}
		frames := []Frame{
			{Type: "MEMORY_WRITE", T: t, GID: gid, Mem: seatMem(e.SeatID), Value: "HELD", Tone: tone("HELD")},
			{Type: "PAYLOAD_CREATE", T: t, GID: gid, PID: u64(uint64(e.ReservationID)), Label: fmt.Sprintf("reservation %d", e.ReservationID)},
		}
		if prev, ok := st.heldBy[e.SeatID]; ok && prev != e.RequestNum && !st.raced[e.SeatID] {
			// the same seat was handed to two requests: the backend state is now wrong
			st.raced[e.SeatID] = true
			frames = append(frames, Frame{
				Type: "RACE_DETECTED", T: t, Mem: seatMem(e.SeatID),
				GIDs: []uint64{e.RequestNum, prev}, Value: "HELD ×2", Expected: "HELD ×1",
				Title:  "DOUBLE BOOKING",
				Detail: fmt.Sprintf("seat %d was reserved by %s and req-%d", e.SeatID, e.RequestID, prev),
			})
		}
		st.heldBy[e.SeatID] = e.RequestNum
		return frames

	case instrument.RequestFinished:
		if !isReservation {
			// a write elsewhere (e.g. a new seat) may have changed what's on screen
			if e.HTTPStatus < 300 {
				return b.syncSeats(st, b.opts.Snapshot(), t)
			}
			return nil
		}
		delete(st.reqs, e.RequestNum)
		return []Frame{{Type: "GOROUTINE_DONE", T: t, GID: gid, Outcome: outcome(e.HTTPStatus), Status: statusText(e.HTTPStatus)}}
	}
	return nil
}

// tone tells the viewer how to colour a value without it knowing the domain.
func tone(status string) string {
	if status == "AVAILABLE" {
		return "calm"
	}
	return "warm"
}

func outcome(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"
	case code == http.StatusConflict:
		return "rejected"
	default:
		return "error"
	}
}

func statusText(code int) string {
	switch code {
	case http.StatusCreated:
		return "201 RESERVED"
	case http.StatusConflict:
		return "409 SEAT UNAVAILABLE"
	default:
		return fmt.Sprintf("%d %s", code, strings.ToUpper(http.StatusText(code)))
	}
}
