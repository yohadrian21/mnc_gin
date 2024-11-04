package models

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/forms"

	uuid "github.com/google/uuid"
)

//Transaction ...

type Transaction struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID        uuid.UUID
	Type          string // CREDIT/DEBIT
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
	Remarks       string
	Status        string
	CreatedAt     time.Time
}

//TransactionModel ...
type TransactionModel struct{}

//Create ...
func (m TransactionModel) Create(userID string, form forms.CreateTransactioForm) (transactionID string, err error) {
	// Generate a UUID for the user ID
	transactionID = uuid.New().String()
	err = db.GetDB().QueryRow("INSERT INTO public.transaction(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id", transactionID, userID, form.Type, form.Amount, form.BalanceBefore, form.BalanceAfter, form.Remarks, form.Status, form.CreatedAt, form.PaymentID, form.TopUpID, form.TransferID).Scan(&transactionID)
	return transactionID, err
}

//Create ...
func (m TransactionModel) CreateTopUp(userID string, form forms.CreateTransactioForm) (transactionID string, err error) {

	err = db.GetDB().SelectOne(&form.BalanceBefore, "SELECT balance FROM public.user WHERE id=$1 LIMIT 1", userID)
	if err != nil {
		log.Fatalf("Error fetching user balance: %v", err)
	}
	transactionID = uuid.New().String()

	form.BalanceAfter = form.BalanceBefore + form.Amount
	form.CreatedAt = time.Now()
	form.Status = "SUCCESS"
	form.Type = "CREDIT"
	err = db.GetDB().QueryRow("INSERT INTO public.transactions(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id", transactionID, userID, form.Type, form.Amount, form.BalanceBefore, form.BalanceAfter, form.Remarks, form.Status, form.CreatedAt, form.TopUpID, form.PaymentID, form.TransferID).Scan(&transactionID)
	_, err = db.GetDB().Exec("UPDATE public.user SET balance = $1, updated_at = $2 WHERE id = $3", form.BalanceAfter, time.Now().Unix(), userID)
	if err != nil {
		// Handle the error appropriately
		return "", err
	}
	return transactionID, err
}

//Create ...
func (m TransactionModel) CreatePayment(userID string, form forms.CreateTransactioForm) (transactionID string, err error) {

	err = db.GetDB().SelectOne(&form.BalanceBefore, "SELECT balance FROM public.user WHERE id=$1 LIMIT 1", userID)
	if err != nil {
		log.Fatalf("Error fetching user balance: %v", err)
	}
	transactionID = uuid.New().String()

	form.BalanceAfter = form.BalanceBefore - form.Amount
	form.CreatedAt = time.Now()
	form.Status = "SUCCESS"
	form.Type = "DEBIT"
	err = db.GetDB().QueryRow("INSERT INTO public.transactions(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id", transactionID, userID, form.Type, form.Amount, form.BalanceBefore, form.BalanceAfter, form.Remarks, form.Status, form.CreatedAt, form.TopUpID, form.PaymentID, form.TransferID).Scan(&transactionID)
	_, err = db.GetDB().Exec("UPDATE public.user SET balance = $1, updated_at = $2 WHERE id = $3", form.BalanceAfter, time.Now().Unix(), userID)
	if err != nil {
		// Handle the error appropriately
		return "", err
	}
	return transactionID, err
}

//Create ...
func (m TransactionModel) CreateTransfer(userID string, targetUserID string, form forms.CreateTransactioForm) (transactionID string, err error) {

	//userID
	err = db.GetDB().SelectOne(&form.BalanceBefore, "SELECT balance FROM public.user WHERE id=$1 LIMIT 1", userID)
	if err != nil {
		log.Fatalf("Error fetching user balance: %v", err)
	}
	transactionID = uuid.New().String()

	form.BalanceAfter = form.BalanceBefore - form.Amount
	form.CreatedAt = time.Now()
	form.Status = "SUCCESS"
	form.Type = "DEBIT"
	err = db.GetDB().QueryRow("INSERT INTO public.transactions(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id", transactionID, userID, form.Type, form.Amount, form.BalanceBefore, form.BalanceAfter, form.Remarks, form.Status, form.CreatedAt, form.PaymentID, form.PaymentID, form.TransferID).Scan(&transactionID)
	_, err = db.GetDB().Exec("UPDATE public.user SET balance = $1, updated_at = $2 WHERE id = $3", form.BalanceAfter, time.Now().Unix(), userID)
	if err != nil {
		// Handle the error appropriately
		return "", err
	}
	//target User Id
	formTarget := new(forms.CreateTransactioForm)
	err = db.GetDB().SelectOne(&formTarget.BalanceBefore, "SELECT balance FROM public.user WHERE id=$1 LIMIT 1", targetUserID)
	if err != nil {
		log.Fatalf("Error fetching user balance: %v", err)
	}
	transactionID = uuid.New().String()

	formTarget.BalanceAfter = formTarget.BalanceBefore + form.Amount
	formTarget.CreatedAt = time.Now()
	formTarget.Status = "SUCCESS"
	formTarget.Type = "CREDIT"
	err = db.GetDB().QueryRow("INSERT INTO public.transactions(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id", transactionID, targetUserID, formTarget.Type, formTarget.Amount, formTarget.BalanceBefore, formTarget.BalanceAfter, formTarget.Remarks, formTarget.Status, formTarget.CreatedAt, formTarget.TopUpID, formTarget.PaymentID, formTarget.TransferID).Scan(&transactionID)
	_, err = db.GetDB().Exec("UPDATE public.user SET balance = $1, updated_at = $2 WHERE id = $3", formTarget.BalanceAfter, time.Now().Unix(), targetUserID)
	if err != nil {
		// Handle the error appropriately
		return "", err
	}
	return transactionID, err
}

