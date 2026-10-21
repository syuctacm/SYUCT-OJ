// Package message implements Hydro's user-to-user message collection
// (packages/hydrooj/src/model/message.ts) for the single-site rewrite.
package message

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Flag bits mirror Hydro's MessageModel flags.
const (
	FlagUnread   = 1
	FlagAlert    = 2
	FlagRichtext = 4
	FlagInfo     = 8
	FlagI18N     = 16
)

var ErrNotFound = errors.New("message not found")

// Message is one document of the `message` collection.
type Message struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	From      int64         `bson:"from" json:"from"`
	To        []int64       `bson:"to" json:"to"`
	Content   string        `bson:"content" json:"content"`
	Flag      int64         `bson:"flag" json:"flag"`
	CreatedAt time.Time     `bson:"-" json:"createdAt"`
}

type Repository struct {
	coll  *mongo.Collection
	users *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("message"), users: db.Collection("user")}
}

func (r *Repository) touch(items []Message) {
	for i := range items {
		items[i].CreatedAt = items[i].ID.Timestamp()
	}
}

// Send stores a message and bumps the receivers' unread counter, the same way
// MessageModel.send does upstream.
func (r *Repository) Send(ctx context.Context, from int64, to []int64, content string, flag int64) (*Message, error) {
	item := Message{
		ID:        bson.NewObjectID(),
		From:      from,
		To:        to,
		Content:   content,
		Flag:      flag,
		CreatedAt: time.Now(),
	}
	if _, err := r.coll.InsertOne(ctx, bson.M{
		"_id": item.ID, "from": from, "to": to, "content": content, "flag": flag,
	}); err != nil {
		return nil, err
	}
	if flag&FlagUnread != 0 && len(to) > 0 {
		_, _ = r.users.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": to}},
			bson.M{"$inc": bson.M{"unreadMsg": int64(1)}})
	}
	return &item, nil
}

// ListByUser returns the newest messages a user sent or received.
func (r *Repository) ListByUser(ctx context.Context, uid int64, limit int) ([]Message, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	cursor, err := r.coll.Find(ctx,
		bson.M{"$or": []bson.M{{"from": uid}, {"to": uid}}},
		options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Message, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	r.touch(items)
	return items, nil
}

// Delete removes a message the caller sent; other users' messages stay.
func (r *Repository) Delete(ctx context.Context, id bson.ObjectID, uid int64) error {
	result, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "from": uid})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkRead clears the unread counter, matching the profile handler upstream.
func (r *Repository) MarkRead(ctx context.Context, uid int64) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": bson.M{"unreadMsg": int64(0)}})
	return err
}

// Unread reports the stored unread counter.
func (r *Repository) Unread(ctx context.Context, uid int64) (int64, error) {
	var doc struct {
		UnreadMsg int64 `bson:"unreadMsg"`
	}
	if err := r.users.FindOne(ctx, bson.M{"_id": uid},
		options.FindOne().SetProjection(bson.M{"unreadMsg": 1})).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}
		return 0, err
	}
	return doc.UnreadMsg, nil
}
