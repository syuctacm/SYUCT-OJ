package filemeta

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var ErrNotFound = fmt.Errorf("file owner document not found")

type File struct {
	ID           string    `bson:"_id" json:"id"`
	Name         string    `bson:"name" json:"name"`
	Size         int64     `bson:"size" json:"size"`
	ETag         string    `bson:"etag,omitempty" json:"etag,omitempty"`
	LastModified time.Time `bson:"lastModified" json:"lastModified"`
}

func Upsert(ctx context.Context, coll *mongo.Collection, filter bson.M, field string, file File) error {
	if _, err := coll.UpdateOne(ctx, filter, bson.M{"$pull": bson.M{field: bson.M{"_id": file.ID}}}); err != nil {
		return err
	}
	result, err := coll.UpdateOne(ctx, filter, bson.M{"$push": bson.M{field: file}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func Delete(ctx context.Context, coll *mongo.Collection, filter bson.M, field string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	result, err := coll.UpdateOne(ctx, filter, bson.M{"$pull": bson.M{field: bson.M{"_id": bson.M{"$in": ids}}}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
