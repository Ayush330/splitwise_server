package main

import (
	"log"
	"net/http"
	"time"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/handlers"
	"github.com/go-chi/chi/v5" // Import the library
)

func main() {
	log.Println("Creating the splitwise")
	database.Connect()
	Router := chi.NewRouter()
	Router.Post("/createUser", handlers.CreateUser)
	Router.Post("/createGroup", handlers.CreateGroup)
	Router.Post("/createExpense", handlers.AddExpense)
	Router.Post("/userExpenseData", handlers.GetUserGroupAndTheirExpenses)
	//Router.Get("/login", handlers.Login)
	//Router.Get("/logout", handlers.Logout)
	server := &http.Server{
		Addr:           ":8080",
		Handler:        Router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(server.ListenAndServe())
}
