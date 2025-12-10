package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/models"
	"github.com/go-chi/chi/v5"
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
	// Check if the total split is equal to the total amount
	totalSplit := expense.TotalAmount
	for _, shareHolder := range expense.Shareholders {
		totalSplit -= shareHolder.Amount
	}
	if totalSplit != 0 {
		createExpenseRequestError("Total split is not equal to total amount", http.StatusBadRequest)
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
	queryString := `SELECT 
					BIN_TO_UUID(ug.uuid) AS group_uuid, 
					ug.name AS group_name,
					BIN_TO_UUID(tx.uuid) AS transaction_uuid, 
					tx.description AS transaction_description, 
					e.amount AS user_share_amount, 
					BIN_TO_UUID(u_creator.uuid) AS creator_uuid, 
					u_creator.name AS creator_name,
					tx.amount AS total_transaction_amount
				FROM 
					transactions tx
				JOIN 
					ugroups ug ON tx.rel_group = ug.id
				JOIN 
					expense e ON tx.transaction_id = e.transaction_id  -- FIXED: Changed tx.id to tx.transaction_id
				JOIN 
					users u_shareholder ON e.user_id = u_shareholder.id
				JOIN 
					users u_creator ON tx.creator_id = u_creator.id
				WHERE 
					u_shareholder.uuid = UUID_TO_BIN(?)
				ORDER BY 
					tx.created_at DESC;`
	result, err = tx.Query(queryString, body.UserId)
	log.Println("User Is: ", body.UserId)
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
		Amount                 *int
		TransactionCreatorId   *string
		UserName               *string
		TransactionTotalAmount *int
	}
	var resList = []sqlRes{}

	for result.Next() {
		var row sqlRes
		result.Scan(&row.GroupUuid, &row.GroupName, &row.TransactionUuid, &row.TransactionDescription, &row.Amount, &row.TransactionCreatorId, &row.UserName, &row.TransactionTotalAmount)
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
		Amount                 int    `json:"amount"`
		TransactionCreatorId   string `json:"transaction_creator_id"`
		CreatorName            string `json:"creator_name"`
	}

	type UserExpensesResponse struct {
		GroupId      string              `json:"group_id"`
		GroupName    string              `json:"group_name"`
		GroupAmount  int                 `json:"group_amount"`
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
				GroupAmount:  0,
				Transactions: []transactionStruct{},
			}
		}

		if r.TransactionUuid != nil {
			currentGroup := groupedResponses[groupUUID]

			// Check if transaction already exists in the group to avoid double counting
			transactionExists := false
			for _, t := range currentGroup.Transactions {
				if t.TransactionId == *r.TransactionUuid {
					transactionExists = true
					break
				}
			}

			if !transactionExists {
				currentGroup.Transactions = append(currentGroup.Transactions, transactionStruct{
					TransactionId:          *r.TransactionUuid,
					TransactionDescription: *r.TransactionDescription,
					Amount:                 *r.Amount,
					TransactionCreatorId:   *r.TransactionCreatorId,
					CreatorName:            *r.UserName,
				})
				if r.TransactionTotalAmount != nil {
					currentGroup.GroupAmount += *r.TransactionTotalAmount
				}
			}
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

func GetUserGroups(res http.ResponseWriter, req *http.Request) {
	userID := chi.URLParam(req, "userid")
	errResponse := func(err string, statusCode int) {
		http.Error(res, err, statusCode)
	}
	query := `
				WITH USERIDCTE AS (
					SELECT id, email FROM users WHERE uuid = UUID_TO_BIN(?)
				),
				GROUPCTE AS (
					SELECT group_id FROM group_membership WHERE user_id = (SELECT id FROM USERIDCTE)
				),
				PAID_CTE AS (
					SELECT rel_group AS group_id, SUM(amount) AS total_paid
					FROM transactions
					WHERE creator_id = (SELECT id FROM USERIDCTE)
					GROUP BY rel_group
				),
				PRELEDGER_CTE AS(
					SELECT t.rel_group AS group_id, t.transaction_id AS transaction_id, e.amount AS amount
					FROM transactions t
					JOIN expense e ON t.transaction_id = e.transaction_id
					AND t.rel_group IN (SELECT group_id FROM GROUPCTE)
					AND e.user_id = (SELECT id FROM USERIDCTE)
				),
				LEDGER_CTE AS (
					SELECT group_id, SUM(amount) AS total_ledger
					FROM PRELEDGER_CTE
					GROUP BY group_id
				)
				SELECT 
					BIN_TO_UUID(u.uuid) AS group_uuid, 
					CASE 
						WHEN u.name = (SELECT email FROM USERIDCTE) THEN 'Non-Group Expenses'
						ELSE u.name
					END AS group_name,
					COALESCE(p.total_paid, 0) + COALESCE(l.total_ledger, 0) AS balance 
					FROM ugroups u
					LEFT JOIN PAID_CTE p ON u.id = p.group_id
					LEFT JOIN LEDGER_CTE l ON u.id = l.group_id
					WHERE u.id IN (SELECT group_id FROM GROUPCTE)`
	result, err := database.DB.Query(query, userID)
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Close()
	type queryResponse struct {
		GroupId   *string `json:"group_id"`
		GroupName *string `json:"group_name"`
		Balance   *int    `json:"balance"`
	}
	var finalResponse []queryResponse
	for result.Next() {
		var tempRes queryResponse
		err = result.Scan(&tempRes.GroupId, &tempRes.GroupName, &tempRes.Balance)
		if err != nil {
			log.Println(err.Error())
		} else {
			finalResponse = append(finalResponse, tempRes)
		}
	}
	type responseStruct struct {
		Response []queryResponse `json:"response"`
		Message  string          `json:"message"`
	}
	f, err := json.Marshal(responseStruct{Response: finalResponse, Message: "Groups Successfully Fetched!"})
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	responseWrapper(http.StatusOK, f, res)
}

func GetUserGroupExpenseDetails(res http.ResponseWriter, req *http.Request) {
	var err error
	errResponse := func(err string, statusCode int) {
		http.Error(res, err, statusCode)
	}
	userID := chi.URLParam(req, "userid")
	groupID := chi.URLParam(req, "groupid")
	query := `
				WITH USERIDCTE AS(
						SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)
					),
					GROUPCTE AS(
						SELECT id FROM ugroups WHERE uuid = UUID_TO_BIN(?)
					)
				SELECT CASE WHEN e.amount > 0 THEN 'settlement' ELSE 'expense' END AS type, BIN_TO_UUID(t.uuid) AS transaction_uuid, t.amount, e.amount, t.description
				FROM transactions AS t
				JOIN expense AS e ON e.transaction_id = t.transaction_id
				WHERE t.rel_group = (SELECT id from GROUPCTE)
				AND e.user_id = (SELECT id FROM USERIDCTE)
			`
	result, err := database.DB.Query(query, userID, groupID)
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Close()
	type queryResponse struct {
		Type            string `json:"type"`
		TransactionUuid string `json:"transaction_uuid"`
		TotalAmount     int    `json:"total_amount"`
		Amount          int    `json:"myshare"`
		Description     string `json:"description"`
	}
	var finalResponse []queryResponse
	for result.Next() {
		var tempRes queryResponse
		err = result.Scan(&tempRes.Type, &tempRes.TransactionUuid, &tempRes.TotalAmount, &tempRes.Amount, &tempRes.Description)
		if err != nil {
			log.Println(err.Error())
		} else {
			finalResponse = append(finalResponse, tempRes)
		}
	}
	log.Println("[GetUserGroupExpenseDetails]->finalResponse: ", finalResponse)
	type responseStruct struct {
		Response []queryResponse `json:"response"`
		Message  string          `json:"message"`
	}
	f, err := json.Marshal(responseStruct{Response: finalResponse, Message: "Expenses Successfully Fetched!"})
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	responseWrapper(http.StatusOK, f, res)
}
