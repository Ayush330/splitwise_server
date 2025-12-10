package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"database/sql"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
)

func Logout(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("Logged Out..."))
}

func CreateUser(res http.ResponseWriter, req *http.Request) {
	var user models.CreateUserRequest
	var err error
	if err = json.NewDecoder(req.Body).Decode(&user); err != nil {
		http.Error(res, "Invalid Json", http.StatusBadRequest)
		return
	}
	createUserError := func(errr string) {
		http.Error(res, errr, http.StatusInternalServerError)
	}
	tx, err := database.DB.Begin()
	if err != nil {
		createUserError(err.Error())
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO users (email, name, password) VALUES (?, ?, ?)", user.Email, user.Name, user.Password)
	if err != nil {
		createUserError(err.Error())
		return
	}
	_, err = tx.Exec("INSERT INTO ugroups (name, owner_id) VALUES(?, (SELECT id FROM users WHERE email = ?))", user.Email, user.Email)
	if err != nil {
		createUserError(err.Error())
		return
	}
	_, err = tx.Exec("INSERT INTO group_membership  SELECT id, owner_id FROM ugroups WHERE name = ?", user.Email)
	if err != nil {
		createUserError(err.Error())
		return
	}
	err = tx.Commit()
	if err != nil {
		createUserError(err.Error())
		return
	}
	res.WriteHeader(http.StatusCreated)
	query := `SELECT id, BIN_TO_UUID(uuid), name, password FROM users WHERE email = ?`
	var userRes struct {
		ID       int
		UUID     string
		Name     string
		Password string
	}
	err = database.DB.QueryRow(query, user.Email).Scan(&userRes.ID, &userRes.UUID, &userRes.Name, &userRes.Password)
	if err == sql.ErrNoRows {
		http.Error(res, "User not found", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Println("Login Error:", err)
		http.Error(res, "Database error", http.StatusInternalServerError)
		return
	}
	res.Header().Set("Content-Type", "application/json")
	json.NewEncoder(res).Encode(map[string]interface{}{
		"message": "Login successful",
		"user": map[string]interface{}{
			"id":    userRes.ID,
			"uuid":  userRes.UUID,
			"name":  userRes.Name,
			"email": user.Email,
		},
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	// 1. Define Request DTO (Inline struct)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 2. Query the Database
	// We need the Password to verify, and the UUID/ID to return to the frontend.
	query := `SELECT id, BIN_TO_UUID(uuid), name, password FROM users WHERE email = ?`

	var user struct {
		ID       int
		UUID     string
		Name     string
		Password string
	}

	err := database.DB.QueryRow(query, req.Email).Scan(&user.ID, &user.UUID, &user.Name, &user.Password)

	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Println("Login Error:", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 3. Verify Password (Plaintext for MVP)
	if user.Password != req.Password {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// 4. Send Success Response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Login successful",
		"user": map[string]interface{}{
			"id":    user.ID,
			"uuid":  user.UUID, // <--- THIS is what Flutter needs!
			"name":  user.Name,
			"email": req.Email,
		},
	})
}
