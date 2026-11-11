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

//AdminTopUpForm ...
type AdminTopUpForm struct {
	UserID    string  `json:"user_id" binding:"required"`
	Amount    float64 `json:"amount" binding:"required,gt=0"`
	Reference string  `json:"reference" binding:"required,max=128"`
	Remarks   string  `json:"remarks" binding:"required,max=255"`
}

//TopUpWebhookForm is the event the payment provider posts about a top-up payment.
type TopUpWebhookForm struct {
	Reference string  `json:"reference" binding:"required,max=128"`
	UserID    string  `json:"user_id" binding:"required"`
	Amount    float64 `json:"amount" binding:"required,gt=0"`
	Status    string  `json:"status" binding:"required"`
}

//CreateTransactionPaymentForm ...
type CreateTransactionPaymentForm struct {
	Amount  float64 `form:"amount" json:"amount" binding:"required,gt=0"`
	Remarks string  `form:"remarks" json:"remarks" binding:"required"`
}

//CreateTransactionTransferForm ...
type CreateTransactionTransferForm struct {
	TargetUser string  `form:"target_user" json:"target_user" binding:"required"`
	Amount     float64 `form:"amount" json:"amount" binding:"required,gt=0"`
	Remarks    string  `form:"remarks" json:"remarks" binding:"required"`
}

//Create maps a binding error on a money request to a client-facing message.
func (f TransactionForm) Create(err error) string {
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, fieldErr := range validationErrs {
			switch fieldErr.Field() {
			case "Amount":
				return "Please enter an amount greater than zero"
			case "TargetUser", "UserID":
				return "Please enter the target user"
			case "Remarks":
				return "Please enter the remarks"
			case "Reference":
				return "Please enter a reference of up to 128 characters"
			case "Status":
				return "Please enter the payment status"
			}
		}
	}
	return "Invalid request"
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
