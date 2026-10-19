package user

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const hydroHashRounds = 100000

func HashPassword(password, salt string) string {
	key := pbkdf2.Key([]byte(password), []byte(salt), hydroHashRounds, 64, sha256.New)
	return hex.EncodeToString(key)[:64]
}

func VerifyPassword(u *User, password string) error {
	if u.HashType != "" && u.HashType != "hydro" {
		return fmt.Errorf("unsupported password hash type %q", u.HashType)
	}
	actual := HashPassword(password, u.Salt)
	if subtle.ConstantTimeCompare([]byte(actual), []byte(u.Hash)) != 1 {
		return fmt.Errorf("invalid credentials")
	}
	return nil
}
