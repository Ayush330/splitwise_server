package handlers

import (
	"encoding/json"
	_ "log"
	"net/http"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
)

func Login(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("Logged In..."))
}

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
	res.Write([]byte("Received user: " + user.Name))
}
