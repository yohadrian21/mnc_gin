package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/forms"

	"github.com/go-gorp/gorp"
	uuid "github.com/google/uuid"
)

// ErrInsufficientBalance is returned when a debit would make the balance negative.
var ErrInsufficientBalance = errors.New("insufficient balance")

// ErrInvalidAmount is returned when the amount is zero or negative.
var ErrInvalidAmount = errors.New("amount must be greater than zero")

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

const insertTransactionSQL = "INSERT INTO public.transactions(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id"

// lockBalance reads a user's balance inside tx and locks the row (SELECT ... FOR UPDATE)
// so concurrent requests on the same wallet are serialized until tx commits or rolls back.
func lockBalance(tx *gorp.Transaction, userID string) (float64, error) {
	var balance float64
	if err := tx.SelectOne(&balance, "SELECT balance FROM public.user WHERE id=$1 FOR UPDATE", userID); err != nil {
		return 0, fmt.Errorf("fetching balance for user %s: %w", userID, err)
	}
	return balance, nil
}

// applyBalanceChange writes the ledger row and the new balance for one user inside tx.
func applyBalanceChange(tx *gorp.Transaction, transactionID, userID string, form forms.CreateTransactioForm) error {
	if err := tx.QueryRow(insertTransactionSQL, transactionID, userID, form.Type, form.Amount, form.BalanceBefore, form.BalanceAfter, form.Remarks, form.Status, form.CreatedAt, form.TopUpID, form.PaymentID, form.TransferID).Scan(&transactionID); err != nil {
		return err
	}
	_, err := tx.Exec("UPDATE public.user SET balance = $1, updated_at = $2 WHERE id = $3", form.BalanceAfter, time.Now().Unix(), userID)
	return err
}

// withTx runs fn in a DB transaction, committing on success and rolling back on error.
func withTx(fn func(tx *gorp.Transaction) error) error {
	tx, err := db.GetDB().Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

//CreatePayment debits the user's balance atomically; fails if the balance is insufficient.
func (m TransactionModel) CreatePayment(userID string, form forms.CreateTransactioForm) (transactionID string, err error) {
	if form.Amount <= 0 {
		return "", ErrInvalidAmount
	}
	transactionID = uuid.New().String()
	err = withTx(func(tx *gorp.Transaction) error {
		balance, err := lockBalance(tx, userID)
		if err != nil {
			return err
		}
		if balance < form.Amount {
			return ErrInsufficientBalance
		}
		form.BalanceBefore = balance
		form.BalanceAfter = balance - form.Amount
		form.CreatedAt = time.Now()
		form.Status = "SUCCESS"
		form.Type = "DEBIT"
		return applyBalanceChange(tx, transactionID, userID, form)
	})
	if err != nil {
		return "", err
	}
	return transactionID, nil
}

//CreateTransfer moves funds between two users in one DB transaction.
//Both rows are locked in a fixed order (by id) so opposite transfers can't deadlock.
func (m TransactionModel) CreateTransfer(userID string, targetUserID string, form forms.CreateTransactioForm) (transactionID string, err error) {
	if form.Amount <= 0 {
		return "", ErrInvalidAmount
	}
	if userID == targetUserID {
		return "", errors.New("cannot transfer to the same user")
	}
	debitID := uuid.New().String()
	creditID := uuid.New().String()
	err = withTx(func(tx *gorp.Transaction) error {
		balances := map[string]float64{}
		first, second := userID, targetUserID
		if second < first {
			first, second = second, first
		}
		for _, id := range []string{first, second} {
			b, err := lockBalance(tx, id)
			if err != nil {
				return err
			}
			balances[id] = b
		}
		if balances[userID] < form.Amount {
			return ErrInsufficientBalance
		}
		now := time.Now()

		debit := form
		debit.BalanceBefore = balances[userID]
		debit.BalanceAfter = balances[userID] - form.Amount
		debit.CreatedAt = now
		debit.Status = "SUCCESS"
		debit.Type = "DEBIT"
		if err := applyBalanceChange(tx, debitID, userID, debit); err != nil {
			return err
		}

		credit := forms.CreateTransactioForm{
			Amount:        form.Amount,
			TransferID:    form.TransferID,
			Remarks:       form.Remarks,
			BalanceBefore: balances[targetUserID],
			BalanceAfter:  balances[targetUserID] + form.Amount,
			CreatedAt:     now,
			Status:        "SUCCESS",
			Type:          "CREDIT",
		}
		return applyBalanceChange(tx, creditID, targetUserID, credit)
	})
	if err != nil {
		return "", err
	}
	return debitID, nil
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
