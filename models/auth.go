package models

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/go-redis/redis/v7"
	jwt "github.com/golang-jwt/jwt/v4"
	uuid "github.com/google/uuid"
)

//TokenDetails ...
type TokenDetails struct {
	AccessToken  string
	RefreshToken string
	AccessUUID   string
	RefreshUUID  string
	SessionID    string
	AtExpires    int64
	RtExpires    int64
}

//AccessDetails ...
type AccessDetails struct {
	AccessUUID string
	SessionID  string
	UserID     string
}

//RefreshDetails are the verified claims of a refresh token.
type RefreshDetails struct {
	RefreshUUID string
	SessionID   string
	UserID      string
	Expires     int64
}

//Token ...
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

//AuthModel ...
type AuthModel struct{}

var (
	//ErrInvalidToken ...
	ErrInvalidToken = errors.New("invalid token")
	//ErrRefreshTokenReused is returned when a refresh token is presented after it was already rotated.
	ErrRefreshTokenReused = errors.New("refresh token was already used")
	//ErrSessionRevoked is returned when the session was logged out or has expired.
	ErrSessionRevoked = errors.New("session is no longer active")
)

const minSecretLength = 32

//placeholderSecretPrefix marks the sample values in .env_copy.
const placeholderSecretPrefix = "CHANGE_ME"

//ValidateSecrets refuses missing, placeholder, short or shared JWT signing secrets, and a weak
//PAYMENT_WEBHOOK_SECRET when one is set.
func ValidateSecrets() error {
	access, refresh := os.Getenv("ACCESS_SECRET"), os.Getenv("REFRESH_SECRET")
	if err := checkSecret("ACCESS_SECRET", access); err != nil {
		return err
	}
	if err := checkSecret("REFRESH_SECRET", refresh); err != nil {
		return err
	}
	if access == refresh {
		return errors.New("ACCESS_SECRET and REFRESH_SECRET must be different")
	}
	if webhook := os.Getenv("PAYMENT_WEBHOOK_SECRET"); webhook != "" {
		return checkSecret("PAYMENT_WEBHOOK_SECRET", webhook)
	}
	return nil
}

func checkSecret(name, value string) error {
	switch {
	case value == "":
		return fmt.Errorf("%s is not set", name)
	case strings.HasPrefix(value, placeholderSecretPrefix):
		return fmt.Errorf("%s still holds the placeholder from .env_copy; generate one with: openssl rand -base64 48", name)
	case len(value) < minSecretLength:
		return fmt.Errorf("%s must be at least %d characters; generate one with: openssl rand -base64 48", name, minSecretLength)
	}
	return nil
}

//A session is one login. Its Redis hash holds the access and refresh UUIDs currently issued for it,
//so logout and reuse detection can revoke both tokens.
func sessionKey(sessionID string) string {
	return "session:" + sessionID
}

func usedRefreshKey(refreshUUID string) string {
	return "refresh_used:" + refreshUUID
}

func hmacKey(secretEnv string) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv(secretEnv)), nil
	}
}

//CreateToken ...
func (m AuthModel) CreateToken(userID, sessionID string) (*TokenDetails, error) {

	td := &TokenDetails{SessionID: sessionID}
	td.AtExpires = time.Now().Add(time.Minute * 15).Unix()
	td.AccessUUID = uuid.New().String()

	td.RtExpires = time.Now().Add(time.Hour * 24 * 7).Unix()
	td.RefreshUUID = uuid.New().String()

	var err error
	//Creating Access Token
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	atClaims["access_uuid"] = td.AccessUUID
	atClaims["session_id"] = sessionID
	atClaims["user_id"] = userID
	atClaims["exp"] = td.AtExpires

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(os.Getenv("ACCESS_SECRET")))
	if err != nil {
		return nil, err
	}
	//Creating Refresh Token
	rtClaims := jwt.MapClaims{}
	rtClaims["refresh_uuid"] = td.RefreshUUID
	rtClaims["session_id"] = sessionID
	rtClaims["user_id"] = userID
	rtClaims["exp"] = td.RtExpires
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(os.Getenv("REFRESH_SECRET")))
	if err != nil {
		return nil, err
	}
	return td, nil
}

//CreateSession issues and stores the token pair for a new login.
func (m AuthModel) CreateSession(userID string) (*TokenDetails, error) {
	td, err := m.CreateToken(userID, uuid.New().String())
	if err != nil {
		return nil, err
	}
	if err := m.CreateAuth(userID, td); err != nil {
		return nil, err
	}
	return td, nil
}

//CreateAuth ...
func (m AuthModel) CreateAuth(userid string, td *TokenDetails) error {
	at := time.Unix(td.AtExpires, 0) //converting Unix to UTC(to Time object)
	rt := time.Unix(td.RtExpires, 0)
	now := time.Now()

	_, err := db.GetRedis().TxPipelined(func(pipe redis.Pipeliner) error {
		pipe.Set(td.AccessUUID, userid, at.Sub(now))
		pipe.Set(td.RefreshUUID, userid, rt.Sub(now))
		pipe.HSet(sessionKey(td.SessionID), "access_uuid", td.AccessUUID, "refresh_uuid", td.RefreshUUID)
		pipe.Expire(sessionKey(td.SessionID), rt.Sub(now))
		return nil
	})
	return err
}

