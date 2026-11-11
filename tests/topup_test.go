package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adminUserID = "a5d2c7e1-3b4f-4c8a-9e6d-1f2b3c4d5e6f"

func signWebhook(secret, body string, at time.Time) http.Header {
	timestamp := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + body))
	return http.Header{
		"X-Webhook-Timestamp": {timestamp},
		"X-Webhook-Signature": {"sha256=" + hex.EncodeToString(mac.Sum(nil))},
	}
}

func webhookBody(reference, userID string, amount float64, status string) string {
	return fmt.Sprintf(`{"reference":%q,"user_id":%q,"amount":%v,"status":%q}`, reference, userID, amount, status)
}

func adminTopUpBody(userID string, amount float64, reference string) string {
	return fmt.Sprintf(`{"user_id":%q,"amount":%v,"reference":%q,"remarks":"support ticket"}`, userID, amount, reference)
}

// captured records a string SQL argument so tests can compare IDs across statements.
type captured struct{ into *string }

func (c captured) Match(v driver.Value) bool {
	s, ok := v.(string)
	*c.into = s
	return ok
}

func (e *testEnv) expectRole(userID, role string) {
	e.sql.ExpectQuery(regexp.QuoteMeta("SELECT role FROM public.user WHERE id=$1")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(role))
}

func (e *testEnv) expectBalanceLock(userID string, balance float64) {
	e.sql.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM public.user WHERE id=$1 FOR UPDATE")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
}

func (e *testEnv) expectAppliedCount(source, reference string, count int) {
	e.sql.ExpectQuery(regexp.QuoteMeta("SELECT count(id) FROM public.topup_audit WHERE source=$1 AND reference=$2 AND outcome=$3")).
		WithArgs(source, reference, "APPLIED").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func (e *testEnv) expectCreditedToday(userID string, total float64) {
	e.sql.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(amount), 0) FROM public.topup_audit WHERE user_id=$1 AND outcome=$2")).
		WithArgs(userID, "APPLIED").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(total))
}

type topUpRows struct{ ledgerID, ledgerTopUpID, auditID, auditTransactionID string }

