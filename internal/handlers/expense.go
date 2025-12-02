package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
)

func AddExpense(res http.ResponseWriter, req *http.Request) {
	var err error
	var expense models.CreateExpenseRequest
	var tx *sql.Tx
	var result sql.Result
	createExpenseRequestError := func(err string, statusCode int) {
		http.Error(res, err, statusCode)
	}
	if err = json.NewDecoder(req.Body).Decode(&expense); err != nil {
		createExpenseRequestError(err.Error(), http.StatusBadRequest)
		return
	}
	tx, err = database.DB.Begin()
	defer tx.Rollback()
	if err != nil {
		createExpenseRequestError(err.Error(), http.StatusInternalServerError)
		return
	}
	result, err = tx.Exec("INSERT INTO transactions (amount, description, creator_id, rel_group) VALUES (?, ?, (SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)), (SELECT id from ugroups WHERE uuid = UUID_TO_BIN(?)))", expense.TotalAmount, expense.Description, expense.PayerId, expense.GroupId)
	if err != nil {
		createExpenseRequestError(err.Error(), http.StatusInternalServerError)
		return
	}
	RowsUpdated, err := result.RowsAffected()
	if RowsUpdated == 0 {
		createExpenseRequestError("Cannot find group or user", http.StatusInternalServerError)
		return
	}
	TransactionId, err := result.LastInsertId()
	if err != nil {
		createExpenseRequestError(err.Error(), http.StatusInternalServerError)
		return
	}
	for _, shareHolder := range expense.Shareholders {
		result, err = tx.Exec("INSERT INTO expense (transaction_id, user_id, amount) SELECT ?, id, ? FROM users WHERE uuid =  UUID_TO_BIN(?)", TransactionId, shareHolder.Amount, shareHolder.UserId)
		if err != nil {
			createExpenseRequestError(err.Error(), http.StatusInternalServerError)
			return
		}
		RowsUpdated, err = result.RowsAffected()
		if err != nil {
			createExpenseRequestError(err.Error(), http.StatusInternalServerError)
			return
		}
		if RowsUpdated == 0 {
			createExpenseRequestError("Cannot find group or user", http.StatusInternalServerError)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		createExpenseRequestError(err.Error(), http.StatusInternalServerError)
		return
	}
	responseWrapper(http.StatusOK, []byte("Successfully added the expense"), res)
}

func GetUserGroupAndTheirExpenses(res http.ResponseWriter, req *http.Request) {
	var err error
	var body models.UserExpensesRequest
	var tx *sql.Tx
	var errResponse func(string, int)
	errResponse = func(err string, statusCode int) {
		http.Error(res, err, statusCode)
	}
	tx, err = database.DB.Begin()
	if err != nil {
		errResponse("Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	var result *sql.Rows
	result, err = tx.Query("SELECT ")
}
