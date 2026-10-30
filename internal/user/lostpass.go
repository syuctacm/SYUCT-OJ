package user

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// LostPassTokenType mirrors Hydro's TokenModel.TYPE_LOSTPASS.
const LostPassTokenType = 5

var ErrLostPassTokenInvalid = errors.New("lost password token is invalid or expired")

// CreateLostPassToken stores a one-time password-reset token for a user.
func (r *Repository) CreateLostPassToken(ctx context.Context, uid int64, ttl time.Duration) (string, error) {
	if uid <= 0 {
		return "", ErrNotFound
	}
	code, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if _, err := r.tokens.InsertOne(ctx, bson.M{
		"_id": code, "tokenType": LostPassTokenType, "uid": uid,
		"createAt": now, "updateAt": now, "expireAt": now.Add(ttl),
	}); err != nil {
		return "", err
	}
	return code, nil
}

// LostPassToken returns the uid bound to an unexpired reset token.
func (r *Repository) LostPassToken(ctx context.Context, code string) (int64, error) {
	var document struct {
		UID      int64     `bson:"uid"`
		ExpireAt time.Time `bson:"expireAt"`
	}
	if err := r.tokens.FindOne(ctx, bson.M{"_id": code, "tokenType": LostPassTokenType}).Decode(&document); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, ErrLostPassTokenInvalid
		}
		return 0, err
	}
	if !document.ExpireAt.IsZero() && time.Now().After(document.ExpireAt) {
		_ = r.DeleteLostPassToken(ctx, code)
		return 0, ErrLostPassTokenInvalid
	}
	return document.UID, nil
}

func (r *Repository) DeleteLostPassToken(ctx context.Context, code string) error {
	_, err := r.tokens.DeleteOne(ctx, bson.M{"_id": code, "tokenType": LostPassTokenType})
	return err
}

// ResetPassword replaces the password and invalidates secondary credentials.
// Existing browser sessions are revoked separately by the handler.
func (r *Repository) ResetPassword(ctx context.Context, uid int64, password string) error {
	if uid <= 0 || password == "" {
		return ErrNotFound
	}
	salt, err := randomToken(16)
	if err != nil {
		return err
	}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{
		"$set":   bson.M{"salt": salt, "hash": HashPassword(password, salt), "hashType": "hydro"},
		"$unset": bson.M{"tfa": ""},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	_, err = r.credentials.DeleteMany(ctx, bson.M{"uid": uid})
	return err
}
