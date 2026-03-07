package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Ayush330/splitwise_server/internal/database"
	"github.com/Ayush330/splitwise_server/internal/hub"
	"github.com/Ayush330/splitwise_server/internal/middleware/authentication"
	"github.com/Ayush330/splitwise_server/internal/models"
	"github.com/go-chi/chi/v5"
)

var hhub *hub.Hub

func SetHub(h *hub.Hub) {
	hhub = h
}

func broadcastUpdate(userUUIDs []string) {
	if hhub != nil {
		hhub.BroadcastToUsers(userUUIDs, map[string]string{"type": "update"})
	}
}

func AddExpense(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var err error
	var expense models.CreateExpenseRequest
	var tx *sql.Tx
	var result sql.Result
	createExpenseRequestError := func(err string, statusCode int) {
		jsonError(res, err, statusCode)
	}
	if err = json.NewDecoder(req.Body).Decode(&expense); err != nil {
		log.Println("[AYUSH]: AddExpenseError", err)
		createExpenseRequestError(err.Error(), http.StatusBadRequest)
		return
	}
	// Check if the total split is equal to the total amount
	log.Println("[AYUSH]: AddExpense", expense)
	totalSplit := expense.TotalAmount
	for _, shareHolder := range expense.Shareholders {
		totalSplit += shareHolder.Amount
	}
	if totalSplit != 0 {
		createExpenseRequestError("Total split is not equal to total amount", http.StatusBadRequest)
		return
	}
	tx, err = database.DB.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		createExpenseRequestError(err.Error(), http.StatusInternalServerError)
		return
	}
	result, err = tx.ExecContext(ctx, "INSERT INTO transactions (amount, description, creator_id, rel_group) VALUES (?, ?, (SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)), (SELECT id from ugroups WHERE uuid = UUID_TO_BIN(?)))", expense.TotalAmount, expense.Description, expense.PayerId, expense.GroupId)
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
		result, err = tx.ExecContext(ctx, "INSERT INTO expense (transaction_id, user_id, amount) SELECT ?, id, ? FROM users WHERE uuid =  UUID_TO_BIN(?)", TransactionId, shareHolder.Amount, shareHolder.UserId)
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

	// Broadcast updates to all group members
	var groupMembers []string
	memberQuery := `SELECT BIN_TO_UUID(u.uuid) 
					FROM group_membership gm 
					JOIN users u ON gm.user_id = u.id 
					WHERE gm.group_id = (SELECT id FROM ugroups WHERE uuid = UUID_TO_BIN(?))`
	rows, err := database.DB.QueryContext(ctx, memberQuery, expense.GroupId)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mUUID string
			if err := rows.Scan(&mUUID); err == nil {
				groupMembers = append(groupMembers, mUUID)
			}
		}
	}
	if len(groupMembers) > 0 {
		broadcastUpdate(groupMembers)
	} else {
		// Fallback to involved users
		involvedUsers := []string{expense.PayerId}
		for _, s := range expense.Shareholders {
			involvedUsers = append(involvedUsers, s.UserId)
		}
		broadcastUpdate(involvedUsers)
	}
}

