package main

import (
	"seatrush/internal/app"
	"seatrush/internal/domain"
	"seatrush/internal/handler"
	"seatrush/internal/repository"
	"seatrush/internal/service"
)

func main() {
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