//rotateSession consumes a refresh token and installs the next token pair in one step, so a
//concurrent logout or a second use of the same refresh token cannot slip in between.
//Returns 0 when rotated, 1 when the refresh token is no longer live, 2 when it was already used.
var rotateSession = redis.NewScript(`
if not redis.call('SET', KEYS[3], ARGV[1], 'NX', 'EX', ARGV[3]) then return 2 end
if redis.call('DEL', KEYS[2]) == 0 then return 1 end
local previous = redis.call('HGET', KEYS[1], 'access_uuid')
if previous then redis.call('DEL', previous) end
redis.call('SET', KEYS[4], ARGV[2], 'EX', ARGV[4])
redis.call('SET', KEYS[5], ARGV[2], 'EX', ARGV[5])
redis.call('HSET', KEYS[1], 'access_uuid', KEYS[4], 'refresh_uuid', KEYS[5])
redis.call('EXPIRE', KEYS[1], ARGV[5])
return 0
`)

var revokeSession = redis.NewScript(`
local tokens = redis.call('HMGET', KEYS[1], 'access_uuid', 'refresh_uuid')
for _, key in ipairs(tokens) do
  if key then redis.call('DEL', key) end
end
return redis.call('DEL', KEYS[1])
`)

//RotateSession exchanges a refresh token for a new token pair in the same session. Presenting a
//refresh token a second time means it leaked, so the whole session is revoked.
func (m AuthModel) RotateSession(rd *RefreshDetails) (*TokenDetails, error) {
	td, err := m.CreateToken(rd.UserID, rd.SessionID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	result, err := rotateSession.Run(db.GetRedis(),
		[]string{sessionKey(rd.SessionID), rd.RefreshUUID, usedRefreshKey(rd.RefreshUUID), td.AccessUUID, td.RefreshUUID},
		rd.SessionID, rd.UserID,
		wholeSeconds(time.Unix(rd.Expires, 0).Sub(now)),
		wholeSeconds(time.Unix(td.AtExpires, 0).Sub(now)),
		wholeSeconds(time.Unix(td.RtExpires, 0).Sub(now)),
	).Int64()
	if err != nil {
		return nil, err
	}
	switch result {
	case 0:
		return td, nil
	case 2:
		if err := m.RevokeSession(rd.SessionID); err != nil {
			return nil, err
		}
		return nil, ErrRefreshTokenReused
	default:
		return nil, ErrSessionRevoked
	}
}

//RevokeSession deletes the session's current access and refresh UUIDs, so neither token works again.
func (m AuthModel) RevokeSession(sessionID string) error {
	return revokeSession.Run(db.GetRedis(), []string{sessionKey(sessionID)}).Err()
}

//ParseRefreshToken verifies a refresh token and returns its claims.
func (m AuthModel) ParseRefreshToken(tokenString string) (*RefreshDetails, error) {
	token, err := jwt.Parse(tokenString, hmacKey("REFRESH_SECRET"))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid || !claims.VerifyExpiresAt(time.Now().Unix(), true) {
		return nil, ErrInvalidToken
	}
	refreshUUID, _ := claims["refresh_uuid"].(string)
	sessionID, _ := claims["session_id"].(string)
	userID, _ := claims["user_id"].(string)
	expires, _ := claims["exp"].(float64)
	if refreshUUID == "" || sessionID == "" || userID == "" {
		return nil, ErrInvalidToken
	}
	return &RefreshDetails{
		RefreshUUID: refreshUUID,
		SessionID:   sessionID,
		UserID:      userID,
		Expires:     int64(expires),
	}, nil
}

//ExtractToken ...
func (m AuthModel) ExtractToken(r *http.Request) string {
	bearToken := r.Header.Get("Authorization")
	//normally Authorization the_token_xxx
	strArr := strings.Split(bearToken, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}

//VerifyToken ...
func (m AuthModel) VerifyToken(r *http.Request) (*jwt.Token, error) {
	tokenString := m.ExtractToken(r)
	token, err := jwt.Parse(tokenString, hmacKey("ACCESS_SECRET"))
	if err != nil {
		return nil, err
	}
	return token, nil
}

//TokenValid ...
func (m AuthModel) TokenValid(r *http.Request) error {
	token, err := m.VerifyToken(r)
	if err != nil {
		return err
	}
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		return err
	}
	return nil
}

//ExtractTokenMetadata ...
func (m AuthModel) ExtractTokenMetadata(r *http.Request) (*AccessDetails, error) {
	token, err := m.VerifyToken(r)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	accessUUID, _ := claims["access_uuid"].(string)
	sessionID, _ := claims["session_id"].(string)
	userID, _ := claims["user_id"].(string)
	if accessUUID == "" || sessionID == "" {
		return nil, ErrInvalidToken
	}
	return &AccessDetails{
		AccessUUID: accessUUID,
		SessionID:  sessionID,
		UserID:     userID,
	}, nil
}

//FetchAuth ...
func (m AuthModel) FetchAuth(authD *AccessDetails) (string, error) {
	userid, err := db.GetRedis().Get(authD.AccessUUID).Result()
	if err != nil {
		return "", err
	}
	// userID, _ := strconv.ParseInt(userid, 10, 64)
	return userid, nil
}

//DeleteAuth ...
func (m AuthModel) DeleteAuth(givenUUID string) (int64, error) {
	deleted, err := db.GetRedis().Del(givenUUID).Result()
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
