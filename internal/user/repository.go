package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID          int64     `bson:"_id" json:"id"`
	Mail        string    `bson:"mail" json:"email"`
	MailLower   string    `bson:"mailLower" json:"-"`
	Uname       string    `bson:"uname" json:"username"`
	UnameLower  string    `bson:"unameLower" json:"-"`
	Salt        string    `bson:"salt" json:"-"`
	Hash        string    `bson:"hash" json:"-"`
	HashType    string    `bson:"hashType" json:"-"`
	Priv        int64     `bson:"priv" json:"priv"`
	RegAt       time.Time `bson:"regat" json:"regAt"`
	LoginAt     time.Time `bson:"loginat" json:"loginAt"`
	DisplayName string    `bson:"displayName" json:"displayName,omitempty"`
	StudentID   string    `bson:"studentId" json:"studentId,omitempty"`
	Avatar      string    `bson:"avatar" json:"avatar,omitempty"`
	Bio         string    `bson:"bio" json:"bio,omitempty"`
}

type Repository struct {
	collection *mongo.Collection
	records    *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{collection: db.Collection("user"), records: db.Collection("record")}
}

func (r *Repository) FindByLogin(ctx context.Context, identifier string) (*User, error) {
	lower := strings.ToLower(strings.TrimSpace(identifier))
	if lower == "" {
		return nil, ErrNotFound
	}
	var u User
	err := r.collection.FindOne(ctx, bson.M{
		"$or": []bson.M{
			{"unameLower": lower},
			{"mailLower": lower},
			{"uname": identifier},
			{"mail": identifier},
		},
	}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*User, error) {
	var u User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) Statistics(ctx context.Context, uid int64) (int64, int64, error) {
	submit, err := r.records.CountDocuments(ctx, bson.M{"uid": uid, "domainId": "system"})
	if err != nil {
		return 0, 0, err
	}
	result := r.records.Distinct(ctx, "pid", bson.M{"uid": uid, "domainId": "system", "status": 1})
	if err := result.Err(); err != nil {
		return 0, 0, err
	}
	var pids []any
	if err := result.Decode(&pids); err != nil {
		return 0, 0, err
	}
	return submit, int64(len(pids)), nil
}
