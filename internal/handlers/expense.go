package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
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
	if RowsUpdated == 0 || err != nil {
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
	var response []byte
	errResponse = func(err string, statusCode int) {
		http.Error(res, err, statusCode)
	}
	if err = json.NewDecoder(req.Body).Decode(&body); err != nil {
		errResponse(err.Error(), http.StatusBadRequest)
		return
	}
	tx, err = database.DB.Begin()
	if err != nil {
		errResponse("Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	var result *sql.Rows
	queryString := `WITH usercte AS
						(SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)),
						usergroupcte AS
						(SELECT group_id FROM group_membership WHERE user_id IN (SELECT id FROM usercte)),
						groupcte AS
						(SELECT id, uuid, name FROM ugroups WHERE id IN (SELECT group_id FROM usergroupcte)),
					 	transactioncte AS
						(SELECT g.id AS group_id, g.uuid AS group_uuid, g.name AS group_name, tx.transaction_id AS transaction_id, tx.uuid AS transaction_uuid, tx.description AS transaction_description, tx.creator_id AS transaction_creator_id FROM transactions AS tx INNER JOIN groupcte AS g ON tx.rel_group = g.id),
						finalcte AS
						(SELECT u.uuid AS user_uuid, u.name, t.group_id AS group_id, t.group_uuid AS group_uuid, t.group_name AS group_name, t.transaction_id AS transaction_id, t.transaction_uuid AS transaction_uuid, t.transaction_description AS transaction_description, BIN_TO_UUID(t.transaction_creator_id) AS transaction_creator_id FROM users AS u INNER JOIN transactioncte AS t ON u.id = t.transaction_creator_id)	
					SELECT BIN_TO_UUID(t.group_uuid) AS group_uuid, t.group_name,  BIN_TO_UUID(t.transaction_uuid) AS transaction_uuid, t.transaction_description, BIN_TO_UUID(t.user_uuid) AS user_uuid, t.name FROM expense as e INNER JOIN finalcte AS t ON e.transaction_id = t.transaction_id
					`
	result, err = tx.Query(queryString, body.UserId)
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Close()
	type sqlRes struct {
		GroupUuid              *string
		GroupName              *string
		TransactionUuid        *string
		TransactionDescription *string
		TransactionCreatorId   *string
		UserName               *string
	}
	var resList = []sqlRes{}
	var row sqlRes
	for result.Next() {
		result.Scan(&row.GroupUuid, &row.GroupName, &row.TransactionUuid, &row.TransactionDescription, &row.TransactionCreatorId)
		resList = append(resList, row)
	}
	log.Println(resList)
	err = tx.Commit()
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	//convert to response data

	type transactionStruct struct {
		TransactionId          string `json:"transaction_id"`
		TransactionDescription string `json:"transaction_description"`
		TransactionCreatorId   string `json:"transaction_creator_id"`
		CreatorName            string `json:"creator_name"`
	}

	type UserExpensesResponse struct {
		GroupId      string              `json:"group_id"`
		GroupName    string              `json:"group_name"`
		Transactions []transactionStruct `json:"transactions"`
	}

	groupedResponses := make(map[string]UserExpensesResponse)

	for _, r := range resList {
		if r.GroupUuid == nil || r.GroupName == nil {
			continue
		}
		groupUUID := *r.GroupUuid
		groupName := *r.GroupName

		if _, ok := groupedResponses[groupUUID]; !ok {
			groupedResponses[groupUUID] = UserExpensesResponse{
				GroupId:      groupUUID,
				GroupName:    groupName,
				Transactions: []transactionStruct{},
			}
		}

		if r.TransactionUuid != nil {
			currentGroup := groupedResponses[groupUUID]
			currentGroup.Transactions = append(currentGroup.Transactions, transactionStruct{
				TransactionId:          *r.TransactionUuid,
				TransactionDescription: *r.TransactionDescription,
				TransactionCreatorId:   *r.TransactionCreatorId,
				CreatorName:            *r.UserName,
			})
			groupedResponses[groupUUID] = currentGroup
		}
	}

	var finalResponse []UserExpensesResponse
	for _, v := range groupedResponses {
		finalResponse = append(finalResponse, v)
	}

	output := struct {
		Response []UserExpensesResponse `json:"response"`
	}{
		Response: finalResponse,
	}
	response, err = json.Marshal(output)
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	responseWrapper(http.StatusOK, response, res)
}
