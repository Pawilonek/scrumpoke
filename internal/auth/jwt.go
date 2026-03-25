package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTIssuer issues and validates user tokens used for WebSocket auth.
// Tokens are signed with a secret generated on server startup.
type JWTIssuer struct {
	secret     []byte
	tokenTTL   time.Duration
	clockSkew  time.Duration
}

func NewJWTIssuer(tokenTTL time.Duration) (*JWTIssuer, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return &JWTIssuer{
		secret:    secret,
		tokenTTL:  tokenTTL,
		clockSkew: 30 * time.Second,
	}, nil
}

func (j *JWTIssuer) Issue(uuid string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(j.tokenTTL)

	claims := jwt.MapClaims{
		"sub": uuid,
		"exp": exp.Unix(),
		"iat": now.Unix(),
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (j *JWTIssuer) Validate(token string) (uuid string, exp time.Time, err error) {
	if token == "" {
		return "", time.Time{}, errors.New("empty token")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return "", time.Time{}, err
	}
	if !parsed.Valid {
		return "", time.Time{}, errors.New("invalid token")
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", time.Time{}, errors.New("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", time.Time{}, errors.New("missing sub")
	}

	expUnix, ok := claims["exp"].(float64) // jwt parses numbers as float64
	if !ok {
		return "", time.Time{}, errors.New("missing exp")
	}

	exp = time.Unix(int64(expUnix), 0)
	// Make sure it isn't "just expired" due to clock skew.
	if time.Now().After(exp.Add(j.clockSkew)) {
		return "", time.Time{}, errors.New("token expired")
	}
	return sub, exp, nil
}

// DebugSecretHex is intentionally only for debugging; not used by app logic.
func (j *JWTIssuer) DebugSecretHex() string {
	return hex.EncodeToString(j.secret)
}

