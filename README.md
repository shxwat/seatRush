# SeatRush

Go mein event aur seat reservation API. Core problem: **ek seat ko ek saath bahut log reserve karein, toh sirf ek ko mile.**

Ye guide local Git history aur current code par based hai.

## 1. Pehle bas ye four cheezein samjho

| Cheez | Matlab | Example |
|---|---|---|
| Event | Jiske tickets chahiye | `101`: Tech Conference |
| Seat | Us event ki ek seat | Seat `1`, event `101`, `AVAILABLE` |
| User | Reserve karne wala | User `1` |
| Reservation | Kis user ne kaunsi seat hold ki uska record | `501`: user `1`, event `101`, seat `1`, `PENDING` |

Seat aur reservation alag records hain. Seat ka `HELD` status bolta hai ki ab doosra user ise nahi le sakta. Reservation ka `PENDING` status bolta hai ki booking abhi confirm nahi hui.

Current API mein:

```text
Seat:         AVAILABLE --reserve--> HELD
Reservation:                        PENDING
```

`BOOKED` aur `CONFIRMED` constants code mein hain, lekin abhi payment/confirmation endpoint nahi hai. Hold expiry aur cancellation bhi nahi hain; held seat server restart tak held rahegi. Data memory mein hai, restart par changes chale jaate hain.

## 2. Git ke hisaab se project kaise bana

| Date (2026) | Commit | Kya hua aur kyun |
|---|---|---|
| 25 Sep | `e1bf9e4` — initialize project | `go.mod`, README aur welcome print karne wala `main.go`. Abhi booking API nahi thi. |
| 27 Sep | `e944c40` — initial API | Event/Seat/User/Reservation structs aaye. Startup par hardcoded seat ko hold aur turant book karne ka demo tha. `net/http` se root, events aur seats routes bane. Reservation abhi POST request se nahi hoti thi. |
| 28 Sep | `fc02cef` — API functionality | Gin router aaya. Event create/get aur seat add routes bane. Purana `net/http` setup file mein tha, lekin server Gin wala run hone laga. Startup booking demo abhi tha. |
| 29 Sep | `f00835f` — reservation flow | Startup par automatic booking hataayi. `POST /reservations` se user ki request par hold aur `PENDING` reservation banne lagi. Reservation lookup aur event ke seats list karne ka route aaya. Shared data par abhi mutex nahi tha. |
| 1 Oct | `71a7756` — reservation test | Ek standalone test mein 100 goroutines ne same seat ko claim karne ki koshish ki. Check aur update ke beech sleep ne concurrency bug expose kiya. Ye HTTP route ka test nahi tha. |
| 4 Oct | `45ab3e3` — mutex protection | Shared state `App` struct mein gayi. `sync.RWMutex` se reads/writes protect hue. Seat check, hold aur reservation creation ek write lock mein aaye. HTTP test ne ek success aur baaki conflicts check kiye. |
| 4 Oct | `774f0ea` — concurrent reservations | Locked reservation operation `holdSeat` helper mein nikla. Test directly helper ko call karne laga. Kuch reads mein data copy karke JSON response se pehle unlock kiya gaya. Mutex protection previous commit mein already aa chuki thi. |

Iske baad `87ff57f` mein load tester, deliberately unsafe demo, tracing aur Goroutine Colony WebSocket bridge add hua tha. Isi commit mein domain aur repository files bhi aaye, lekin HTTP handlers ka storage repository par fully migrate nahi hua tha. Cleanup mein demo/bridge/tracing hata diye gaye aur ye commit local `main` ki history mein ek fresh cleanup commit se replace hua. Pehle ke seven learning commits preserve hain.

Fresh cleanup mein tumhara started repository refactor complete hua: ab **har route ek hi repository** use karta hai. Purana duplicate `holdSeat` helper aur router ki separate storage/locking hata di gayi. Tests ab actual repository aur HTTP API dono check karte hain.

## 3. Ab sirf in teen files ka connection dekho

```text
main.go
  main(): initial data banao, router setup karo, :8080 par server chalao
  setupRouter(): ek repository banao, HTTP routes register karo
       |
       | repo.CreateEvent / repo.AddSeat / repo.HoldSeat / repo.Get...
       v
internal/repository/memory.go
  events, seats, reservations aur next IDs store karta hai
  mutex se shared data protect karta hai
       |
       | records ka shape
       v
internal/domain/models.go
  Event, Seat, User, Reservation structs aur status constants
```

`repository` yahan ek struct hai jo data rakhta hai aur uske operations deta hai. Koi database ya extra service nahi hai. Router JSON/request/status-code handle karta hai; repository actual records read/change karti hai; domain records ka shape batata hai.

`type Seat = domain.Seat` ek alias hai: same type ko short naam mila hai, doosra Seat type create nahi hua.

## 4. Server start hone par kya hota hai

1. Go `main()` call karta hai.
2. Initial event `101` aur uski seats `1`, `2` banti hain. Dono `AVAILABLE` hain.
3. Next IDs set hote hain: event `102`, seat `3`, reservation `501`.
4. `setupRouter(...)` ye initial data `NewMemoryRepository(...)` ko deta hai.
5. Saare route handlers same `repo` pointer use karte hain. Har request par naya repository nahi banta.
6. `router.Run(":8080")` requests sunna shuru karta hai.

Initial `User` sirf sample print hai. User signup, login aur user existence validation implement nahi hain; request ka `user_id` reservation mein store hota hai.