func GetUserGroupAndTheirExpenses(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var err error
	var body models.UserExpensesRequest
	var tx *sql.Tx
	var errResponse func(string, int)
	var response []byte
	errResponse = func(err string, statusCode int) {
		jsonError(res, err, statusCode)
	}
	if err = json.NewDecoder(req.Body).Decode(&body); err != nil {
		errResponse(err.Error(), http.StatusBadRequest)
		return
	}
	tx, err = database.DB.BeginTx(ctx, nil)
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
					expense e ON tx.transaction_id = e.transaction_id  
				JOIN 
					users u_shareholder ON e.user_id = u_shareholder.id
				JOIN 
					users u_creator ON tx.creator_id = u_creator.id
				WHERE 
					u_shareholder.uuid = UUID_TO_BIN(?)
				ORDER BY 
					tx.created_at DESC;`
	result, err = tx.QueryContext(ctx, queryString, body.UserId)
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
	ctx := req.Context()
	userID := chi.URLParam(req, "userid")
	errResponse := func(err string, statusCode int) {
		jsonError(res, err, statusCode)
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
	result, err := database.DB.QueryContext(ctx, query, userID)
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
	ctx := req.Context()
	var err error
	errResponse := func(err string, statusCode int) {
		jsonError(res, err, statusCode)
	}
	userID := chi.URLParam(req, "userid")
	groupID := chi.URLParam(req, "groupid")
	query := `
				WITH USERIDCTE AS (
					SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)
				),
				GROUPCTE AS (
					SELECT id FROM ugroups WHERE uuid = UUID_TO_BIN(?)
				)
				SELECT 
					BIN_TO_UUID(t.uuid) AS transaction_id,
					t.description,
					t.amount AS total_amount,
					t.created_at,
					creator.name AS payer_name, -- Using Creator Name as Payer Name

					-- 1. Did I Pay? (Checking creator_id instead of payer_id)
					(t.creator_id = (SELECT id FROM USERIDCTE)) AS did_i_pay,

					-- 2. My Net Balance calculation
					(
						(CASE WHEN t.creator_id = (SELECT id FROM USERIDCTE) THEN t.amount ELSE 0 END)
						+
						COALESCE(e.amount, 0)
					) AS my_net_balance,

					-- 3. Am I involved?
					(e.user_id IS NOT NULL OR t.creator_id = (SELECT id FROM USERIDCTE)) AS is_involved

				FROM transactions t
				JOIN users creator ON creator.id = t.creator_id
				
				-- Get my specific expense share (if any)
				LEFT JOIN expense e 
					ON e.transaction_id = t.transaction_id 
					AND e.user_id = (SELECT id FROM USERIDCTE)

				WHERE t.rel_group = (SELECT id FROM GROUPCTE)
				ORDER BY t.created_at DESC;
`
	result, err := database.DB.QueryContext(ctx, query, userID, groupID)
	if err != nil {
		errResponse(err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Close()
	type queryResponse struct {
		TransactionID string `json:"transaction_id"`
		Description   string `json:"description"`
		TotalAmount   int    `json:"total_amount"`
		CreatedAt     string `json:"created_at"`
		PayerName     string `json:"payer_name"`
		DidIPay       bool   `json:"did_i_pay"`
		MyNetBalance  int    `json:"my_net_balance"`
		IsInvolved    bool   `json:"is_involved"`
	}
	var finalResponse []queryResponse
	for result.Next() {
		var tempRes queryResponse
		// Scan matches the query order:
		// 1. transaction_id (string)
		// 2. description (string)
		// 3. total_amount (int)
		// 4. created_at (string/time)
		// 5. payer_name (string)
		// 6. did_i_pay (bool)
		// 7. my_net_balance (int)
		// 8. is_involved (bool)
		err = result.Scan(
			&tempRes.TransactionID,
			&tempRes.Description,
			&tempRes.TotalAmount,
			&tempRes.CreatedAt,
			&tempRes.PayerName,
			&tempRes.DidIPay,
			&tempRes.MyNetBalance,
			&tempRes.IsInvolved,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
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

func GetMembersOfGroup(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var err error
	errorResponse := func(statusCode int, err string) {
		jsonError(res, err, statusCode)
	}
	//userId := chi.URLParam(req, "userid")
	groupId := chi.URLParam(req, "groupid")

	query := `
				WITH GROUPIDCTE AS (
					SELECT id FROM ugroups WHERE uuid = UUID_TO_BIN(?)
				),
				MEMBERS_CTE AS (
					SELECT user_id FROM group_membership WHERE group_id = (SELECT id FROM GROUPIDCTE)
				),
				PAID_CTE AS (
					SELECT creator_id AS user_id, SUM(amount) AS total_paid
					FROM transactions
					WHERE rel_group = (SELECT id FROM GROUPIDCTE)
					GROUP BY creator_id
				),
				OWED_CTE AS (
					SELECT e.user_id, SUM(e.amount) AS total_owed
					FROM expense e
					JOIN transactions t ON e.transaction_id = t.transaction_id
					WHERE t.rel_group = (SELECT id FROM GROUPIDCTE)
					GROUP BY e.user_id
				)
				SELECT 
					BIN_TO_UUID(u.uuid) AS uuid, 
					u.name, 
					u.email,
					COALESCE(p.total_paid, 0) + COALESCE(o.total_owed, 0) AS balance
				FROM users u
				JOIN MEMBERS_CTE m ON u.id = m.user_id
				LEFT JOIN PAID_CTE p ON u.id = p.user_id
				LEFT JOIN OWED_CTE o ON u.id = o.user_id;
			`
	result, err := database.DB.QueryContext(ctx, query, groupId)
	if err != nil {
		errorResponse(http.StatusInternalServerError, err.Error())
		return
	}
	defer result.Close()
	type queryResponse struct {
		Uuid    string `json:"uuid"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Balance int    `json:"balance"`
	}
	var finalResponse []queryResponse
	for result.Next() {
		var tempRes queryResponse
		err = result.Scan(&tempRes.Uuid, &tempRes.Name, &tempRes.Email, &tempRes.Balance)
		if err != nil {
			log.Println(err.Error())
		} else {
			finalResponse = append(finalResponse, tempRes)
		}
	}
	log.Println("[GetMembersOfGroup]->finalResponse: ", finalResponse)
	type responseStruct struct {
		Response []queryResponse `json:"response"`
		Message  string          `json:"message"`
	}
	f, err := json.Marshal(responseStruct{Response: finalResponse, Message: "Members Successfully Fetched!"})
	if err != nil {
		errorResponse(http.StatusInternalServerError, err.Error())
		return
	}
	responseWrapper(http.StatusOK, f, res)
}

