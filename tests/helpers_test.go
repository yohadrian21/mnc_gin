package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/Massad/gin-boilerplate/router"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-gorp/gorp"
	"github.com/go-redis/redis/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAccessSecret  = "test-only-access-secret-not-for-production-use"
	testRefreshSecret = "test-only-refresh-secret-not-for-production-use"
	testWebhookSecret = "test-only-webhook-secret-not-for-production-use"
	testUserID        = "6f1c2a8e-8a59-4d1b-9a43-3c1f0d7f2b10"

	testMaxTopUp       = 1000000.0
	testMaxTopUpPerDay = 1500000.0
)

var testAuthModel = new(models.AuthModel)

// testEnv wires the real router to a sqlmock database and an in-memory Redis.
// sqlmock fails any query that was not explicitly expected, so a test that sets
// no expectations also proves the request never reached the database.
type testEnv struct {
	router *gin.Engine
	sql    sqlmock.Sqlmock
	redis  *miniredis.Miniredis
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("ACCESS_SECRET", testAccessSecret)
	t.Setenv("REFRESH_SECRET", testRefreshSecret)
	t.Setenv("PAYMENT_WEBHOOK_SECRET", testWebhookSecret)
	t.Setenv("TOPUP_MAX_PER_TRANSACTION", fmt.Sprint(testMaxTopUp))
	t.Setenv("TOPUP_MAX_PER_DAY", fmt.Sprint(testMaxTopUpPerDay))

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db.SetDB(&gorp.DbMap{Db: sqlDB, Dialect: gorp.PostgresDialect{}})

	mr := miniredis.RunT(t)
	db.RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})

	r, err := router.New()
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.RedisClient.Close()
		sqlDB.Close()
	})
	return &testEnv{router: r, sql: mock, redis: mr}
}

const (
	testClientIP   = "192.0.2.1"
	testClientAddr = testClientIP + ":41000"
)

func (e *testEnv) request(method, path, body string, header http.Header) *httptest.ResponseRecorder {
	return e.requestFrom(testClientAddr, method, path, body, header)
}

func (e *testEnv) requestFrom(remoteAddr, method, path, body string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	for key, values := range header {
		req.Header[key] = values
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func bearer(token string) http.Header {
	return http.Header{"Authorization": {"Bearer " + token}}
}

// signIn creates a session for userID the same way a successful PIN login does.
func (e *testEnv) signIn(t *testing.T, userID string) *models.TokenDetails {
	t.Helper()
	td, err := testAuthModel.CreateSession(userID)
	require.NoError(t, err)
	return td
}

func TestProtectedRoutesRequireToken(t *testing.T) {
	env := newTestEnv(t)

	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/payment"},
		{http.MethodPost, "/v1/transfer"},
		{http.MethodGet, "/v1/transactions"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := env.request(route.method, route.path, `{}`, nil)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)

			rec = env.request(route.method, route.path, `{}`, bearer("not-a-jwt"))
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}
