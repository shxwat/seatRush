package domain

const (
	StatusAvailable = "AVAILABLE"
	StatusHeld      = "HELD"
	StatusBooked    = "BOOKED"
)

const (
	ReservationStatusPending   = "PENDING"
	ReservationStatusConfirmed = "CONFIRMED"
)

type Seat struct {
	ID      uint
	EventID uint
	Status  string
}

type Event struct {
	ID   uint
	Name string
}

type User struct {
	ID    uint
	Name  string
	Email string
}

type Reservation struct {
	ID      uint
	UserID  uint
	EventID uint
	SeatID  uint
	Status  string
}