// //One ...
// func (m TransactionModel) One(userID, id int64) (article Article, err error) {
// 	err = db.GetDB().SelectOne(&article, "SELECT a.id, a.title, a.content, a.updated_at, a.created_at, json_build_object('id', u.id, 'name', u.name, 'email', u.email) AS user FROM public.article a LEFT JOIN public.user u ON a.user_id = u.id WHERE a.user_id=$1 AND a.id=$2 LIMIT 1", userID, id)
// 	return article, err
// }
// DataListTransaction holds the overall response structure.
type DataListTransaction struct {
	Status string            `json:"status"`
	Result []tempTransaction `json:"result"`
}

// Transaction represents a transaction structure.
type tempTransaction struct {
	TransferID      *string `json:"transfer_id,omitempty"`
	PaymentID       *string `json:"payment_id,omitempty"`
	TopUpID         *string `json:"top_up_id,omitempty"`
	Status          string  `json:"status"`
	UserID          string  `json:"user_id"`
	TransactionType string  `json:"transaction_type"`
	Amount          float64 `json:"amount"`
	Remarks         *string `json:"remarks,omitempty"`
	BalanceBefore   float64 `json:"balance_before"`
	BalanceAfter    float64 `json:"balance_after"`
	CreatedDate     string  `json:"created_date"`
}

// All retrieves all transactions for a specific userID and formats the result as JSON.
func (m TransactionModel) All(userID string) (DataListTransaction, error) {
	var dataList DataListTransaction

	query := `
        SELECT
            CASE 
                WHEN top_up_id IS NOT NULL THEN json_build_object(
                    'top_up_id', top_up_id,
                    'status', status,
                    'user_id', user_id,
                    'transaction_type', "type",
                    'amount', amount,
                    'remarks', remarks,
                    'balance_before', balance_before,
                    'balance_after', balance_after,
                    'created_date', to_char(created_at, 'YYYY-MM-DD HH24:MI:SS')
                )
                WHEN payment_id IS NOT NULL THEN json_build_object(
                    'payment_id', payment_id,
                    'status', status,
                    'user_id', user_id,
                    'transaction_type', "type",
                    'amount', amount,
                    'remarks', remarks,
                    'balance_before', balance_before,
                    'balance_after', balance_after,
                    'created_date', to_char(created_at, 'YYYY-MM-DD HH24:MI:SS')
                )
                ELSE json_build_object(
                    'transfer_id', id,
                    'status', status,
                    'user_id', user_id,
                    'transaction_type', "type",
                    'amount', amount,
                    'remarks', remarks,
                    'balance_before', balance_before,
                    'balance_after', balance_after,
                    'created_date', to_char(created_at, 'YYYY-MM-DD HH24:MI:SS')
                )
            END AS result
        FROM 
            public.transactions
        WHERE 
            user_id = $1;`

	// Declare a slice to hold the raw transaction results
	var results []string
	var transactions []tempTransaction

	// Use Select to populate the results slice
	_, err := db.GetDB().Select(&results, query, userID)
	if err != nil {
		return dataList, err
	}
	// Loop through the array of JSON strings
	for _, jsonString := range results {
		var transaction tempTransaction

		// Unmarshal the JSON string into the transaction struct
		err := json.Unmarshal([]byte(jsonString), &transaction)
		if err != nil {
			fmt.Printf("Error unmarshaling JSON: %v\n", err)
			continue // Skip this entry on error
		}

		// Append the populated transaction to the slice
		transactions = append(transactions, transaction)
	}
	// Populate the dataList struct
	dataList.Status = "SUCCESS"
	dataList.Result = transactions

	return dataList, nil
}
