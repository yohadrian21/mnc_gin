package tests

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loginBody(phone, pin string) string {
	return fmt.Sprintf(`{"phone_number":%q,"pin":%q}`, phone, pin)
}

func (e *testEnv) expectUnknownPhone(phone string) {
	e.sql.ExpectQuery(regexp.QuoteMeta("FROM public.user WHERE phone_number=LOWER($1)")).
		WithArgs(phone).
		WillReturnRows(sqlmock.NewRows([]string{"id", "phone_number", "first_name", "last_name", "address", "pin"}))
}

func TestLoginLocksOutPhoneAfterRepeatedFailures(t *testing.T) {
	env := newTestEnv(t)
	limits := models.DefaultLoginLimiter
	hash := pinHash(t, testPIN)

	for i := 0; i < limits.MaxPhoneFailures; i++ {
		env.expectLoginLookup(testPhone, hash)
		rec := env.request(http.MethodPost, "/v1/login", loginBody(testPhone, "000000"), nil)
		require.Equal(t, http.StatusNotAcceptable, rec.Code, "attempt %d", i+1)
	}

	// Locked: even the correct PIN is refused, and the database is not consulted.
	rec := env.request(http.MethodPost, "/v1/login", loginBody(testPhone, testPIN), nil)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, fmt.Sprint(int(limits.Lockout/time.Second)), rec.Header().Get("Retry-After"))

	// The lock targets the phone, not every request from this client.
	env.expectLoginLookup("081111111111", hash)
	rec = env.request(http.MethodPost, "/v1/login", loginBody("081111111111", testPIN), nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	env.redis.FastForward(limits.Lockout)
	env.expectLoginLookup(testPhone, hash)
	rec = env.request(http.MethodPost, "/v1/login", loginBody(testPhone, testPIN), nil)
	assert.Equal(t, http.StatusOK, rec.Code, "lockout should expire")
}

func TestLoginLocksOutIPAfterRepeatedFailures(t *testing.T) {
	env := newTestEnv(t)
	limits := models.DefaultLoginLimiter

	for i := 0; i < limits.MaxIPFailures; i++ {
		phone := fmt.Sprintf("0899%08d", i)
		env.expectUnknownPhone(phone)
		rec := env.request(http.MethodPost, "/v1/login", loginBody(phone, testPIN), nil)
		require.Equal(t, http.StatusNotAcceptable, rec.Code, "attempt %d", i+1)
	}

	rec := env.request(http.MethodPost, "/v1/login", loginBody("0899fresh", testPIN), nil)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	spoofed := http.Header{"X-Forwarded-For": {"198.51.100.7"}}
	rec = env.request(http.MethodPost, "/v1/login", loginBody("0899fresh", testPIN), spoofed)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "X-Forwarded-For from an untrusted peer must not change the client IP")

	env.expectUnknownPhone("0899fresh")
	rec = env.requestFrom("198.51.100.7:41000", http.MethodPost, "/v1/login", loginBody("0899fresh", testPIN), nil)
	assert.Equal(t, http.StatusNotAcceptable, rec.Code, "other client IPs are not locked")
}

func TestParallelLoginGuessesCannotExceedPhoneCap(t *testing.T) {
	env := newTestEnv(t)
	env.sql.MatchExpectationsInOrder(false)
	limits := models.DefaultLoginLimiter
	hash := pinHash(t, testPIN)
	for i := 0; i < limits.MaxPhoneFailures; i++ {
		env.expectLoginLookup(testPhone, hash)
	}

	const guesses = 15
	codes := make(chan int, guesses)
	var wg sync.WaitGroup
	for i := 0; i < guesses; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes <- env.request(http.MethodPost, "/v1/login", loginBody(testPhone, fmt.Sprintf("%06d", i)), nil).Code
		}(i)
	}
	wg.Wait()
	close(codes)

	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	assert.Equal(t, limits.MaxPhoneFailures, counts[http.StatusNotAcceptable], "only the capped number of guesses may reach the PIN check")
	assert.Equal(t, guesses-limits.MaxPhoneFailures, counts[http.StatusTooManyRequests])
}

func TestSuccessfulLoginClearsPhoneFailures(t *testing.T) {
	env := newTestEnv(t)
	hash := pinHash(t, testPIN)

	for i := 0; i < models.DefaultLoginLimiter.MaxPhoneFailures-1; i++ {
		env.expectLoginLookup(testPhone, hash)
		env.request(http.MethodPost, "/v1/login", loginBody(testPhone, "000000"), nil)
	}
	env.expectLoginLookup(testPhone, hash)
	rec := env.request(http.MethodPost, "/v1/login", loginBody(testPhone, testPIN), nil)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.False(t, env.redis.Exists("login_attempts:phone:"+testPhone))
	ipFailures, err := env.redis.Get("login_attempts:ip:192.0.2.1")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprint(models.DefaultLoginLimiter.MaxPhoneFailures-1), ipFailures, "a success only returns its own IP attempt")
}