func GetUserBalances(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	userID := chi.URLParam(req, "userid")

	query := `
		WITH my_id AS (
			SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)
		),
		-- Expenses where others paid and I have shares
		my_shares AS (
			SELECT t.creator_id AS friend_id, t.rel_group, SUM(e.amount) AS total
			FROM expense e
			JOIN transactions t ON t.transaction_id = e.transaction_id
			WHERE e.user_id = (SELECT id FROM my_id)
			  AND t.creator_id != (SELECT id FROM my_id)
			GROUP BY t.creator_id, t.rel_group
		),
		-- Expenses where I paid and others have shares
		others_shares AS (
			SELECT e.user_id AS friend_id, t.rel_group, SUM(e.amount) AS total
			FROM expense e
			JOIN transactions t ON t.transaction_id = e.transaction_id
			WHERE t.creator_id = (SELECT id FROM my_id)
			  AND e.user_id != (SELECT id FROM my_id)
			GROUP BY e.user_id, t.rel_group
		),
		-- Combine friend and group pairs that have transactions
		involved_pairs AS (
			SELECT friend_id, rel_group FROM my_shares
			UNION
			SELECT friend_id, rel_group FROM others_shares
		)
		SELECT
			BIN_TO_UUID(u.uuid) AS friend_uuid,
			u.name AS friend_name,
			BIN_TO_UUID(ug.uuid) AS group_uuid,
			COALESCE(ms.total, 0) - COALESCE(os.total, 0) AS group_net_balance
		FROM involved_pairs ip
		JOIN users u ON u.id = ip.friend_id
		JOIN ugroups ug ON ug.id = ip.rel_group
		LEFT JOIN others_shares os ON os.friend_id = ip.friend_id AND os.rel_group = ip.rel_group
		LEFT JOIN my_shares ms ON ms.friend_id = ip.friend_id AND ms.rel_group = ip.rel_group
		ORDER BY u.name, group_net_balance DESC`

	result, err := database.DB.QueryContext(ctx, query, userID)
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Close()

	type groupBalance struct {
		GroupUUID string `json:"group_uuid"`
		Balance   int    `json:"balance"`
	}
	type friendBalance struct {
		FriendUUID string         `json:"friend_uuid"`
		FriendName string         `json:"friend_name"`
		NetBalance int            `json:"net_balance"`
		Groups     []groupBalance `json:"groups"`
	}

	balancesMap := make(map[string]*friendBalance)
	for result.Next() {
		var fUUID, fName, gUUID string
		var gNet int
		if err := result.Scan(&fUUID, &fName, &gUUID, &gNet); err != nil {
			log.Println("Error scanning balanced row:", err)
			continue
		}
		if _, ok := balancesMap[fUUID]; !ok {
			balancesMap[fUUID] = &friendBalance{
				FriendUUID: fUUID,
				FriendName: fName,
				Groups:     []groupBalance{},
			}
		}
		balancesMap[fUUID].NetBalance += gNet
		balancesMap[fUUID].Groups = append(balancesMap[fUUID].Groups, groupBalance{
			GroupUUID: gUUID,
			Balance:   gNet,
		})
	}

	var balances []friendBalance
	for _, b := range balancesMap {
		balances = append(balances, *b)
	}

	type responseStruct struct {
		Response []friendBalance `json:"response"`
		Message  string          `json:"message"`
	}
	f, err := json.Marshal(responseStruct{Response: balances, Message: "Balances fetched successfully!"})
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	responseWrapper(http.StatusOK, f, res)
}

