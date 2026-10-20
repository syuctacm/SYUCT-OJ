package training

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DocumentTypeTraining = 40
	DomainSystem         = "system"
)

var ErrNotFound = fmt.Errorf("training not found")

type Training struct {
	ID          bson.ObjectID `bson:"_id" json:"id"`
	DocID       bson.ObjectID `bson:"docId" json:"docId"`
	Title       string        `bson:"title" json:"title"`
	Description string        `bson:"description" json:"description,omitempty"`
	Content     string        `bson:"content" json:"content,omitempty"`
	Pin         bool          `bson:"pin" json:"pin"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("document")}
}

func (r *Repository) List(ctx context.Context) ([]Training, error) {
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
	}, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "description": 1, "pin": 1}).
		SetSort(bson.D{{Key: "pin", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Training, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, tid string) (*Training, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	var item Training
	err = r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}).Decode(&item)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
