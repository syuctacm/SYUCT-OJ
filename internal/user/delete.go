package user

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DeletionGracePeriod = 7 * 24 * time.Hour

type SessionRevoker interface {
	RevokeUser(context.Context, int64) error
}

// RequestDeletion schedules account anonymization after the grace period.
func (r *Repository) RequestDeletion(ctx context.Context, uid int64) (time.Time, error) {
	at := time.Now().UTC().Add(DeletionGracePeriod)
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": bson.M{"deleteAt": at}})
	if err != nil {
		return time.Time{}, err
	}
	if result.MatchedCount == 0 {
		return time.Time{}, ErrNotFound
	}
	return at, nil
}

func (r *Repository) CancelDeletion(ctx context.Context, uid int64) error {
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$unset": bson.M{"deleteAt": ""}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeletionAt(ctx context.Context, uid int64) (*time.Time, error) {
	var document struct {
		DeleteAt *time.Time `bson:"deleteAt"`
	}
	if err := r.collection.FindOne(ctx, bson.M{"_id": uid},
		options.FindOne().SetProjection(bson.M{"deleteAt": 1})).Decode(&document); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return document.DeleteAt, nil
}

// PurgeExpiredUsers anonymizes accounts whose grace period has expired. Public
// problem/contest/record content is intentionally preserved; only account and
// authentication data is removed or anonymized.
func PurgeExpiredUsers(ctx context.Context, db *mongo.Database, sessions SessionRevoker) (int64, error) {
	cursor, err := db.Collection("user").Find(ctx, bson.M{
		"deleteAt": bson.M{"$lte": time.Now().UTC()},
		"deleted":  bson.M{"$ne": true},
	})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var users []User
	if err := cursor.All(ctx, &users); err != nil {
		return 0, err
	}
	var purged int64
	for _, account := range users {
		uid := account.ID
		anon := "deleted-" + formatUID(uid)
		_, err := db.Collection("user").UpdateOne(ctx, bson.M{"_id": uid}, bson.M{
			"$set": bson.M{
				"uname": anon, "unameLower": anon, "displayName": "Deleted User",
				"mail": "", "mailLower": "", "avatar": "", "bio": "", "qq": "", "wechat": "",
				"school": "", "studentId": "", "backgroundImage": "", "priv": int64(0),
				"banned": true, "deleted": true, "salt": "", "hash": "", "hashType": "",
				"tfa": "", "webauthnId": []byte{}, "_files": []any{},
			},
			"$unset": bson.M{"deleteAt": ""},
		})
		if err != nil {
			slog.Warn("unable to anonymize expired account", "uid", uid, "error", err)
			continue
		}
		if sessions != nil {
			if err := sessions.RevokeUser(ctx, uid); err != nil {
				slog.Warn("unable to revoke deleted user sessions", "uid", uid, "error", err)
			}
		}
		_, _ = db.Collection("domain.user").DeleteMany(ctx, bson.M{"uid": uid})
		_, _ = db.Collection("token").DeleteMany(ctx, bson.M{"$or": []bson.M{{"uid": uid}, {"mail": account.Mail}}})
		_, _ = db.Collection("webauthn_credentials").DeleteMany(ctx, bson.M{"uid": uid})
		_, _ = db.Collection("message").DeleteMany(ctx, bson.M{"$or": []bson.M{{"from": uid}, {"to": uid}}})
		purged++
	}
	return purged, nil
}

func formatUID(uid int64) string {
	if uid == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for uid > 0 {
		i--
		digits[i] = byte('0' + uid%10)
		uid /= 10
	}
	return string(digits[i:])
}

func RunDeletionWorker(ctx context.Context, db *mongo.Database, sessions SessionRevoker, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	if count, err := PurgeExpiredUsers(ctx, db, sessions); err != nil {
		slog.Warn("account deletion cleanup failed", "error", err)
	} else if count > 0 {
		slog.Info("account deletion cleanup completed", "count", count)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if count, err := PurgeExpiredUsers(ctx, db, sessions); err != nil {
				slog.Warn("account deletion cleanup failed", "error", err)
			} else if count > 0 {
				slog.Info("account deletion cleanup completed", "count", count)
			}
		}
	}
}