func DeleteExpense(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	expenseUUID := chi.URLParam(req, "expenseid")

	// Get authenticated user ID from JWT context
	authUserID, ok := req.Context().Value(authentication.UserIDKey).(string)
	if !ok || authUserID == "" {
		jsonError(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Look up the transaction, verify creator and get group ID for broadcasting
	var transactionID int
	var creatorID int
	var relGroupID int
	lookupQuery := `SELECT transaction_id, creator_id, rel_group FROM transactions WHERE uuid = UUID_TO_BIN(?)`
	err := database.DB.QueryRowContext(ctx, lookupQuery, expenseUUID).Scan(&transactionID, &creatorID, &relGroupID)
	if err == sql.ErrNoRows {
		jsonError(res, "Expense not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(res, "Database error", http.StatusInternalServerError)
		return
	}

	// Verify the authenticated user is the creator
	authUserIDInt, err := strconv.Atoi(authUserID)
	if err != nil || authUserIDInt != creatorID {
		jsonError(res, "Only the creator can delete this expense", http.StatusForbidden)
		return
	}

	// Delete in a transaction (expense shares first, then the transaction)
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM expense WHERE transaction_id = ?", transactionID)
	if err != nil {
		jsonError(res, fmt.Sprintf("Failed to delete expense shares: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM transactions WHERE transaction_id = ?", transactionID)
	if err != nil {
		jsonError(res, fmt.Sprintf("Failed to delete transaction: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		jsonError(res, "Expense not found", http.StatusNotFound)
		return
	}

	if err = tx.Commit(); err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}

	responseWrapper(http.StatusOK, []byte(`{"message": "Expense deleted successfully"}`), res)

	// Broadcast update to all group members
	var groupMembers []string
	memberQuery := `SELECT BIN_TO_UUID(u.uuid) FROM group_membership gm JOIN users u ON gm.user_id = u.id WHERE gm.group_id = ?`
	rows, err := database.DB.QueryContext(ctx, memberQuery, relGroupID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mUUID string
			if err := rows.Scan(&mUUID); err == nil {
				groupMembers = append(groupMembers, mUUID)
			}
		}
	}
	if len(groupMembers) > 0 {
		broadcastUpdate(groupMembers)
	} else {
		// Resolve numeric authUserID to UUID for the fallback
		var fallbackUUID string
		if err := database.DB.QueryRowContext(ctx, "SELECT BIN_TO_UUID(uuid) FROM users WHERE id = ?", authUserID).Scan(&fallbackUUID); err == nil {
			broadcastUpdate([]string{fallbackUUID})
		}
	}
}

func GetUserActivity(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	userID := chi.URLParam(req, "userid")

	query := `
		SELECT
			BIN_TO_UUID(t.uuid) AS transaction_id,
			t.description,
			t.amount,
			t.created_at,
			BIN_TO_UUID(g.uuid) AS group_id,
			CASE
				WHEN g.name = (SELECT email FROM users WHERE uuid = UUID_TO_BIN(?)) THEN 'Non-Group Expenses'
				ELSE g.name
			END AS group_name,
			creator.name AS created_by
		FROM transactions t
		JOIN ugroups g ON g.id = t.rel_group
		JOIN users creator ON creator.id = t.creator_id
		WHERE t.rel_group IN (
			SELECT group_id FROM group_membership WHERE user_id = (
				SELECT id FROM users WHERE uuid = UUID_TO_BIN(?)
			)
		)
		ORDER BY t.created_at DESC
		LIMIT 50`

	result, err := database.DB.QueryContext(ctx, query, userID, userID)
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer result.Close()

	type activityItem struct {
		TransactionID string `json:"transaction_id"`
		Description   string `json:"description"`
		Amount        int    `json:"amount"`
		CreatedAt     string `json:"created_at"`
		GroupID       string `json:"group_id"`
		GroupName     string `json:"group_name"`
		CreatedBy     string `json:"created_by"`
	}
	var activities []activityItem
	for result.Next() {
		var a activityItem
		if err := result.Scan(&a.TransactionID, &a.Description, &a.Amount, &a.CreatedAt, &a.GroupID, &a.GroupName, &a.CreatedBy); err != nil {
			log.Println("Error scanning activity row:", err)
			continue
		}
		activities = append(activities, a)
	}

	type responseStruct struct {
		Response []activityItem `json:"response"`
		Message  string         `json:"message"`
	}
	f, err := json.Marshal(responseStruct{Response: activities, Message: "Activity fetched successfully!"})
	if err != nil {
		jsonError(res, err.Error(), http.StatusInternalServerError)
		return
	}
	responseWrapper(http.StatusOK, f, res)
}
