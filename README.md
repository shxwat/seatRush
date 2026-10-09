# SeatRush

A high-concurrency event ticket reservation backend in Go.

When a popular show goes on sale, hundreds of people click **Book** on the same seat in the same millisecond. Exactly one of them should get it. SeatRush is a booking API built around that one guarantee, plus a load tester that proves it and a deliberately broken mode that shows what happens without it.

## The result

`cmd/loadtest` fires N reservation requests for the **same seat at the same instant**, then checks the HTTP responses against the server's own state.

| Concurrent buyers | Safe mode | Unsafe mode (`SEATRUSH_DEMO_UNSAFE=true`) |
|---:|---|---|
| 20  | 1 × `201`, 19 × `409` — **1 reservation** | 20 × `201` — **20 reservations for one seat** |
| 100 | 1 × `201`, 99 × `409` — **1 reservation** | 100 × `201` — **100 reservations for one seat** |
| 500 | 1 × `201`, 499 × `409` — **1 reservation** | 500 × `201` — **500 reservations for one seat** |

In safe mode the winning request completed in about 1–3 ms even with 500 buyers queued on the lock. Measured on an Apple M4, Go 1.26.

## How it works

A seat moves through three states:

```
AVAILABLE ──(POST /reservations)──▶ HELD ──(planned)──▶ BOOKED
```

The whole **check-and-claim** happens inside one critical section: lock, find the seat, confirm it is `AVAILABLE`, mark it `HELD`, record the reservation, unlock. Every other request queues on the mutex. When it gets in, it sees `HELD` and gets a `409 Conflict`.

The unsafe mode (`demo_unsafe.go`) splits that into two critical sections: *check*, then a 25 ms gap standing in for a database round trip, then *claim* without re-checking. Each access is still behind the lock, so `go test -race` stays quiet, but the operation is not atomic, and every buyer who checks inside the gap "wins". That is the classic check-then-act bug, and it is why "it has a mutex" is not the same as "it's correct".

## API

| Method | Path | Description |
|---|---|---|
| `POST` | `/events` | Create an event |
| `GET` | `/events` | List events |
| `GET` | `/events/:id` | Get an event |
| `POST` | `/events/:id/seats` | Add a seat to an event |
| `GET` | `/events/:id/seats` | List an event's seats and their status |
| `POST` | `/reservations` | Hold a seat: `{"user_id", "event_id", "seat_id"}` → `201` or `409` |
| `GET` | `/reservations/:id` | Get a reservation |

## Run it

```sh
go run .                              # API on :8080
go run ./cmd/loadtest -n 100          # 100 buyers race for one fresh seat
```

Watch it fail on purpose:

```sh
SEATRUSH_DEMO_UNSAFE=true go run .
go run ./cmd/loadtest -n 100          # Verdict: DOUBLE BOOKING
```

Tests, with the race detector:

```sh
go test -race ./...
```

## Watch it live in Goroutine Colony

SeatRush can stream every request's goroutine, lock wait and seat read/write to [Goroutine Colony](https://github.com/shxwat/goroutine-colony), which renders them as a live 3D colony. You can watch requests pile up on the mutex and peel away with `409`, or see the double booking happen in unsafe mode.

```sh
SEATRUSH_COLONY_ADDR=:7070 go run .
# then open the colony with ?ws=ws://localhost:7070/colony
```

The bridge listens on its own port and only runs when `SEATRUSH_COLONY_ADDR` is set, so the booking API is unaffected.

## Layout

```
main.go                      Gin router, handlers, the safe reservation path
demo_unsafe.go               the deliberately broken check-then-act path
main_test.go                 concurrency and HTTP tests
cmd/loadtest/                concurrent buyer simulator and verdict
internal/domain/             Event, Seat, User, Reservation, status constants
internal/repository/         in-memory, mutex-guarded store
internal/instrument/         per-request spans: lock wait, lock acquired, reads, writes
internal/colonybridge/       WebSocket stream to Goroutine Colony
```

## What's next

- PostgreSQL storage with `SELECT … FOR UPDATE`, so the guarantee holds across multiple instances
- Hold expiry: `HELD` seats return to `AVAILABLE` if payment doesn't confirm in time
- `HELD → BOOKED` confirmation endpoint
