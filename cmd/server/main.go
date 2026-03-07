package main

import (
	"log"
	"net/http"
	"time"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/handlers"
	"github.com/Ayush330/splitwise_server/internal/hub"
	"github.com/Ayush330/splitwise_server/internal/middleware/authentication"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	log.Println("Creating the splitwise")
	database.Connect()
	h := hub.NewHub()
	go h.Run()
	handlers.SetHub(h) // Inject hub into handlers

	Router := chi.NewRouter()

	// CORS must be the first middleware to handle preflight OPTIONS requests properly
	Router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Public routes (no auth required)
	Router.Post("/createUser", handlers.CreateUser)
	Router.Post("/login", handlers.Login)

	// Protected routes (JWT required)
	Router.Group(func(r chi.Router) {
		r.Use(authentication.JWTMiddleware)
		r.Post("/createGroup", handlers.CreateGroup)
		r.Post("/createExpense", handlers.AddExpense)
		r.Post("/userExpenseData", handlers.GetUserGroupAndTheirExpenses)
		r.Get("/users/{userid}/groups", handlers.GetUserGroups)
		r.Get("/users/{userid}/groups/{groupid}/expenses", handlers.GetUserGroupExpenseDetails)
		r.Get("/users/{userid}/groups/{groupid}/members", handlers.GetMembersOfGroup)
		r.Get("/users/{userid}/groups/{groupid}/addMember/{email}", handlers.AddMemberToGroup)
		r.Get("/users/{userid}/balances", handlers.GetUserBalances)
		r.Get("/users/{userid}/activity", handlers.GetUserActivity)
		r.Delete("/expenses/{expenseid}", handlers.DeleteExpense)

		// WebSocket endpoint
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			userDBID, _ := r.Context().Value(authentication.UserIDKey).(string)
			if userDBID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// Resolve numeric DB ID to UUID for Hub registration
			var userUUID string
			err := database.DB.QueryRow("SELECT BIN_TO_UUID(uuid) FROM users WHERE id = ?", userDBID).Scan(&userUUID)
			if err != nil {
				log.Printf("WebSocket: failed to resolve UUID for user %s: %v", userDBID, err)
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
			log.Printf("WebSocket: user %s connected with UUID %s", userDBID, userUUID)
			h.ServeWs(w, r, userUUID)
		})
	})
	server := &http.Server{
		Addr:           ":8080",
		Handler:        Router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(server.ListenAndServe())
}
