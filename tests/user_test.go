package tests

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	testPhone = "081234567890"
	testPIN   = "482915"
)

var userResponseFields = []string{"id", "phone_number", "first_name", "last_name", "address"}

func pinHash(t *testing.T, pin string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.MinCost)
	require.NoError(t, err)
	return string(hash)
}

func (e *testEnv) expectLoginLookup(phone, hash string) {
	e.sql.ExpectQuery(regexp.QuoteMeta("FROM public.user WHERE phone_number=LOWER($1)")).
		WithArgs(phone).
		WillReturnRows(sqlmock.NewRows([]string{"id", "phone_number", "first_name", "last_name", "address", "pin"}).
			AddRow(testUserID, phone, "Budi", "Santoso", "Jl. Merdeka 1", hash))
}

func fieldNames(object map[string]interface{}) []string {
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}
	return names
}

func TestLoginResponseHasNoPINHash(t *testing.T) {
	env := newTestEnv(t)
	hash := pinHash(t, testPIN)
	env.expectLoginLookup(testPhone, hash)

	rec := env.request(http.MethodPost, "/v1/login", `{"phone_number":"`+testPhone+`","pin":"`+testPIN+`"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.NotContains(t, rec.Body.String(), hash)
	assert.NotContains(t, rec.Body.String(), "$2a$")

	var body struct {
		User  map[string]interface{} `json:"user"`
		Token models.Token           `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.ElementsMatch(t, userResponseFields, fieldNames(body.User))
	assert.Equal(t, testUserID, body.User["id"])
	assert.NotEmpty(t, body.Token.AccessToken)
	assert.NotEmpty(t, body.Token.RefreshToken)
}

func TestRegisterResponseHasNoPINHash(t *testing.T) {
	env := newTestEnv(t)
	env.sql.ExpectQuery(regexp.QuoteMeta("SELECT count(id) FROM public.user")).
		WithArgs(testPhone).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	env.sql.ExpectQuery(regexp.QuoteMeta("INSERT INTO public.user")).
		WithArgs(sqlmock.AnyArg(), "Budi", "Santoso", testPhone, "Jl. Merdeka 1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUserID))

	rec := env.request(http.MethodPost, "/v1/register",
		`{"phone_number":"`+testPhone+`","first_name":"Budi","last_name":"Santoso","address":"Jl. Merdeka 1","pin":"`+testPIN+`"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.NotContains(t, rec.Body.String(), "$2a$")
	var body struct {
		User map[string]interface{} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.ElementsMatch(t, userResponseFields, fieldNames(body.User))
}
