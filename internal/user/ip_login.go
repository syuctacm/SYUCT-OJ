package user

import (
	"context"
	"errors"
	"net"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type IPLogin struct {
	IP  string `bson:"_id" json:"ip"`
	UID int64  `bson:"uid" json:"uid"`
}

func NormalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "::ffff:") {
		value = strings.TrimPrefix(value, "::ffff:")
	}
	parsed := net.ParseIP(value)
	if parsed == nil {
		return ""
	}
	return parsed.String()
}

func (r *Repository) SetIPLogin(ctx context.Context, ip string, uid int64) error {
	ip = NormalizeIP(ip)
	if ip == "" || uid <= 0 {
		return ErrNotFound
	}
	_, err := r.ipLogins.UpdateOne(ctx, bson.M{"_id": ip}, bson.M{"$set": bson.M{"uid": uid}}, options.UpdateOne().SetUpsert(true))
	return err
}

func (r *Repository) DeleteIPLogin(ctx context.Context, ip string) error {
	ip = NormalizeIP(ip)
	if ip == "" {
		return ErrNotFound
	}
	_, err := r.ipLogins.DeleteOne(ctx, bson.M{"_id": ip})
	return err
}

func (r *Repository) ListIPLogins(ctx context.Context) ([]IPLogin, error) {
	cursor, err := r.ipLogins.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]IPLogin, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) IPLoginUID(ctx context.Context, ip string) (int64, bool, error) {
	ip = NormalizeIP(ip)
	if ip == "" {
		return 0, false, nil
	}
	var item IPLogin
	if err := r.ipLogins.FindOne(ctx, bson.M{"_id": ip}).Decode(&item); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return item.UID, true, nil
}
