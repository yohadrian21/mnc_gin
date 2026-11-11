package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Massad/gin-boilerplate/forms"
	"github.com/Massad/gin-boilerplate/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	webhookTimestampHeader = "X-Webhook-Timestamp"
	webhookSignatureHeader = "X-Webhook-Signature"
	//webhookTolerance bounds how old a signed delivery may be, so a captured request cannot be replayed later.
	webhookTolerance     = 5 * time.Minute
	maxWebhookBodyBytes  = 64 << 10
	webhookStatusPaid    = "PAID"
	paymentProviderActor = "payment-provider"
)

//validWebhookSignature checks signature ("sha256=<hex>") against an HMAC-SHA256 of "<timestamp>.<body>".
func validWebhookSignature(secret []byte, timestamp, signature string, body []byte, now time.Time) bool {
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if age := now.Sub(time.Unix(unix, 0)); age > webhookTolerance || age < -webhookTolerance {
		return false
	}
	hexDigest, ok := strings.CutPrefix(signature, "sha256=")
	if !ok {
		return false
	}
	given, err := hex.DecodeString(hexDigest)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return hmac.Equal(given, mac.Sum(nil))
}

//TopUpWebhook credits a wallet once the payment provider reports a settled top-up payment.
func (ctrl TransactionController) TopUpWebhook(c *gin.Context) {
	secret := os.Getenv("PAYMENT_WEBHOOK_SECRET")
	if secret == "" {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "Top up webhook is not configured"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodyBytes))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"message": "Invalid request"})
		return
	}

	if !validWebhookSignature([]byte(secret), c.GetHeader(webhookTimestampHeader), c.GetHeader(webhookSignatureHeader), body, time.Now()) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid signature"})
		return
	}

	var event forms.TopUpWebhookForm
	if validationErr := binding.JSON.BindBody(body, &event); validationErr != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": transactionForm.Create(validationErr)})
		return
	}
	if event.Status != webhookStatusPaid {
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored"})
		return
	}

	limits, err := models.LoadTopUpLimits()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Top up could not be processed"})
		return
	}

	id, err := transactionModel.CreateTopUp(models.TopUp{
		UserID:    event.UserID,
		Amount:    event.Amount,
		Source:    models.TopUpSourceWebhook,
		Reference: event.Reference,
		Actor:     paymentProviderActor,
		Remarks:   "Top up via payment provider",
		ClientIP:  c.ClientIP(),
	}, limits)
	if errors.Is(err, models.ErrDuplicateTopUp) {
		//Providers redeliver until they get a 2xx; acknowledge without crediting twice.
		c.JSON(http.StatusOK, gin.H{"message": "Top up already processed"})
		return
	}
	if err != nil {
		status, message := topUpError(err)
		c.AbortWithStatusJSON(status, gin.H{"message": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Top up applied", "id": id})
}

//AdminTopUp lets an admin credit another user's wallet; the admin is recorded as the actor.
func (ctrl TransactionController) AdminTopUp(c *gin.Context) {
	adminID := getUserID(c)

	var form forms.AdminTopUpForm
	if validationErr := c.ShouldBindJSON(&form); validationErr != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": transactionForm.Create(validationErr)})
		return
	}

	limits, err := models.LoadTopUpLimits()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Top up could not be processed"})
		return
	}

	id, err := transactionModel.CreateTopUp(models.TopUp{
		UserID:    form.UserID,
		Amount:    form.Amount,
		Source:    models.TopUpSourceAdmin,
		Reference: form.Reference,
		Actor:     adminID,
		Remarks:   form.Remarks,
		ClientIP:  c.ClientIP(),
	}, limits)
	if err != nil {
		status, message := topUpError(err)
		c.AbortWithStatusJSON(status, gin.H{"message": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Top up applied", "id": id})
}

func topUpError(err error) (int, string) {
	switch {
	case errors.Is(err, models.ErrTopUpLimitExceeded):
		return http.StatusUnprocessableEntity, "Top up exceeds the per-user limit"
	case errors.Is(err, models.ErrUserNotFound):
		return http.StatusUnprocessableEntity, "User not found"
	case errors.Is(err, models.ErrDuplicateTopUp):
		return http.StatusConflict, "Top up reference was already processed"
	case errors.Is(err, models.ErrSelfTopUp):
		return http.StatusForbidden, "Admins cannot top up their own wallet"
	case errors.Is(err, models.ErrInvalidAmount):
		return http.StatusNotAcceptable, "Please enter an amount greater than zero"
	}
	log.Printf("top up failed: %v", err)
	return http.StatusInternalServerError, "Top up could not be processed"
}
