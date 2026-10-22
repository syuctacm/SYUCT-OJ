package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// EmailChangeTokenType is Hydro's TokenModel.TYPE_CHANGEMAIL.
const EmailChangeTokenType = 3

var ErrEmailChangeTokenInvalid = errors.New("email change token is invalid or expired")

func (r *Repository) CreateEmailChangeToken(ctx context.Context, uid int64, mail string, ttl time.Duration) (string, error) {
	code, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	if _, err := r.tokens.InsertOne(ctx, bson.M{
		"_id": code, "tokenType": EmailChangeTokenType, "uid": uid,
		"mail": strings.TrimSpace(mail), "createAt": now, "updateAt": now,
		"expireAt": now.Add(ttl),
	}); err != nil {
		return "", err
	}
	return code, nil
}

func (r *Repository) EmailChangeToken(ctx context.Context, code string) (int64, string, error) {
	var document struct {
		UID      int64     `bson:"uid"`
		Mail     string    `bson:"mail"`
		ExpireAt time.Time `bson:"expireAt"`
	}
	if err := r.tokens.FindOne(ctx, bson.M{"_id": code, "tokenType": EmailChangeTokenType}).Decode(&document); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, "", ErrEmailChangeTokenInvalid
		}
		return 0, "", err
	}
	if !document.ExpireAt.IsZero() && time.Now().After(document.ExpireAt) {
		_ = r.DeleteEmailChangeToken(ctx, code)
		return 0, "", ErrEmailChangeTokenInvalid
	}
	return document.UID, strings.TrimSpace(document.Mail), nil
}

func (r *Repository) DeleteEmailChangeToken(ctx context.Context, code string) error {
	_, err := r.tokens.DeleteOne(ctx, bson.M{"_id": code, "tokenType": EmailChangeTokenType})
	return err
}

func (r *Repository) UpdateEmail(ctx context.Context, uid int64, mail string) error {
	mail = strings.TrimSpace(mail)
	if !ValidEmail(mail) {
		return fmt.Errorf("invalid email")
	}
	if existing, err := r.FindByMail(ctx, mail); err == nil && existing.ID != uid {
		return ErrMailExists
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": bson.M{
		"mail": mail, "mailLower": strings.ToLower(mail),
	}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
