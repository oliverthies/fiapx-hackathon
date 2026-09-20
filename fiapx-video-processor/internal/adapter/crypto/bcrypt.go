package crypto

import "golang.org/x/crypto/bcrypt"

type Hasher struct{}

func (Hasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func (Hasher) Match(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
