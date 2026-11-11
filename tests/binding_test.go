package tests

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoneyRoutesRejectBindingErrors(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	const (
		invalid       = "Invalid request"
		badAmount     = "Please enter an amount greater than zero"
		noRemarks     = "Please enter the remarks"
		noTargetUser  = "Please enter the target user"
		ledgerFailure = "Transactions could not be created"
	)

	// No SQL expectations: every request below must be rejected by binding, before any balance is read
	// or written. The message tells a binding rejection apart from the ledger code failing later on.
	for name, tc := range map[string]struct{ path, body, message string }{
		"payment: malformed JSON":       {"/v1/payment", `{"amount": 100, "remarks": `, invalid},
		"payment: empty body":           {"/v1/payment", ``, invalid},
		"payment: missing amount":       {"/v1/payment", `{"remarks":"coffee"}`, badAmount},
		"payment: zero amount":          {"/v1/payment", `{"amount":0,"remarks":"coffee"}`, badAmount},
		"payment: negative amount":      {"/v1/payment", `{"amount":-500,"remarks":"coffee"}`, badAmount},
		"payment: amount as string":     {"/v1/payment", `{"amount":"100","remarks":"coffee"}`, invalid},
		"payment: missing remarks":      {"/v1/payment", `{"amount":100}`, noRemarks},
		"transfer: malformed JSON":      {"/v1/transfer", `not json`, invalid},
		"transfer: missing target user": {"/v1/transfer", `{"amount":100,"remarks":"rent"}`, noTargetUser},
		"transfer: negative amount":     {"/v1/transfer", `{"target_user":"` + otherUserID + `","amount":-1,"remarks":"rent"}`, badAmount},
		"transfer: missing remarks":     {"/v1/transfer", `{"target_user":"` + otherUserID + `","amount":100}`, noRemarks},
	} {
		t.Run(name, func(t *testing.T) {
			rec := env.request(http.MethodPost, tc.path, tc.body, bearer(session.AccessToken))
			assert.Equal(t, http.StatusNotAcceptable, rec.Code, rec.Body.String())
			assert.JSONEq(t, `{"message":"`+tc.message+`"}`, rec.Body.String())
			assert.NotContains(t, rec.Body.String(), ledgerFailure)
		})
	}
}

const otherUserID = "0d9f3b4e-2c1a-4e6f-8b7d-5a4c3b2a1f00"

func TestValidPaymentStillReachesLedger(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	env.sql.ExpectBegin()
	env.sql.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM public.user WHERE id=$1 FOR UPDATE")).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(1000.0))
	env.sql.ExpectQuery(regexp.QuoteMeta("INSERT INTO public.transactions")).
		WithArgs(sqlmock.AnyArg(), testUserID, "DEBIT", 250.0, 1000.0, 750.0, "coffee", "SUCCESS", sqlmock.AnyArg(), "", sqlmock.AnyArg(), "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ledger-row"))
	env.sql.ExpectExec(regexp.QuoteMeta("UPDATE public.user SET balance = $1")).
		WithArgs(750.0, sqlmock.AnyArg(), testUserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	env.sql.ExpectCommit()

	rec := env.request(http.MethodPost, "/v1/payment", `{"amount":250,"remarks":"coffee"}`, bearer(session.AccessToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
