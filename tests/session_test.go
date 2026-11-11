package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Massad/gin-boilerplate/models"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func refreshBody(refreshToken string) string {
	return fmt.Sprintf(`{"refresh_token":%q}`, refreshToken)
}

func (e *testEnv) refresh(t *testing.T, refreshToken string) (int, models.Token) {
	t.Helper()
	rec := e.request(http.MethodPost, "/v1/token/refresh", refreshBody(refreshToken), nil)
	var tokens models.Token
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))
	}
	return rec.Code, tokens
}

// assertAccessAccepted checks a protected route accepts the token and resolves the real user ID,
// which the old %.f formatting of the user_id claim corrupted.
func (e *testEnv) assertAccessAccepted(t *testing.T, accessToken string) {
	t.Helper()
	e.sql.ExpectQuery(`FROM\s+public\.transactions\s+WHERE\s+user_id = \$1`).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows([]string{"result"}))
	rec := e.request(http.MethodGet, "/v1/transactions", "", bearer(accessToken))
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func (e *testEnv) assertAccessRejected(t *testing.T, accessToken string) {
	t.Helper()
	rec := e.request(http.MethodGet, "/v1/transactions", "", bearer(accessToken))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func claimsOf(t *testing.T, token, secret string) jwt.MapClaims {
	t.Helper()
	parsed, err := jwt.Parse(token, func(*jwt.Token) (interface{}, error) { return []byte(secret), nil })
	require.NoError(t, err)
	return parsed.Claims.(jwt.MapClaims)
}

func TestLogoutInvalidatesRefreshToken(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	rec := env.request(http.MethodGet, "/v1/user/logout", "", bearer(session.AccessToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	code, _ := env.refresh(t, session.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, code, "the refresh token must die with the session")

	env.assertAccessRejected(t, session.AccessToken)

	assert.False(t, env.redis.Exists(session.AccessUUID))
	assert.False(t, env.redis.Exists(session.RefreshUUID))
	assert.False(t, env.redis.Exists("session:"+session.SessionID))

	rec = env.request(http.MethodGet, "/v1/user/logout", "", bearer(session.AccessToken))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogoutOnlyEndsItsOwnSession(t *testing.T) {
	env := newTestEnv(t)
	phone := env.signIn(t, testUserID)
	laptop := env.signIn(t, testUserID)

	rec := env.request(http.MethodGet, "/v1/user/logout", "", bearer(phone.AccessToken))
	require.Equal(t, http.StatusOK, rec.Code)

	code, _ := env.refresh(t, laptop.RefreshToken)
	assert.Equal(t, http.StatusOK, code)
}

func TestRefreshRotatesTokensAndKeepsUserID(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	code, next := env.refresh(t, session.RefreshToken)
	require.Equal(t, http.StatusOK, code)
	assert.NotEqual(t, session.RefreshToken, next.RefreshToken)

	nextAccess := claimsOf(t, next.AccessToken, testAccessSecret)
	assert.Equal(t, testUserID, nextAccess["user_id"])
	assert.Equal(t, session.SessionID, nextAccess["session_id"], "rotation stays in the same session")
	stored, err := env.redis.Get(nextAccess["access_uuid"].(string))
	require.NoError(t, err)
	assert.Equal(t, testUserID, stored)

	env.assertAccessAccepted(t, next.AccessToken)
	env.assertAccessRejected(t, session.AccessToken) // rotation revokes the previous access token

	code, _ = env.refresh(t, next.RefreshToken)
	assert.Equal(t, http.StatusOK, code, "the rotated refresh token is usable once")
}

func TestRefreshTokenReuseRevokesSession(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	code, next := env.refresh(t, session.RefreshToken)
	require.Equal(t, http.StatusOK, code)

	// The original refresh token comes back: someone kept a copy.
	code, _ = env.refresh(t, session.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, code)

	code, _ = env.refresh(t, next.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, code, "reuse revokes the tokens issued by the rotation too")
	env.assertAccessRejected(t, next.AccessToken)
	assert.False(t, env.redis.Exists("session:"+session.SessionID))
}

func TestParallelRefreshWithOneTokenMintsOnePair(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	const attempts = 10
	codes := make(chan int, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- env.request(http.MethodPost, "/v1/token/refresh", refreshBody(session.RefreshToken), nil).Code
		}()
	}
	wg.Wait()
	close(codes)

	succeeded := 0
	for code := range codes {
		if code == http.StatusOK {
			succeeded++
		}
	}
	assert.Equal(t, 1, succeeded)
}

func TestRefreshRejectsTokensThatAreNotLiveRefreshTokens(t *testing.T) {
	env := newTestEnv(t)
	session := env.signIn(t, testUserID)

	withoutSession := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"refresh_uuid": session.RefreshUUID,
		"user_id":      testUserID,
		"exp":          time.Now().Add(time.Hour).Unix(),
	})
	legacy, err := withoutSession.SignedString([]byte(testRefreshSecret))
	require.NoError(t, err)

	for name, token := range map[string]string{
		"access token":                  session.AccessToken,
		"refresh token without session": legacy,
		"garbage":                       "not-a-jwt",
	} {
		t.Run(name, func(t *testing.T) {
			code, _ := env.refresh(t, token)
			assert.Equal(t, http.StatusUnauthorized, code)
		})
	}

	code, _ := env.refresh(t, session.RefreshToken)
	assert.Equal(t, http.StatusOK, code, "rejected attempts must not burn the real refresh token")
}