// expectAppliedTopUp expects one credit committed together with its APPLIED audit row.
func (e *testEnv) expectAppliedTopUp(userID string, balance, amount float64, source, reference, actor string) *topUpRows {
	rows := &topUpRows{}
	e.sql.ExpectBegin()
	e.expectBalanceLock(userID, balance)
	e.expectAppliedCount(source, reference, 0)
	e.expectCreditedToday(userID, 0)
	e.sql.ExpectQuery(regexp.QuoteMeta("INSERT INTO public.transactions")).
		WithArgs(captured{&rows.ledgerID}, userID, "CREDIT", amount, balance, balance+amount, sqlmock.AnyArg(), "SUCCESS", sqlmock.AnyArg(), captured{&rows.ledgerTopUpID}, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ledger-row"))
	e.sql.ExpectExec(regexp.QuoteMeta("UPDATE public.user SET balance = $1")).
		WithArgs(balance+amount, sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	e.sql.ExpectExec(regexp.QuoteMeta("INSERT INTO public.topup_audit")).
		WithArgs(captured{&rows.auditID}, userID, amount, source, reference, actor, "APPLIED", nil, testClientIP, captured{&rows.auditTransactionID}).
		WillReturnResult(sqlmock.NewResult(0, 1))
	e.sql.ExpectCommit()
	return rows
}

func (e *testEnv) expectRejectedTopUp(userID string, amount float64, source, reference, actor string, reason error) {
	e.sql.ExpectExec(regexp.QuoteMeta("INSERT INTO public.topup_audit")).
		WithArgs(sqlmock.AnyArg(), userID, amount, source, reference, actor, "REJECTED", reason.Error(), testClientIP, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestSelfServiceTopUpIsRejected(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	// The unrestricted self-service route is gone, with or without a token.
	assert.Equal(t, http.StatusNotFound, env.request(http.MethodPost, "/v1/topup", `{"amount":999999999}`, nil).Code)
	assert.Equal(t, http.StatusNotFound, env.request(http.MethodPost, "/v1/topup", `{"amount":999999999}`, bearer(session.AccessToken)).Code)

	selfCredit := adminTopUpBody(testUserID, 500000, "self-credit")
	assert.Equal(t, http.StatusUnauthorized, env.request(http.MethodPost, "/v1/admin/topup", selfCredit, nil).Code)

	env.expectRole(testUserID, "USER")
	rec := env.request(http.MethodPost, "/v1/admin/topup", selfCredit, bearer(session.AccessToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "a regular user's token must not unlock the admin route")

	rec = env.request(http.MethodPost, "/v1/webhooks/topup", webhookBody("pay_1", testUserID, 500000, "PAID"), bearer(session.AccessToken))
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a user token is not a payment-provider signature")
}

func TestTopUpWebhookRejectsUnsignedOrForgedDeliveries(t *testing.T) {
	env := newTestEnv(t)
	body := webhookBody("pay_123", testUserID, 50000, "PAID")
	now := time.Now()

	unprefixed := signWebhook(testWebhookSecret, body, now)
	unprefixed.Set("X-Webhook-Signature", unprefixed.Get("X-Webhook-Signature")[len("sha256="):])

	// No SQL expectations: nothing may be read or credited for these.
	for name, header := range map[string]http.Header{
		"no signature headers":   nil,
		"signed with other key":  signWebhook("some-other-key-that-is-long-enough-to-pass", body, now),
		"signed for other body":  signWebhook(testWebhookSecret, webhookBody("pay_123", testUserID, 5, "PAID"), now),
		"replayed after 10 min":  signWebhook(testWebhookSecret, body, now.Add(-10*time.Minute)),
		"timestamp in future":    signWebhook(testWebhookSecret, body, now.Add(10*time.Minute)),
		"missing sha256= prefix": unprefixed,
		"signature not hex": {
			"X-Webhook-Timestamp": {strconv.FormatInt(now.Unix(), 10)},
			"X-Webhook-Signature": {"sha256=not-hex"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := env.request(http.MethodPost, "/v1/webhooks/topup", body, header)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		})
	}
}

func TestTopUpWebhookIsDisabledWithoutSecret(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("PAYMENT_WEBHOOK_SECRET", "")

	body := webhookBody("pay_123", testUserID, 50000, "PAID")
	rec := env.request(http.MethodPost, "/v1/webhooks/topup", body, signWebhook("", body, time.Now()))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestTopUpWebhookCreditsOnceWithAuditTrail(t *testing.T) {
	env := newTestEnv(t)
	body := webhookBody("pay_123", testUserID, 50000, "PAID")

	rows := env.expectAppliedTopUp(testUserID, 1000, 50000, models.TopUpSourceWebhook, "pay_123", "payment-provider")
	rec := env.request(http.MethodPost, "/v1/webhooks/topup", body, signWebhook(testWebhookSecret, body, time.Now()))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, rows.auditID, rows.ledgerTopUpID, "the ledger row points at its audit entry")
	assert.Equal(t, rows.ledgerID, rows.auditTransactionID, "the audit entry points at its ledger row")

	// The provider redelivers the same payment: acknowledged, recorded, not credited again.
	env.sql.ExpectBegin()
	env.expectBalanceLock(testUserID, 51000)
	env.expectAppliedCount(models.TopUpSourceWebhook, "pay_123", 1)
	env.sql.ExpectRollback()
	env.expectRejectedTopUp(testUserID, 50000, models.TopUpSourceWebhook, "pay_123", "payment-provider", models.ErrDuplicateTopUp)

	rec = env.request(http.MethodPost, "/v1/webhooks/topup", body, signWebhook(testWebhookSecret, body, time.Now()))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"message":"Top up already processed"}`, rec.Body.String())
}

func TestTopUpWebhookIgnoresUnpaidAndRejectsInvalidEvents(t *testing.T) {
	env := newTestEnv(t)

	body := webhookBody("pay_123", testUserID, 50000, "FAILED")
	rec := env.request(http.MethodPost, "/v1/webhooks/topup", body, signWebhook(testWebhookSecret, body, time.Now()))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"message":"Event ignored"}`, rec.Body.String())

	for _, body := range []string{
		`{"reference":"pay_123","user_id":"` + testUserID + `","status":"PAID"}`,
		webhookBody("pay_123", testUserID, -50000, "PAID"),
		webhookBody("", testUserID, 50000, "PAID"),
		`{"reference":`,
	} {
		rec := env.request(http.MethodPost, "/v1/webhooks/topup", body, signWebhook(testWebhookSecret, body, time.Now()))
		assert.Equal(t, http.StatusNotAcceptable, rec.Code, body)
	}
}

func TestTopUpEnforcesPerUserLimits(t *testing.T) {
	t.Run("single credit above the per-transaction limit", func(t *testing.T) {
		env := newTestEnv(t)
		amount := testMaxTopUp + 1
		body := webhookBody("pay_big", testUserID, amount, "PAID")

		env.expectRejectedTopUp(testUserID, amount, models.TopUpSourceWebhook, "pay_big", "payment-provider", models.ErrTopUpLimitExceeded)
		rec := env.request(http.MethodPost, "/v1/webhooks/topup", body, signWebhook(testWebhookSecret, body, time.Now()))
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("credit that would pass the rolling 24h limit", func(t *testing.T) {
		env := newTestEnv(t)
		admin := env.signIn(t, adminUserID)
		alreadyCredited := testMaxTopUp
		amount := testMaxTopUpPerDay - alreadyCredited + 1

		env.expectRole(adminUserID, "ADMIN")
		env.sql.ExpectBegin()
		env.expectBalanceLock(testUserID, 0)
		env.expectAppliedCount(models.TopUpSourceAdmin, "TICKET-7", 0)
		env.expectCreditedToday(testUserID, alreadyCredited)
		env.sql.ExpectRollback()
		env.expectRejectedTopUp(testUserID, amount, models.TopUpSourceAdmin, "TICKET-7", adminUserID, models.ErrTopUpLimitExceeded)

		rec := env.request(http.MethodPost, "/v1/admin/topup", adminTopUpBody(testUserID, amount, "TICKET-7"), bearer(admin.AccessToken))
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	})
}

func TestAdminTopUpRecordsAdminAsActor(t *testing.T) {
	env := newTestEnv(t)
	admin := env.signIn(t, adminUserID)

	env.expectRole(adminUserID, "ADMIN")
	rows := env.expectAppliedTopUp(testUserID, 0, 250000, models.TopUpSourceAdmin, "TICKET-42", adminUserID)

	rec := env.request(http.MethodPost, "/v1/admin/topup", adminTopUpBody(testUserID, 250000, "TICKET-42"), bearer(admin.AccessToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, rows.auditID, rows.ledgerTopUpID)
	assert.Equal(t, rows.ledgerID, rows.auditTransactionID)
}

func TestAdminCannotTopUpOwnWallet(t *testing.T) {
	env := newTestEnv(t)
	admin := env.signIn(t, adminUserID)

	env.expectRole(adminUserID, "ADMIN")
	env.expectRejectedTopUp(adminUserID, 250000, models.TopUpSourceAdmin, "TICKET-43", adminUserID, models.ErrSelfTopUp)

	rec := env.request(http.MethodPost, "/v1/admin/topup", adminTopUpBody(adminUserID, 250000, "TICKET-43"), bearer(admin.AccessToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAdminTopUpRejectsBindingErrors(t *testing.T) {
	env := newTestEnv(t)
	admin := env.signIn(t, adminUserID)

	for _, body := range []string{
		`{"user_id":"` + testUserID + `","amount":1000,"remarks":"no reference"}`,
		`{"user_id":"` + testUserID + `","amount":-1000,"reference":"T-1","remarks":"negative"}`,
		`{"amount":1000,"reference":"T-1","remarks":"no user"}`,
		`{"user_id":`,
	} {
		env.expectRole(adminUserID, "ADMIN")
		rec := env.request(http.MethodPost, "/v1/admin/topup", body, bearer(admin.AccessToken))
		assert.Equal(t, http.StatusNotAcceptable, rec.Code, body)
	}
}
