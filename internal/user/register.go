package user

// User registration: the upstream Hydro flow issues a registration token for an
// email address and then creates the account at /register/<code>. Without SMTP
// the token is handed back to the browser directly, which is exactly what
// upstream does when mail delivery is not configured.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RegistrationTokenType is Hydro's TokenModel.TYPE_REGISTRATION.
const RegistrationTokenType = 2

// DefaultPriv mirrors Hydro's PRIV_DEFAULT (user profile, create file, send
// message) so a database migrated back to upstream Hydro keeps working.
const DefaultPriv int64 = 16842756

var (
	ErrMailExists     = errors.New("email already registered")
	ErrUsernameExists = errors.New("username already taken")
	ErrTokenInvalid   = errors.New("registration token is invalid or expired")
)

type RegisterInput struct {
	Mail     string
	Username string
	Password string
	IP       string
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buffer), "="), nil
}

// FindByMail looks a user up by email address only.
func (r *Repository) FindByMail(ctx context.Context, mail string) (*User, error) {
	return r.findOne(ctx, bson.M{"mailLower": strings.ToLower(strings.TrimSpace(mail))})
}

// FindByUsername looks a user up by username only.
func (r *Repository) FindByUsername(ctx context.Context, username string) (*User, error) {
	return r.findOne(ctx, bson.M{"unameLower": strings.ToLower(strings.TrimSpace(username))})
}

func (r *Repository) findOne(ctx context.Context, filter bson.M) (*User, error) {
	var u User
	if err := r.collection.FindOne(ctx, filter).Decode(&u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// Create registers a new account and joins it to the single system site.
func (r *Repository) Create(ctx context.Context, input RegisterInput) (*User, error) {
	mail := strings.TrimSpace(input.Mail)
	username := strings.TrimSpace(input.Username)
	if mail == "" || username == "" || input.Password == "" {
		return nil, fmt.Errorf("mail, username and password are required")
	}
	if _, err := r.FindByMail(ctx, mail); err == nil {
		return nil, ErrMailExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if _, err := r.FindByUsername(ctx, username); err == nil {
		return nil, ErrUsernameExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var last struct {
		ID int64 `bson:"_id"`
	}
	err := r.collection.FindOne(ctx, bson.M{},
		options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}}).SetProjection(bson.M{"_id": 1}),
	).Decode(&last)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	uid := last.ID + 1
	if uid < 2 {
		uid = 2
	}
	salt, err := randomToken(16)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	document := bson.M{
		"_id": uid, "mail": mail, "mailLower": strings.ToLower(mail),
		"uname": username, "unameLower": strings.ToLower(username),
		"salt": salt, "hash": HashPassword(input.Password, salt), "hashType": "hydro",
		"regat": now, "loginat": now,
		"ip": []string{input.IP}, "loginip": input.IP,
		"priv": DefaultPriv, "avatar": "gravatar:" + mail,
	}
	if _, err := r.collection.InsertOne(ctx, document); err != nil {
		return nil, err
	}
	if _, err := r.domainUsers.UpdateOne(ctx,
		bson.M{"domainId": "system", "uid": uid},
		bson.M{"$set": bson.M{"join": true}},
		options.UpdateOne().SetUpsert(true)); err != nil {
		slog.Warn("unable to create domain.user row for new account", "uid", uid, "error", err)
	}
	return r.FindByID(ctx, uid)
}

// CreateRegistrationToken stores the emailed registration token.
func (r *Repository) CreateRegistrationToken(ctx context.Context, mail string, ttl time.Duration) (string, error) {
	code, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if _, err := r.tokens.InsertOne(ctx, bson.M{
		"_id": code, "tokenType": RegistrationTokenType, "mail": mail,
		"createAt": now, "updateAt": now, "expireAt": now.Add(ttl),
	}); err != nil {
		return "", err
	}
	return code, nil
}

// RegistrationToken returns the email bound to an unexpired token.
func (r *Repository) RegistrationToken(ctx context.Context, code string) (string, error) {
	var document struct {
		Mail     string    `bson:"mail"`
		ExpireAt time.Time `bson:"expireAt"`
	}
	if err := r.tokens.FindOne(ctx, bson.M{"_id": code, "tokenType": RegistrationTokenType}).Decode(&document); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", ErrTokenInvalid
		}
		return "", err
	}
	if !document.ExpireAt.IsZero() && time.Now().After(document.ExpireAt) {
		_ = r.DeleteRegistrationToken(ctx, code)
		return "", ErrTokenInvalid
	}
	return document.Mail, nil
}

func (r *Repository) DeleteRegistrationToken(ctx context.Context, code string) error {
	_, err := r.tokens.DeleteOne(ctx, bson.M{"_id": code, "tokenType": RegistrationTokenType})
	return err
}
