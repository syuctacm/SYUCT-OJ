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
	DocumentTypeDiscussion      = 21
	DocumentTypeDiscussionReply = 22
	DomainSystem                = "system"
	MaxReplyLength              = 64 * 1024
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
	Lock      bool          `bson:"lock" json:"lock"`
}

type TailReply struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	Owner     int64         `bson:"owner" json:"owner"`
	Content   string        `bson:"content" json:"content"`
	Edited    bool          `bson:"edited" json:"edited"`
	CreatedAt time.Time     `bson:"-" json:"createdAt"`
}

type Reply struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	DocID     bson.ObjectID `bson:"docId" json:"docId"`
	ParentID  bson.ObjectID `bson:"parentId" json:"parentId"`
	Owner     int64         `bson:"owner" json:"owner"`
	Content   string        `bson:"content" json:"content"`
	Edited    bool          `bson:"edited" json:"edited"`
	Replies   []TailReply   `bson:"reply" json:"replies,omitempty"`
	CreatedAt time.Time     `bson:"-" json:"createdAt"`
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
		SetProjection(bson.M{"docId": 1, "title": 1, "owner": 1, "updateAt": 1, "nReply": 1, "views": 1, "pin": 1, "highlight": 1, "lock": 1}).
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

func (r *Repository) ListReplies(ctx context.Context, discussionID bson.ObjectID) ([]Reply, error) {
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeDiscussionReply,
		"parentType": DocumentTypeDiscussion,
		"parentId":   discussionID,
	}, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Reply, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CreatedAt = items[i].ID.Timestamp()
		for j := range items[i].Replies {
			items[i].Replies[j].CreatedAt = items[i].Replies[j].ID.Timestamp()
		}
	}
	return items, nil
}

func (r *Repository) AddReply(ctx context.Context, discussionID bson.ObjectID, owner int64, content string) (*Reply, error) {
	now := time.Now()
	replyID := bson.NewObjectID()
	item := Reply{
		ID:        replyID,
		DocID:     replyID,
		ParentID:  discussionID,
		Owner:     owner,
		Content:   content,
		CreatedAt: now,
	}
	_, err := r.coll.InsertOne(ctx, bson.M{
		"_id":        replyID,
		"domainId":   DomainSystem,
		"docType":    DocumentTypeDiscussionReply,
		"docId":      replyID,
		"parentType": DocumentTypeDiscussion,
		"parentId":   discussionID,
		"owner":      owner,
		"content":    content,
		"edited":     false,
	})
	if err != nil {
		return nil, err
	}
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"_id":      discussionID,
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
	}, bson.M{
		"$inc": bson.M{"nReply": 1},
		"$set": bson.M{"updateAt": now},
	})
	if err != nil {
		_, _ = r.coll.DeleteOne(ctx, bson.M{"_id": replyID})
		return nil, err
	}
	if result.MatchedCount == 0 {
		_, _ = r.coll.DeleteOne(ctx, bson.M{"_id": replyID})
		return nil, ErrNotFound
	}
	return &item, nil
}
