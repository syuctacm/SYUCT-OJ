package migration

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type legacyAuthenticator struct {
	Name                    string                 `bson:"name"`
	RegAt                   any                    `bson:"regat"`
	Format                  string                 `bson:"fmt"`
	Counter                 any                    `bson:"counter"`
	AAGUID                  any                    `bson:"aaguid"`
	CredentialID            any                    `bson:"credentialID"`
	CredentialPublicKey     any                    `bson:"credentialPublicKey"`
	AttestationObject       any                    `bson:"attestationObject"`
	UserVerified            bool                   `bson:"userVerified"`
	CredentialDeviceType    string                 `bson:"credentialDeviceType"`
	CredentialBackedUp      bool                   `bson:"credentialBackedUp"`
	AuthenticatorAttachment string                 `bson:"authenticatorAttachment"`
	Transports              []string               `bson:"transports"`
	Credential              legacyNestedCredential `bson:"credential"`
}

type legacyNestedCredential struct {
	ID        string   `bson:"id"`
	PublicKey any      `bson:"publicKey"`
	Counter   any      `bson:"counter"`
	Transport []string `bson:"transports"`
}

type plannedWebAuthnCredential struct {
	ID                 string
	Name               string
	CreatedAt          time.Time
	Credential         *wa.Credential
	Existing           bool
	PreviouslyImported bool
}

func convertLegacyAuthenticator(raw legacyAuthenticator) (string, *wa.Credential, time.Time, error) {
	credentialID, err := bytesValue(raw.CredentialID)
	if err != nil || len(credentialID) == 0 {
		if raw.Credential.ID != "" {
			credentialID, err = decodeBase64Flexible(raw.Credential.ID)
		}
	}
	if err != nil || len(credentialID) == 0 {
		return "", nil, time.Time{}, errors.New("legacy authenticator is missing credentialID")
	}
	publicKey, err := bytesValue(raw.CredentialPublicKey)
	if err != nil || len(publicKey) == 0 {
		publicKey, err = bytesValue(raw.Credential.PublicKey)
	}
	if err != nil || len(publicKey) == 0 {
		return "", nil, time.Time{}, errors.New("legacy authenticator is missing credentialPublicKey")
	}
	counterValue := raw.Counter
	if counterValue == nil {
		counterValue = raw.Credential.Counter
	}
	counter, err := uint32Value(counterValue)
	if err != nil {
		return "", nil, time.Time{}, fmt.Errorf("invalid WebAuthn counter: %w", err)
	}
	createdAt, err := timeValue(raw.RegAt)
	if err != nil {
		return "", nil, time.Time{}, fmt.Errorf("invalid authenticator registration time: %w", err)
	}
	deviceType := strings.TrimSpace(raw.CredentialDeviceType)
	if deviceType == "" {
		deviceType = "singleDevice"
	}
	flags := protocol.FlagUserPresent
	if raw.UserVerified {
		flags |= protocol.FlagUserVerified
	}
	if deviceType == "multiDevice" {
		flags |= protocol.FlagBackupEligible
		if raw.CredentialBackedUp {
			flags |= protocol.FlagBackupState
		}
	}
	aaguid, _ := aaguidValue(raw.AAGUID)
	attestationObject, _ := bytesValue(raw.AttestationObject)
	format := strings.TrimSpace(raw.Format)
	if format == "" {
		format = "none"
	}
	attachment := strings.TrimSpace(raw.AuthenticatorAttachment)
	if attachment == "" {
		attachment = "cross-platform"
	}
	transports := raw.Transports
	if len(transports) == 0 {
		transports = raw.Credential.Transport
	}
	credential := &wa.Credential{
		ID:                credentialID,
		PublicKey:         publicKey,
		AttestationFormat: format,
		Transport:         authenticatorTransports(transports),
		Flags:             wa.NewCredentialFlags(flags),
		Authenticator: wa.Authenticator{
			AAGUID:     aaguid,
			SignCount:  counter,
			Attachment: protocol.AuthenticatorAttachment(attachment),
		},
		Attestation: wa.CredentialAttestation{Object: attestationObject},
	}
	id := base64.RawURLEncoding.EncodeToString(credentialID)
	return id, credential, createdAt, nil
}

func bytesValue(value any) ([]byte, error) {
	switch item := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return append([]byte(nil), item...), nil
	case bson.Binary:
		return append([]byte(nil), item.Data...), nil
	case *bson.Binary:
		if item == nil {
			return nil, nil
		}
		return append([]byte(nil), item.Data...), nil
	case string:
		return decodeBase64Flexible(item)
	default:
		return nil, fmt.Errorf("unsupported binary value %T", value)
	}
}

func decodeBase64Flexible(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, encoding := range []*base64.Encoding{
		base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding,
	} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 value")
}

func aaguidValue(value any) ([]byte, error) {
	if bytes, err := bytesValue(value); err == nil && len(bytes) == 16 {
		return bytes, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported AAGUID value %T", value)
	}
	text = strings.ReplaceAll(strings.TrimSpace(text), "-", "")
	if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == 16 {
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid AAGUID")
}

func uint32Value(value any) (uint32, error) {
	switch item := value.(type) {
	case nil:
		return 0, nil
	case int:
		if item >= 0 {
			return uint32(item), nil
		}
	case int32:
		if item >= 0 {
			return uint32(item), nil
		}
	case int64:
		if item >= 0 && item <= int64(^uint32(0)) {
			return uint32(item), nil
		}
	case float64:
		if item >= 0 && item <= float64(^uint32(0)) && item == float64(uint32(item)) {
			return uint32(item), nil
		}
	}
	return 0, fmt.Errorf("value %v is not a uint32", value)
}

func timeValue(value any) (time.Time, error) {
	switch item := value.(type) {
	case nil:
		return time.Time{}, nil
	case time.Time:
		return item.UTC(), nil
	case bson.DateTime:
		return item.Time().UTC(), nil
	case int:
		return unixTimestamp(int64(item)), nil
	case int32:
		return unixTimestamp(int64(item)), nil
	case int64:
		return unixTimestamp(item), nil
	case float64:
		return unixTimestamp(int64(item)), nil
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, item)
		if err != nil {
			return time.Time{}, err
		}
		return parsed.UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp %T", value)
	}
}

func unixTimestamp(value int64) time.Time {
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func authenticatorTransports(values []string) []protocol.AuthenticatorTransport {
	seen := map[string]struct{}{}
	result := make([]protocol.AuthenticatorTransport, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, protocol.AuthenticatorTransport(value))
	}
	return result
}

func credentialName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Passkey"
	}
	if utf8.RuneCountInString(name) <= 64 {
		return name
	}
	runes := []rune(name)
	return string(runes[:64])
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
