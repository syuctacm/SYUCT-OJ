package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	SecretLength = 13
	Period       = 30
	Digits       = 6
	secretChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

var base32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

func GenerateSecret() (string, error) {
	output := make([]byte, SecretLength)
	random := make([]byte, SecretLength)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index, value := range random {
		output[index] = secretChars[int(value)%len(secretChars)]
	}
	return string(output), nil
}

func ValidateSecret(secret string) error {
	secret = strings.ToUpper(strings.TrimSpace(secret))
	if secret == "" {
		return fmt.Errorf("secret is required")
	}
	if _, err := base32NoPadding.DecodeString(secret); err != nil {
		return fmt.Errorf("invalid secret")
	}
	return nil
}

func Code(secret string, at time.Time) (string, error) {
	key, err := decode(secret)
	if err != nil {
		return "", err
	}
	counter := uint64(at.Unix() / Period)
	return codeForCounter(key, counter), nil
}

func Verify(secret, code string, at time.Time) bool {
	code = normalizeCode(code)
	if len(code) != Digits {
		return false
	}
	key, err := decode(secret)
	if err != nil {
		return false
	}
	counter := uint64(at.Unix() / Period)
	for offset := -1; offset <= 1; offset++ {
		candidateCounter := counter
		if offset < 0 {
			if counter == 0 {
				continue
			}
			candidateCounter--
		} else if offset > 0 {
			candidateCounter++
		}
		candidate := codeForCounter(key, candidateCounter)
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func ProvisioningURI(issuer, account, secret string) string {
	label := issuer + ":" + account
	return "otpauth://totp/" + url.PathEscape(label) +
		"?secret=" + url.QueryEscape(strings.ToUpper(secret)) +
		"&issuer=" + url.QueryEscape(issuer)
}

func decode(secret string) ([]byte, error) {
	secret = strings.ToUpper(strings.TrimSpace(secret))
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}
	key, err := base32NoPadding.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("invalid secret")
	}
	return key, nil
}

func codeForCounter(key []byte, counter uint64) string {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(bytes)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1000000)
}

func normalizeCode(code string) string {
	var output strings.Builder
	for _, char := range code {
		if char >= '0' && char <= '9' {
			output.WriteRune(char)
		}
	}
	return output.String()
}
