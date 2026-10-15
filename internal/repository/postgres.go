package repository

import (
	"database/sql"
	"seatrush/internal/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}
func (r *PostgresRepository) GetEventByID(id uint) (domain.Event, error) {
	var event domain.Event

	err := r.db.QueryRow(
		"SELECT id, name FROM events WHERE id = $1",
		id,
	).Scan(
		&event.ID,
		&event.Name,
	)
	return event, err
}
func (r *PostgresRepository) CreateEvent(name string) (domain.Event, error) {
	var event domain.Event

	err := r.db.QueryRow(
		"INSERT INTO events (name) VALUES ($1) RETURNING id, name",
		name,
	).Scan(
		&event.ID,
		&event.Name,
	)
	return event, err
}
func (r *PostgresRepository) ListEvents() ([]domain.Event, error) {
	rows, err := r.db.Query(
		"SELECT id, name FROM events ORDER BY id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.Event

	for rows.Next() {
		var event domain.Event

		if err := rows.Scan(&event.ID, &event.Name); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()

}
func (r *PostgresRepository) GetReservationByID(id uint) (domain.Reservation, error) {
	var reservation domain.Reservation

	err := r.db.QueryRow(
		"SELECT id, user_id, event_id, seat_id, status FROM reservations WHERE id = $1",
		id,
	).Scan(
		&reservation.ID,
		&reservation.UserID,
		&reservation.EventID,
		&reservation.SeatID,
		&reservation.Status,
	)
	return reservation, err
}
