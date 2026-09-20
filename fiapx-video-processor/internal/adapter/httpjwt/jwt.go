package httpjwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Issuer {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

func (i *Issuer) Issue(userID uuid.UUID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"exp":   time.Now().Add(i.ttl).Unix(),
		"iat":   time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(i.secret)
}

func (i *Issuer) Parse(token string) (uuid.UUID, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return i.secret, nil
	})
	if err != nil || !parsed.Valid {
		return uuid.Nil, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, jwt.ErrTokenInvalidClaims
	}
	sub, _ := claims["sub"].(string)
	return uuid.Parse(sub)
}
