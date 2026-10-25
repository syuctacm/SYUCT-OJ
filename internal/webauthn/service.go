package webauthn

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/syuctacm/syuct-oj/internal/user"
)

var (
	ErrNoCredentials    = errors.New("no WebAuthn credentials")
	ErrChallengeInvalid = errors.New("WebAuthn challenge is invalid or expired")
	ErrCredentialExists = errors.New("WebAuthn credential belongs to another user")
)

type CredentialInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"createdAt"`
	Attachment string    `json:"attachment,omitempty"`
	DeviceType string    `json:"deviceType,omitempty"`
	Format     string    `json:"format,omitempty"`
}

type Config struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
}

type Service struct {
	users       *user.Repository
	credentials *mongo.Collection
	challenges  *mongo.Collection
	rp          *wa.WebAuthn
}

type challengeDocument struct {
	ID        string    `bson:"_id"`
	UID       int64     `bson:"uid"`
	Purpose   string    `bson:"purpose"`
	Data      string    `bson:"data"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type credentialDocument struct {
	ID        string    `bson:"_id"`
	UID       int64     `bson:"uid"`
	Name      string    `bson:"name"`
	Data      string    `bson:"data"`
	CreatedAt time.Time `bson:"createdAt"`
}

type userAdapter struct {
	doc         user.User
	handle      []byte
	credentials []wa.Credential
}

func (u *userAdapter) WebAuthnID() []byte   { return u.handle }
func (u *userAdapter) WebAuthnName() string { return u.doc.Uname }
func (u *userAdapter) WebAuthnDisplayName() string {
	if u.doc.DisplayName != "" {
		return u.doc.DisplayName
	}
	return u.doc.Uname
}
func (u *userAdapter) WebAuthnCredentials() []wa.Credential { return u.credentials }

func New(ctx context.Context, db *mongo.Database, users *user.Repository, cfg Config) (*Service, error) {
	rp, err := wa.New(&wa.Config{
		RPID:          cfg.RPID,
		RPDisplayName: cfg.RPDisplayName,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return nil, err
	}
	service := &Service{
		users:       users,
		credentials: db.Collection("webauthn_credentials"),
		challenges:  db.Collection("webauthn_challenges"),
		rp:          rp,
	}
	if _, err := service.challenges.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetName("webauthn_challenge_ttl").SetExpireAfterSeconds(0),
	}); err != nil {
		return nil, err
	}
	if _, err := service.credentials.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "uid", Value: 1}},
		Options: options.Index().SetName("webauthn_credentials_uid"),
	}); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *Service) BeginRegistration(ctx context.Context, uid int64) (*protocol.CredentialCreation, string, error) {
	account, err := s.loadUser(ctx, uid)
	if err != nil {
		return nil, "", err
	}
	opts := []wa.RegistrationOption{
		wa.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
		wa.WithExclusions(wa.Credentials(account.credentials).CredentialDescriptors()),
	}
	creation, session, err := s.rp.BeginRegistration(account, opts...)
	if err != nil {
		return nil, "", err
	}
	token, err := s.saveChallenge(ctx, uid, "register", session)
	if err != nil {
		return nil, "", err
	}
	return creation, token, nil
}

func (s *Service) FinishRegistration(ctx context.Context, uid int64, token, name string, request *http.Request) (*CredentialInfo, error) {
	session, err := s.consumeChallenge(ctx, token, uid, "register")
	if err != nil {
		return nil, err
	}
	account, err := s.loadUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	credential, err := s.rp.FinishRegistration(account, *session, request)
	if err != nil {
		return nil, err
	}
	if err := s.saveCredential(ctx, uid, name, credential); err != nil {
		return nil, err
	}
	info := infoFromCredential(credential, normalizeName(name), time.Now().UTC())
	return &info, nil
}

func (s *Service) BeginLogin(ctx context.Context, username string) (*protocol.CredentialAssertion, string, error) {
	accountDoc, err := s.users.FindByLogin(ctx, username)
	if err != nil {
		return nil, "", err
	}
	count, err := s.credentials.CountDocuments(ctx, bson.M{"uid": accountDoc.ID})
	if err != nil {
		return nil, "", err
	}
	if count == 0 {
		return nil, "", ErrNoCredentials
	}
	account, err := s.loadUser(ctx, accountDoc.ID)
	if err != nil {
		return nil, "", err
	}
	if len(account.credentials) == 0 {
		return nil, "", ErrNoCredentials
	}
	assertion, session, err := s.rp.BeginLogin(account, wa.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return nil, "", err
	}
	token, err := s.saveChallenge(ctx, accountDoc.ID, "login", session)
	if err != nil {
		return nil, "", err
	}
	return assertion, token, nil
}

func (s *Service) FinishLogin(ctx context.Context, token string, request *http.Request) (*user.User, error) {
	challenge, err := s.consumeChallengeDocument(ctx, token, 0, "login")
	if err != nil {
		return nil, err
	}
	account, err := s.loadUser(ctx, challenge.UID)
	if err != nil {
		return nil, err
	}
	session := new(wa.SessionData)
	if err := json.Unmarshal([]byte(challenge.Data), session); err != nil {
		return nil, ErrChallengeInvalid
	}
	credential, err := s.rp.FinishLogin(account, *session, request)
	if err != nil {
		return nil, err
	}
	if err := s.updateCredential(ctx, challenge.UID, credential); err != nil {
		return nil, err
	}
	return &account.doc, nil
}

