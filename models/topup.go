package models

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/forms"

	"github.com/go-gorp/gorp"
	uuid "github.com/google/uuid"
	"github.com/lib/pq"
)

//Top-up sources. Users cannot credit their own wallet; a credit always comes from one of these.
const (
	TopUpSourceWebhook = "PAYMENT_WEBHOOK"
	TopUpSourceAdmin   = "ADMIN"
)

const (
	topUpApplied  = "APPLIED"
	topUpRejected = "REJECTED"
)

var (
	//ErrTopUpLimitExceeded is returned when a credit would exceed the per-user limits.
	ErrTopUpLimitExceeded = errors.New("top-up exceeds the per-user limit")
	//ErrDuplicateTopUp is returned when the source reference was already credited.
	ErrDuplicateTopUp = errors.New("top-up reference was already applied")
	//ErrSelfTopUp is returned when an admin tries to credit their own wallet.
	ErrSelfTopUp = errors.New("admins cannot top up their own wallet")
	//ErrUserNotFound ...
	ErrUserNotFound = errors.New("user not found")
)

//TopUpLimits caps how much a single user can be credited.
type TopUpLimits struct {
	MaxPerTransaction float64
	MaxPerDay         float64 //rolling 24 hours
}

//LoadTopUpLimits reads TOPUP_MAX_PER_TRANSACTION and TOPUP_MAX_PER_DAY, with defaults when unset.
func LoadTopUpLimits() (TopUpLimits, error) {
	perTransaction, err := positiveAmountEnv("TOPUP_MAX_PER_TRANSACTION", 10000000)
	if err != nil {
		return TopUpLimits{}, err
	}
	perDay, err := positiveAmountEnv("TOPUP_MAX_PER_DAY", 20000000)
	if err != nil {
		return TopUpLimits{}, err
	}
	return TopUpLimits{MaxPerTransaction: perTransaction, MaxPerDay: perDay}, nil
}

func positiveAmountEnv(name string, fallback float64) (float64, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !(value > 0) || math.IsInf(value, 1) {
		return 0, fmt.Errorf("%s must be a positive number, got %q", name, raw)
	}
	return value, nil
}

//TopUp is a credit request from a trusted source.
type TopUp struct {
	UserID    string
	Amount    float64
	Source    string //TopUpSourceWebhook or TopUpSourceAdmin
	Reference string //provider payment reference or admin ticket; credited at most once per source
	Actor     string //admin user ID, or the payment provider
	Remarks   string
	ClientIP  string
}

const insertTopUpAuditSQL = "INSERT INTO public.topup_audit(id, user_id, amount, source, reference, actor, outcome, reason, client_ip, transaction_id) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"

//CreateTopUp credits a user's wallet within the per-user limits. Every attempt is written to
//topup_audit: an APPLIED row commits together with the balance change, a REJECTED row is written
//after the transaction rolls back.
func (m TransactionModel) CreateTopUp(topUp TopUp, limits TopUpLimits) (transactionID string, err error) {
	transactionID, err = applyTopUp(topUp, limits)
	if err != nil {
		recordRejectedTopUp(topUp, err)
		return "", err
	}
	return transactionID, nil
}

func applyTopUp(topUp TopUp, limits TopUpLimits) (string, error) {
	if topUp.Amount <= 0 {
		return "", ErrInvalidAmount
	}
	if topUp.Source == TopUpSourceAdmin && topUp.Actor == topUp.UserID {
		return "", ErrSelfTopUp
	}
	if topUp.Amount > limits.MaxPerTransaction {
		return "", ErrTopUpLimitExceeded
	}

	auditID := uuid.New().String()
	transactionID := uuid.New().String()
	err := withTx(func(tx *gorp.Transaction) error {
		balance, err := lockBalance(tx, topUp.UserID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}

		applied, err := tx.SelectInt("SELECT count(id) FROM public.topup_audit WHERE source=$1 AND reference=$2 AND outcome=$3", topUp.Source, topUp.Reference, topUpApplied)
		if err != nil {
			return err
		}
		if applied > 0 {
			return ErrDuplicateTopUp
		}

		//The user row is locked above, so no other credit for this user can land between this sum and the insert.
		creditedToday, err := tx.SelectFloat("SELECT COALESCE(SUM(amount), 0) FROM public.topup_audit WHERE user_id=$1 AND outcome=$2 AND created_at > NOW() - INTERVAL '24 hours'", topUp.UserID, topUpApplied)
		if err != nil {
			return err
		}
		if creditedToday+topUp.Amount > limits.MaxPerDay {
			return ErrTopUpLimitExceeded
		}

		form := forms.CreateTransactioForm{
			TopUpID:       auditID,
			Amount:        topUp.Amount,
			Remarks:       topUp.Remarks,
			BalanceBefore: balance,
			BalanceAfter:  balance + topUp.Amount,
			CreatedAt:     time.Now(),
			Status:        "SUCCESS",
			Type:          "CREDIT",
		}
		if err := applyBalanceChange(tx, transactionID, topUp.UserID, form); err != nil {
			return err
		}
		_, err = tx.Exec(insertTopUpAuditSQL, auditID, topUp.UserID, topUp.Amount, topUp.Source, topUp.Reference, topUp.Actor, topUpApplied, nil, topUp.ClientIP, transactionID)
		if isUniqueViolation(err) {
			return ErrDuplicateTopUp
		}
		return err
	})
	if err != nil {
		return "", err
	}
	return transactionID, nil
}

func recordRejectedTopUp(topUp TopUp, reason error) {
	_, err := db.GetDB().Exec(insertTopUpAuditSQL, uuid.New().String(), topUp.UserID, topUp.Amount, topUp.Source, topUp.Reference, topUp.Actor, topUpRejected, reason.Error(), topUp.ClientIP, nil)
	if err != nil {
		log.Printf("topup audit: could not record rejected %s top-up %q for user %s: %v", topUp.Source, topUp.Reference, topUp.UserID, err)
	}
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
