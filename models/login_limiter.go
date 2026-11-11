package models

import (
	"strings"
	"time"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/go-redis/redis/v7"
)

//LoginLimiter caps unsuccessful PIN logins per phone number and per client IP within Window,
//then refuses further attempts for that phone or IP until Lockout has passed.
type LoginLimiter struct {
	MaxPhoneFailures int
	MaxIPFailures    int
	Window           time.Duration
	Lockout          time.Duration
}

//DefaultLoginLimiter ...
var DefaultLoginLimiter = LoginLimiter{
	MaxPhoneFailures: 5,
	MaxIPFailures:    20,
	Window:           15 * time.Minute,
	Lockout:          15 * time.Minute,
}

//reserveAttempt counts the attempt before the PIN is checked so parallel guesses cannot
//overshoot the cap. Once the cap is exceeded it sets the lock key and returns its TTL in seconds.
var reserveAttempt = redis.NewScript(`
local locked = redis.call('TTL', KEYS[2])
if locked > 0 then return locked end
local attempts = redis.call('INCR', KEYS[1])
if attempts == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
if attempts > tonumber(ARGV[1]) then
  redis.call('DEL', KEYS[1])
  redis.call('SET', KEYS[2], '1', 'EX', ARGV[3])
  return tonumber(ARGV[3])
end
return 0
`)

//releaseAttempt must not create the counter: a DECR on a missing key would leave one without a TTL.
var releaseAttempt = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then redis.call('DECR', KEYS[1]) end
return 0
`)

func loginLimitKeys(kind, value string) (attempts string, lock string) {
	return "login_attempts:" + kind + ":" + value, "login_lock:" + kind + ":" + value
}

func normalizePhone(phone string) string {
	return strings.ToLower(strings.TrimSpace(phone))
}

func wholeSeconds(d time.Duration) int64 {
	if d < time.Second {
		return 1
	}
	return int64(d / time.Second)
}

//Reserve records a login attempt for the phone and IP. A non-zero duration means the attempt is
//refused because one of them is locked out, and is how long the client should wait.
func (l LoginLimiter) Reserve(phone, ip string) (time.Duration, error) {
	for _, limit := range []struct {
		kind, value string
		max         int
	}{
		{"ip", ip, l.MaxIPFailures},
		{"phone", normalizePhone(phone), l.MaxPhoneFailures},
	} {
		attempts, lock := loginLimitKeys(limit.kind, limit.value)
		retryAfter, err := reserveAttempt.Run(db.GetRedis(), []string{attempts, lock}, limit.max, wholeSeconds(l.Window), wholeSeconds(l.Lockout)).Int64()
		if err != nil {
			return 0, err
		}
		if retryAfter > 0 {
			return time.Duration(retryAfter) * time.Second, nil
		}
	}
	return 0, nil
}

//Succeeded clears the phone's failures after a correct PIN. The IP only gets back the attempt just
//reserved, so logging in to one's own account cannot wipe failures piled up against other phones.
func (l LoginLimiter) Succeeded(phone, ip string) error {
	phoneAttempts, _ := loginLimitKeys("phone", normalizePhone(phone))
	if err := db.GetRedis().Del(phoneAttempts).Err(); err != nil {
		return err
	}
	ipAttempts, _ := loginLimitKeys("ip", ip)
	return releaseAttempt.Run(db.GetRedis(), []string{ipAttempts}).Err()
}
