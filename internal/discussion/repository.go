package discussion

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DocumentTypeDiscussion = 21
	DomainSystem           = "system"
)

var ErrNotFound = fmt.Errorf("discussion not found")

type Discussion struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	DocID     bson.ObjectID `bson:"docId" json:"docId"`
	Title     string        `bson:"title" json:"title"`
	Content   string        `bson:"content" json:"content,omitempty"`
	Owner     int64         `bson:"owner" json:"owner"`
	UpdateAt  time.Time     `bson:"updateAt" json:"updateAt"`
	NReply    int64         `bson:"nReply" json:"nReply"`
	Views     int64         `bson:"views" json:"views"`
	Pin       bool          `bson:"pin" json:"pin"`
	Highlight bool          `bson:"highlight" json:"highlight"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("document")}
}

func (r *Repository) List(ctx context.Context) ([]Discussion, error) {
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
	}, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "owner": 1, "updateAt": 1, "nReply": 1, "views": 1, "pin": 1, "highlight": 1}).
		SetSort(bson.D{{Key: "pin", Value: -1}, {Key: "updateAt", Value: -1}, {Key: "docId", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Discussion, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, did string) (*Discussion, error) {
	oid, err := bson.ObjectIDFromHex(did)
	if err != nil {
		return nil, ErrNotFound
	}
	var item Discussion
	err = r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
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