## 5. Ek reservation ko request se response tak follow karo

Request:

```http
POST /reservations
Content-Type: application/json

{"user_id":1,"event_id":101,"seat_id":1}
```

1. Gin URL aur method match karke reservation handler chalata hai.
2. `ShouldBindJSON(&input)` JSON ko `CreateReservationRequest` mein padhta hai. Invalid JSON/type par `400` aata hai. Required IDs ke liye alag validation abhi nahi hai.
3. Handler `repo.HoldSeat(1, 101, 1)` call karta hai.
4. `HoldSeat` mutex lock karta hai. Is repository ka doosra operation tab tak wait karega.
5. Seat ID **aur** event ID match kiye jaate hain. Match nahi mila toh `false, empty reservation` return hota hai: handler `404` deta hai.
6. Seat mili lekin `AVAILABLE` nahi hai toh `true, empty reservation` return hota hai: handler `409` deta hai.
7. Available seat ko `HELD` karo. Reservation `501` banao, status `PENDING` rakho, slice mein append karo aur next reservation ID `502` kar do.
8. `defer r.mu.Unlock()` function return hote waqt lock release karta hai, early return par bhi.
9. Handler reservation JSON ke saath `201 Created` deta hai.

Default seed ke first successful hold ka response:

```json
{"ID":501,"UserID":1,"EventID":101,"SeatID":1,"Status":"PENDING"}
```

Input keys lowercase hain kyunki request struct par JSON tags hain. Response keys capitalized hain kyunki domain structs par JSON tags nahi hain.

## 6. Mutex ki zaroorat kyun padi

HTTP server requests concurrently handle karta hai. Bina lock ke ye interleaving possible hai:

```text
User A: seat AVAILABLE dekhi
User B: seat AVAILABLE dekhi
User A: HELD ki, reservation banayi
User B: HELD ki, doosri reservation banayi
```

Ab poora check + hold + reservation creation **ek lock ke andar** hai:

```text
User A: LOCK -> AVAILABLE dekhi -> HELD + reservation 501 -> UNLOCK -> 201
User B:        lock ka wait ... -> LOCK -> HELD dekhi -> UNLOCK -> 409
```

Lock sirf assignment ke around lagane se kaam nahi chalega: availability check bhi same lock mein hona chahiye. Isi ko atomic operation bol rahe hain: doosri request check aur update ke beech enter nahi kar sakti.

Repository ke reads bhi same mutex lete hain. List methods nayi slice return karti hain, taaki response JSON banne ke waqt doosri request original slice ko safely update kar sake.

Ye mutex ek repository instance ke andar protect karta hai. Multiple server instances ke liye shared database aur usmein atomic reservation logic chahiye hoga.

## 7. Khud chala kar flow dekho

Go version requirement `go.mod` mein `1.26.6` hai.

Terminal 1:

```sh
go run .
```

Terminal 2, fresh server par isi order mein:

```sh
# 1. Initial event aur AVAILABLE seats dekho.
curl -s http://localhost:8080/events
curl -s http://localhost:8080/events/101/seats

# 2. First buyer: 201 aur reservation 501.
curl -i -X POST http://localhost:8080/reservations \
  -H 'Content-Type: application/json' \
  -d '{"user_id":1,"event_id":101,"seat_id":1}'

# 3. Same seat, second buyer: 409 Seat unavailable.
curl -i -X POST http://localhost:8080/reservations \
  -H 'Content-Type: application/json' \
  -d '{"user_id":2,"event_id":101,"seat_id":1}'

# 4. Stored reservation aur updated HELD seat dekho.
curl -s http://localhost:8080/reservations/501
curl -s http://localhost:8080/events/101/seats
```

Same example repeat karna hai toh server stop/start karo; in-memory state reset ho jaayegi.

Tests:

```sh
go test -race ./...
```

Tests check karte hain: repository par 100 concurrent attempts mein ek hold; HTTP par 100 attempts mein ek `201` aur 99 `409`; event create -> seat add -> reservation -> lookup ka connected flow; parallel reads/writes. Race detector shared-memory access bugs detect karta hai; status/state assertions booking rule verify karti hain.

## 8. Baaki endpoints

| Method | Path | Kaam |
|---|---|---|
| GET | `/` | API running message |
| POST | `/events` | `{"name":"Concert"}` se event create |
| GET | `/events` | Saare events |
| GET | `/events/:id` | Ek event |
| POST | `/events/:id/seats` | Existing event mein available seat add; body nahi chahiye |
| GET | `/events/:id/seats` | Event ki seats aur current status |
| POST | `/reservations` | Seat hold aur pending reservation create |
| GET | `/reservations/:id` | Saved reservation |

## 9. Code padhne ka order

1. `internal/domain/models.go`: records ke fields samjho.
2. `main.go` ka `main()`: seed data aur next IDs dekho.
3. `main.go` ka `setupRouter()`: same repo use hote hue dekho.
4. Sirf `POST /reservations` handler padho.
5. `internal/repository/memory.go` ka `HoldSeat` padho aur upar wali request trace karo.
6. `main_test.go` mein `TestAPIRoutes` aur `TestConcurrentHTTPReservations` dekho.

Abhi samajhne ka target: request ka input kahan padha, seat kahan badli, reservation kahan save hui aur response kahan bana. Database/payment baad ka step hai.