func (s *Service) ListCredentials(ctx context.Context, uid int64) ([]CredentialInfo, error) {
	cursor, err := s.credentials.Find(ctx, bson.M{"uid": uid}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]CredentialInfo, 0)
	for cursor.Next(ctx) {
		var doc credentialDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var credential wa.Credential
		if err := json.Unmarshal([]byte(doc.Data), &credential); err != nil {
			continue
		}
		items = append(items, infoFromCredential(&credential, doc.Name, doc.CreatedAt))
	}
	return items, cursor.Err()
}

func (s *Service) DeleteCredential(ctx context.Context, uid int64, id string) error {
	result, err := s.credentials.DeleteOne(ctx, bson.M{"_id": id, "uid": uid})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return user.ErrNotFound
	}
	return nil
}

func (s *Service) loadUser(ctx context.Context, uid int64) (*userAdapter, error) {
	account, err := s.users.FindByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	handle, err := s.users.EnsureWebAuthnID(ctx, uid)
	if err != nil {
		return nil, err
	}
	cursor, err := s.credentials.Find(ctx, bson.M{"uid": uid})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	credentials := make([]wa.Credential, 0)
	for cursor.Next(ctx) {
		var doc credentialDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var credential wa.Credential
		if err := json.Unmarshal([]byte(doc.Data), &credential); err != nil {
			continue
		}
		credentials = append(credentials, credential)
	}
	return &userAdapter{doc: *account, handle: handle, credentials: credentials}, nil
}

func (s *Service) saveCredential(ctx context.Context, uid int64, name string, credential *wa.Credential) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	name = normalizeName(name)
	id := base64.RawURLEncoding.EncodeToString(credential.ID)
	var existing credentialDocument
	err = s.credentials.FindOne(ctx, bson.M{"_id": id}).Decode(&existing)
	if err == nil && existing.UID != uid {
		return ErrCredentialExists
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	if err == nil {
		_, err = s.credentials.UpdateOne(ctx, bson.M{"_id": id, "uid": uid},
			bson.M{"$set": bson.M{"name": name, "data": string(data)}})
		return err
	}
	_, err = s.credentials.InsertOne(ctx, credentialDocument{
		ID: id, UID: uid, Name: name, Data: string(data), CreatedAt: time.Now().UTC(),
	})
	return err
}

func (s *Service) updateCredential(ctx context.Context, uid int64, credential *wa.Credential) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	id := base64.RawURLEncoding.EncodeToString(credential.ID)
	result, err := s.credentials.UpdateOne(ctx, bson.M{"_id": id, "uid": uid}, bson.M{"$set": bson.M{"data": string(data)}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return user.ErrNotFound
	}
	return nil
}

func (s *Service) saveChallenge(ctx context.Context, uid int64, purpose string, session *wa.SessionData) (string, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(token)
	now := time.Now().UTC()
	_, err = s.challenges.InsertOne(ctx, challengeDocument{
		ID: code, UID: uid, Purpose: purpose, Data: string(data), ExpiresAt: now.Add(5 * time.Minute),
	})
	return code, err
}

func (s *Service) consumeChallenge(ctx context.Context, token string, uid int64, purpose string) (*wa.SessionData, error) {
	doc, err := s.consumeChallengeDocument(ctx, token, uid, purpose)
	if err != nil {
		return nil, err
	}
	session := new(wa.SessionData)
	if err := json.Unmarshal([]byte(doc.Data), session); err != nil {
		return nil, ErrChallengeInvalid
	}
	return session, nil
}

func (s *Service) consumeChallengeDocument(ctx context.Context, token string, uid int64, purpose string) (*challengeDocument, error) {
	filter := bson.M{"_id": token, "purpose": purpose, "expiresAt": bson.M{"$gt": time.Now().UTC()}}
	if uid > 0 {
		filter["uid"] = uid
	}
	var doc challengeDocument
	err := s.challenges.FindOneAndDelete(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrChallengeInvalid
		}
		return nil, err
	}
	return &doc, nil
}

func infoFromCredential(credential *wa.Credential, name string, createdAt time.Time) CredentialInfo {
	deviceType := "singleDevice"
	if credential.Flags.BackupEligible {
		deviceType = "multiDevice"
	}
	return CredentialInfo{
		ID:         base64.RawURLEncoding.EncodeToString(credential.ID),
		Name:       name,
		CreatedAt:  createdAt,
		Attachment: string(credential.Authenticator.Attachment),
		DeviceType: deviceType,
		Format:     credential.AttestationFormat,
	}
}

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Passkey"
	}
	runes := []rune(name)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return name
}

// BindRequest returns a copy of the request with the WebAuthn response JSON as
// its body, as required by the library's Finish* helpers.
func BindRequest(request *http.Request, body []byte) *http.Request {
	clone := request.Clone(request.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.Header.Set("Content-Type", "application/json")
	return clone
}
