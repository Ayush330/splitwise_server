package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
	"github.com/go-chi/chi/v5"
)

func CreateGroup(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var group models.CreateGroupRequest
	var err error
	var Result sql.Result
	if err = json.NewDecoder(req.Body).Decode(&group); err != nil {
		jsonError(res, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !isValidName(group.Name) {
		jsonError(res, "Group name must be between 1 and 100 characters", http.StatusBadRequest)
		return
	}
	createGroupError := func(err string) {
		jsonError(res, err, http.StatusInternalServerError)
	}
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		createGroupError(err.Error())
		return
	}
	defer tx.Rollback()
	Result, err = tx.ExecContext(ctx, "INSERT INTO ugroups (name, owner_id) SELECT ?, id FROM users WHERE uuid = UUID_TO_BIN(?)", group.Name, group.UserId)
	if err != nil {
		createGroupError(err.Error())
		return
	}
	RowsAffected, err := Result.RowsAffected()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	if RowsAffected == 0 {
		createGroupError("Error")
		return
	}
	GroupID, err := Result.LastInsertId()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	Result, _ = tx.ExecContext(ctx, "INSERT INTO group_membership  SELECT ?, id FROM users WHERE uuid = UUID_TO_BIN(?)", GroupID, group.UserId)
	RowsAffected, err = Result.RowsAffected()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	if RowsAffected == 0 {
		createGroupError("Error")
		return
	}
	err = tx.Commit()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	responseWrapper(http.StatusOK, []byte("Sucessfully created the group!"), res)
}

func AddMemberToGroup(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var err error
	errorResponse := func(statusCode int, msg string) {
		jsonError(res, msg, statusCode)
	}
	email := chi.URLParam(req, "email")
	groupUUID := chi.URLParam(req, "groupid")
	log.Println("[AYUSH]: AddMemberToGroup", email, groupUUID)
	name, err := UsernameFromEmail(email)
	if err != nil {
		errorResponse(http.StatusBadRequest, err.Error())
		return
	}
	txn, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Println("[AYUSH]: AddMemberToGroupError", err)
		errorResponse(http.StatusInternalServerError, err.Error())
		return
	}
	defer txn.Rollback()
	insertUserQuery := `
        INSERT IGNORE INTO users (email, name, password)
        VALUES (?, ?, NULL)
    `
	_, err = txn.ExecContext(ctx, insertUserQuery, email, name)
	if err != nil {
		errorResponse(http.StatusInternalServerError, "Failed to ensure user exists: "+err.Error())
		return
	}
	insertGroupQuery := `
        INSERT IGNORE INTO group_membership (group_id, user_id)
        SELECT g.id, u.id
        FROM ugroups g
        JOIN users u ON u.email = ?
        WHERE g.uuid = UUID_TO_BIN(?)
    `
	result, err := txn.ExecContext(ctx, insertGroupQuery, email, groupUUID)
	if err != nil {
		errorResponse(http.StatusInternalServerError, "Failed to add member: "+err.Error())
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		log.Println("[AYUSH]: User already in group or Group not found.")
	}
	err = txn.Commit()
	if err != nil {
		errorResponse(http.StatusInternalServerError, err.Error())
		return
	}

	responseWrapper(http.StatusOK, []byte(`{"message": "Member processed successfully"}`), res)
}

func UsernameFromEmail(email string) (string, error) {
	if email == "" {
		return "", errors.New("empty email")
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 { // at==0 => no local part, at==-1 => no @
		return "", errors.New("invalid email: missing or invalid '@' position")
	}
	return email[:at], nil
}
