package main

import (
"encoding/json"
"fmt"
)

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

func main() {
	payload := []byte(`{"groupid":"54", "payer_id":"66", "total_amount":100, "description":"tst", "users":[{"userid":"87b", "amount":-50}]}`)
	var req CreateExpenseRequest
	json.Unmarshal(payload, &req)
	fmt.Printf("%+v\n", req)
}
