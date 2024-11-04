package models

import (
	"log"
	"time"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/forms"

	"github.com/Massad/gin-boilerplate/models"
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
	var userModel models.UserModel
	user, err := userModel.GetUserBalance(userID)
	if err != nil {
		log.Fatalf("Error fetching user balance: %v", err)
	}
	transactionID = uuid.New().String()
	//get from user balance
	form.BalanceBefore = user.balance
	form.BalanceAfter = form.BalanceBefore + form.Amount
	form.CreatedAt = time.Now()
	form.Status = "SUCCESS"
	form.Type = "DEBIT"
	err = db.GetDB().QueryRow("INSERT INTO public.transaction(id,user_id, type,amount,balance_before,balance_after,remarks,status,created_at,top_up_id,payment_id,transfer_id) VALUES($1, $2, $3,$4, $5, $6,$7, $8,$9,$10,$11,$12) RETURNING id", transactionID, userID, form.Type, form.Amount, form.BalanceBefore, form.BalanceAfter, form.Remarks, form.Status, form.CreatedAt, form.PaymentID, form.TopUpID, form.TransferID).Scan(&transactionID)
	return transactionID, err
}

// //One ...
// func (m TransactionModel) One(userID, id int64) (article Article, err error) {
// 	err = db.GetDB().SelectOne(&article, "SELECT a.id, a.title, a.content, a.updated_at, a.created_at, json_build_object('id', u.id, 'name', u.name, 'email', u.email) AS user FROM public.article a LEFT JOIN public.user u ON a.user_id = u.id WHERE a.user_id=$1 AND a.id=$2 LIMIT 1", userID, id)
// 	return article, err
// }

//All ...
func (m TransactionModel) All(userID string) (articles []DataList, err error) {
	_, err = db.GetDB().Select(&articles, "SELECT COALESCE(array_to_json(array_agg(row_to_json(d))), '[]') AS data, (SELECT row_to_json(n) FROM ( SELECT count(a.id) AS total FROM public.article AS a WHERE a.user_id=$1 LIMIT 1 ) n ) AS meta FROM ( SELECT a.id, a.title, a.content, a.updated_at, a.created_at, json_build_object('id', u.id, 'name', u.name, 'email', u.email) AS user FROM public.article a LEFT JOIN public.user u ON a.user_id = u.id WHERE a.user_id=$1 ORDER BY a.id DESC) d", userID)
	return articles, err
}
