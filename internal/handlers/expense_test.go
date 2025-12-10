package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetUserGroupAndTheirExpenses(t *testing.T) {
	// Mock the database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Replace the global DB variable
	database.DB = db

	// Request body
	reqBody := models.UserExpensesRequest{
		UserId: "user-uuid-1",
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "/expenses", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// Expected SQL query
	// Note: sqlmock uses regex for matching, so we need to be careful with special characters
	query := `WITH usercte AS.*`

	// Mock rows
	rows := sqlmock.NewRows([]string{
		"group_uuid", "group_name", "transaction_uuid", "transaction_description", "amount", "user_uuid", "name", "transaction_total_amount",
	}).
		AddRow("group-uuid-1", "Group 1", "tx-uuid-1", "Lunch", 100, "user-uuid-1", "User 1", 300).
		AddRow("group-uuid-1", "Group 1", "tx-uuid-1", "Lunch", 100, "user-uuid-2", "User 2", 300). // Same transaction, different user share
		AddRow("group-uuid-1", "Group 1", "tx-uuid-2", "Dinner", 200, "user-uuid-1", "User 1", 500)

	mock.ExpectBegin()
	mock.ExpectQuery(query).WithArgs(reqBody.UserId).WillReturnRows(rows)
	mock.ExpectCommit()

	// Call the handler
	GetUserGroupAndTheirExpenses(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response body
	var response struct {
		Response []struct {
			GroupId      string `json:"group_id"`
			GroupAmount  int    `json:"group_amount"`
			Transactions []struct {
				TransactionId string `json:"transaction_id"`
			} `json:"transactions"`
		} `json:"response"`
	}

	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	if len(response.Response) != 1 {
		t.Fatalf("expected 1 group, got %d", len(response.Response))
	}

	group := response.Response[0]
	if group.GroupId != "group-uuid-1" {
		t.Errorf("expected group_id 'group-uuid-1', got '%s'", group.GroupId)
	}

	// Expected GroupAmount: 300 (tx-1) + 500 (tx-2) = 800
	// Note: tx-1 appears twice in rows, but should be counted once.
	if group.GroupAmount != 800 {
		t.Errorf("expected group_amount 800, got %d", group.GroupAmount)
	}

	if len(group.Transactions) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(group.Transactions))
	}

	// Verify expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
