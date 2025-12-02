package models

type User struct {
	Id       string `json:"id"`
	Uuid     string `json:"uuid"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type CreateGroupRequest struct {
	Name   string `json:"name"`
	UserId string `json:"userid"`
}

type ShareHolder struct {
	UserId string `json:"userid"`
	Amount int    `json:"amount"`
}

type CreateExpenseRequest struct {
	GroupId      string        `json:"groupid"`
	PayerId      string        `json:"payer_id"`
	TotalAmount  int           `json:"total_amount"`
	Description  string        `json:"description"`
	Shareholders []ShareHolder `json:"users"`
}

// Struct for handling UserExpense Request

// Request Part
type UserExpensesRequest struct {
	UserId string `json:"userid"`
}

// Response Part

type UserExpense struct {
	UserId        string                               `json:"userid"`
	GroupDetails  []UserAssociatedGroupAndTheirDetails `json:"groups"`
	FriendDetails []FriendSplitDetails                 `json:"friends"`
}

type UserAssociatedGroupAndTheirDetails struct {
	GroupId string               `json:"groupid"`
	Splits  []FriendSplitDetails `json:"splits"`
}

type GroupSplits struct {
	FriendSplitDetails
	TransactionID string `json:"txn_id"`
}

type FriendSplitDetails struct {
	UserId       string `json:"userid"`
	Amount       int    `json:"amount"`
	Name         string `json:"name"`
	EmailAddress string `json:"email"`
}
