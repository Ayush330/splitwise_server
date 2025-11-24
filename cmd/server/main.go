package main

import (
	"log"
	"net/http"
	"time"
	"github.com/go-chi/chi/v5" // Import the library
	"github.com/Ayush330/splitwise_server/internal/handlers"
	"github.com/Ayush330/splitwise_server/internal/database"
)

func main() {
	log.Println("Creating the splitwise")
	database.Connect()
	Router := chi.NewRouter()
	Router.Post("/createUser", handlers.CreateUser)
	Router.Get("/login", handlers.Login)
	Router.Get("/logout", handlers.Logout)
	server := &http.Server{
		Addr:           ":8080",
		Handler:        Router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	server.ListenAndServe()
}

