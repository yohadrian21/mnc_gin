package forms

import (
	"encoding/json"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

//TransactionForm ...
type TransactionForm struct{}

//CreateTransactioForm ...
type CreateTransactioForm struct {
	UserID        uuid.UUID `form:"user_id" json:"uuser_id" binding:"required"`
	Type          string    `form:"type" json:"type" binding:"required"` // CREDIT/DEBIT
	Amount        float64   `form:"amount" json:"amount" binding:"required"`
	BalanceBefore float64   `form:"balance_before" json:"balance_before" binding:"required"`
	BalanceAfter  float64   `form:"balance_after" json:"balance_after" binding:"required"`
	Remarks       string    `form:"remarks" json:"remarks" binding:"required"`
	Status        string    `form:"status" json:"status" binding:"required"`
	CreatedAt     time.Time `form:"created_at" json:"created_at" binding:"required"`
	PaymentID     string    `form:"payment_id" json:"payment_id"`
	TopUpID       string    `form:"top_up_id" json:"top_up_id" `
	TransferID    string    `form:"transfer_id" json:"transfer_id"`
}

//CreateTransactioForm ...
type CreateTransactionTopUpForm struct {
	Amount float64 `form:"amount" json:"amount" binding:"required"`
}

//CreateTransactioForm ...
type CreateTransactionPaymentForm struct {
	Amount  float64 `form:"amount" json:"amount" binding:"required"`
	Remarks string  `form:"remarks" json:"remarks" binding:"required"`
}

//CreateTransactioForm ...
type CreateTransactionTransferForm struct {
	TargetUser string  `form:"target_user" json:"target_user" binding:"required"`
	Amount     float64 `form:"amount" json:"amount" binding:"required"`
	Remarks    string  `form:"remarks" json:"remarks" binding:"required"`
}

//Create ...
func (f CreateTransactioForm) Create(err error) string {
	switch err.(type) {
	case validator.ValidationErrors:

		if _, ok := err.(*json.UnmarshalTypeError); ok {
			return "Something went wrong, please try again later"
		}

	default:
		return "Invalid request"
	}

	return "Something went wrong, please try again later"
}
