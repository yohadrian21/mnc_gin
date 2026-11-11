package controllers

import (
	"github.com/Massad/gin-boilerplate/forms"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/google/uuid"

	"net/http"

	"github.com/gin-gonic/gin"
)

//TransactionController ...
type TransactionController struct{}

var transactionModel = new(models.TransactionModel)
var transactionForm = new(forms.TransactionForm)

//CreatePaymentTransaction ...
func (ctrl TransactionController) CreatePaymentTransaction(c *gin.Context) {
	userID := getUserID(c)

	var form forms.CreateTransactioForm

	var formdto forms.CreateTransactionPaymentForm
	if validationErr := c.ShouldBindJSON(&formdto); validationErr != nil {
		message := transactionForm.Create(validationErr)
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
		return
	}

	PaymentID := uuid.New().String()
	form.PaymentID = PaymentID
	form.Amount = formdto.Amount
	form.Remarks = formdto.Remarks

	id, err := transactionModel.CreatePayment(userID, form)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": "Transactions could not be created"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction created", "id": id})
}

//CreateTransferTransaction ...
func (ctrl TransactionController) CreateTransferTransaction(c *gin.Context) {
	userID := getUserID(c)

	var form forms.CreateTransactioForm
	var formdto forms.CreateTransactionTransferForm
	if validationErr := c.ShouldBindJSON(&formdto); validationErr != nil {
		message := transactionForm.Create(validationErr)
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
		return
	}

	TransferID := uuid.New().String()
	form.TransferID = TransferID
	form.Amount = formdto.Amount
	form.Remarks = formdto.Remarks

	id, err := transactionModel.CreateTransfer(userID, formdto.TargetUser, form)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": "Transactions could not be created"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction created", "id": id})
}

//All ...
func (ctrl TransactionController) All(c *gin.Context) {
	userID := getUserID(c)

	results, err := transactionModel.All(userID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"Message": "Could not get articles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}
