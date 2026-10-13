package service

import (
	"seatrush/internal/domain"
)

type Repository interface {
	HoldSeat(userID, eventID, seatID uint) (bool, domain.Reservation)
	GetReservationByID(id uint) (domain.Reservation, bool)
	CreateEvent(name string) domain.Event
	GetEventByID(id uint) (domain.Event, bool)
	ListEvents() []domain.Event
	AddSeat(eventID uint) domain.Seat
	ListSeatsByEvent(eventID uint) []domain.Seat
}
type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}
func (s *Service) ReserveSeat(
	userID uint,
	eventID uint,
	seatID uint,
) (bool, domain.Reservation) {
	return s.repo.HoldSeat(userID, eventID, seatID)
}
func (s *Service) GetReservationByID(id uint) (domain.Reservation, bool) {
	return s.repo.GetReservationByID(id)
}
func (s *Service) CreateEvent(name string) (domain.Event, bool) {
	if name == "" {
		return domain.Event{}, false
	}
	event := s.repo.CreateEvent(name)
	return event, true

}
func (s *Service) GetEventByID(id uint) (domain.Event, bool) {
	return s.repo.GetEventByID(id)
}
func (s *Service) ListEvents() []domain.Event {
	return s.repo.ListEvents()
}
func (s *Service) AddSeat(eventID uint) (domain.Seat, bool) {
	_, found := s.repo.GetEventByID(eventID)

	if !found {
		return domain.Seat{}, false
	}
	seat := s.repo.AddSeat(eventID)
	return seat, true
}
func (s *Service) ListSeatsByEvent(eventID uint) ([]domain.Seat, bool) {
	_, found := s.repo.GetEventByID(eventID)
	if !found {
		return nil, false
	}
	seats := s.repo.ListSeatsByEvent(eventID)
	return seats, true
}
