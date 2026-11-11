package tests

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const profileUpdate = `{"phone_number":"089999999999","first_name":"Eve","last_name":"Wijaya","address":"Jl. Lain 2","pin":"000000"}`

func TestProfileUpdateRequiresToken(t *testing.T) {
	env := newTestEnv(t)

	// No SQL is expected: PUT /v1/profile used to run Register, which would INSERT a new user here.
	rec := env.request(http.MethodPut, "/v1/profile", profileUpdate, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = env.request(http.MethodPut, "/v1/profile", profileUpdate, bearer("not-a-jwt"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	session := env.signIn(t, testUserID)
	env.redis.Del(session.AccessUUID)
	rec = env.request(http.MethodPut, "/v1/profile", profileUpdate, bearer(session.AccessToken))
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a logged-out access token must not update the profile")
}

func TestProfileUpdateWithTokenUpdatesOwnProfile(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	env.sql.ExpectQuery(regexp.QuoteMeta("SELECT id, phone_number, first_name, last_name, address, balance FROM public.user WHERE id=$1")).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "phone_number", "first_name", "last_name", "address", "balance"}).
			AddRow(testUserID, testPhone, "Budi", "Santoso", "Jl. Merdeka 1", 1000.0))
	env.sql.ExpectExec(regexp.QuoteMeta("UPDATE public.user SET last_name = LOWER($1) WHERE id = $2")).
		WithArgs("Wijaya", testUserID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rec := env.request(http.MethodPut, "/v1/profile", `{"last_name":"Wijaya"}`, bearer(session.AccessToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		User map[string]interface{} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, testUserID, body.User["id"])
	assert.Equal(t, testPhone, body.User["phone_number"])
	assert.Equal(t, "Wijaya", body.User["last_name"])
	assert.ElementsMatch(t, userResponseFields, fieldNames(body.User))
}
