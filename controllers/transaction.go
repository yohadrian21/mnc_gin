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

//Create ...
func (ctrl TransactionController) Create(c *gin.Context) {
	userID := getUserID(c)

	var form forms.CreateTransactioForm

	// if validationErr := c.ShouldBindJSON(&form); validationErr != nil {
	// 	message := transactionForm.Create(validationErr)
	// 	c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
	// 	return
	// }

	id, err := transactionModel.Create(userID, form)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": "Transactions could not be created"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction created", "id": id})
}

//Create Top Up ...
func (ctrl TransactionController) CreateTopUpTransaction(c *gin.Context) {
	userID := getUserID(c)

	var form forms.CreateTransactioForm

	// if validationErr := c.ShouldBindJSON(&form); validationErr != nil {
	// 	message := transactionForm.Create(validationErr)
	// 	c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
	// 	return
	// }

	topUpID := uuid.New().String()
	form.TopUpID = topUpID
	id, err := transactionModel.CreateTopUp(userID, form)
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
