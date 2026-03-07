package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"database/sql"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/middleware/authentication"
	"github.com/Ayush330/splitwise_server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func Logout(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("Logged Out..."))
}

func CreateUser(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var user models.CreateUserRequest
	var err error
	if err = json.NewDecoder(req.Body).Decode(&user); err != nil {
		jsonError(res, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !isValidEmail(user.Email) {
		jsonError(res, "Invalid email format", http.StatusBadRequest)
		return
	}
	if !isValidPassword(user.Password) {
		jsonError(res, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if !isValidName(user.Name) {
		jsonError(res, "Name must be between 1 and 100 characters", http.StatusBadRequest)
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(res, "Failed to process password", http.StatusInternalServerError)
		return
	}
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "INSERT INTO users (email, name, password) VALUES (?, ?, ?)", user.Email, user.Name, string(hashedPassword))
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO ugroups (name, owner_id) VALUES(?, (SELECT id FROM users WHERE email = ?))", user.Email, user.Email)
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO group_membership  SELECT id, owner_id FROM ugroups WHERE name = ?", user.Email)
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tx.Commit()
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	query := `SELECT id, BIN_TO_UUID(uuid), name FROM users WHERE email = ?`
	var userRes struct {
		ID   int
		UUID string
		Name string
	}
	err = database.DB.QueryRowContext(ctx, query, user.Email).Scan(&userRes.ID, &userRes.UUID, &userRes.Name)
	if err == sql.ErrNoRows {
		jsonError(res, "User not found", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Println("Signup Error:", err)
		jsonError(res, "Database error", http.StatusInternalServerError)
		return
	}
	token, err := authentication.CreateToken(fmt.Sprintf("%d", userRes.ID))
	if err != nil {
		jsonError(res, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusCreated)
	json.NewEncoder(res).Encode(map[string]interface{}{
		"message": "Signup successful",
		"token":   token,
		"user": map[string]interface{}{
			"id":    userRes.ID,
			"uuid":  userRes.UUID,
			"name":  userRes.Name,
			"email": user.Email,
		},
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !isValidEmail(req.Email) {
		jsonError(w, "Invalid email format", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		jsonError(w, "Password is required", http.StatusBadRequest)
		return
	}

	query := `SELECT id, BIN_TO_UUID(uuid), name, password FROM users WHERE email = ?`

	var user struct {
		ID       int
		UUID     string
		Name     string
		Password string
	}

	err := database.DB.QueryRowContext(ctx, query, req.Email).Scan(&user.ID, &user.UUID, &user.Name, &user.Password)

	if err == sql.ErrNoRows {
		jsonError(w, "User not found", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Println("Login Error:", err)
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		jsonError(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	token, err := authentication.CreateToken(fmt.Sprintf("%d", user.ID))
	if err != nil {
		jsonError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Login successful",
		"token":   token,
		"user": map[string]interface{}{
			"id":    user.ID,
			"uuid":  user.UUID,
			"name":  user.Name,
			"email": req.Email,
		},
	})
}
