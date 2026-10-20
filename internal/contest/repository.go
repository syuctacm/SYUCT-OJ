package contest

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DocumentTypeContest = 30
	DomainSystem        = "system"
)

var ErrNotFound = fmt.Errorf("contest not found")

type Contest struct {
	ID       bson.ObjectID `bson:"_id" json:"id"`
	DocID    bson.ObjectID `bson:"docId" json:"docId"`
	Title    string        `bson:"title" json:"title"`
	Content  string        `bson:"content" json:"content,omitempty"`
	BeginAt  time.Time     `bson:"beginAt" json:"beginAt"`
	EndAt    time.Time     `bson:"endAt" json:"endAt"`
	Rule     string        `bson:"rule" json:"rule"`
	Attend   int64         `bson:"attend" json:"attend"`
	PIDs     []int64       `bson:"pids" json:"pids"`
	Duration int           `bson:"duration" json:"duration"`
	Rated    bool          `bson:"rated" json:"rated"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("document")}
}

func (r *Repository) List(ctx context.Context) ([]Contest, error) {
	projection := bson.M{
		"docId": 1, "title": 1, "beginAt": 1, "endAt": 1,
		"rule": 1, "attend": 1, "pids": 1, "duration": 1, "rated": 1,
	}
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
	}, options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "beginAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Contest, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, tid string) (*Contest, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	var contest Contest
	err = r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}).Decode(&contest)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &contest, nil
}
