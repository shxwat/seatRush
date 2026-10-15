package main

import (
	"database/sql"
	"fmt"
	"seatrush/internal/app"
	"seatrush/internal/domain"
	"seatrush/internal/handler"
	"seatrush/internal/repository"
	"seatrush/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := sql.Open(
		"pgx",
		"host=/tmp user=shashwat dbname=seatrush sslmode=disable",
	)
	if err != nil {
		panic(err)
	}

	if err := db.Ping(); err != nil {
		panic(err)
	}
	pgRepo := repository.NewPostgresRepository(db)

	postgresEvents, err := pgRepo.ListEvents()
	if err != nil {
		panic(err)
	}
	fmt.Println("Event list", postgresEvents)

	defer db.Close()
	events := []domain.Event{
		{ID: 101, Name: "Tech Conference"},
	}

	seats := []domain.Seat{
		{ID: 1, EventID: 101, Status: domain.StatusAvailable},
		{ID: 2, EventID: 101, Status: domain.StatusAvailable},
	}

	repo := repository.NewMemoryRepository(events, seats, nil, 102, 3, 501)
	svc := service.New(repo)
	httpHandler := handler.New(svc)
	router := app.NewRouter(httpHandler)

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
