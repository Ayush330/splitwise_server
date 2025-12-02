package handlers

import (
	"database/sql"
	"encoding/json"
	_ "log"
	"net/http"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
)

func CreateGroup(res http.ResponseWriter, req *http.Request) {
	var group models.CreateGroupRequest
	var err error
	var Result sql.Result
	if err = json.NewDecoder(req.Body).Decode(&group); err != nil {
		http.Error(res, "Invalid Json", http.StatusBadRequest)
		return
	}
	createGroupError := func(err string) {
		http.Error(res, err, http.StatusInternalServerError)
	}
	tx, err := database.DB.Begin()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	defer tx.Rollback()
	Result, err = tx.Exec("INSERT INTO ugroups (name, owner_id) SELECT ?, id FROM users WHERE uuid = UUID_TO_BIN(?)", group.Name, group.UserId)
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
	Result, err = tx.Exec("INSERT INTO group_membership  SELECT ?, id FROM users WHERE uuid = UUID_TO_BIN(?)", GroupID, group.UserId)
	RowsAffected, err = Result.RowsAffected()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	if RowsAffected == 0 {
		createGroupError("Error")
		return
	}
	if err != nil {
		createGroupError(err.Error())
		return
	}
	err = tx.Commit()
	if err != nil {
		createGroupError(err.Error())
		return
	}
	responseWrapper(http.StatusOK, []byte("Sucessfully created the group!"), res)
}
