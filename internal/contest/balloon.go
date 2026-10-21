package contest

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BalloonColor is the per-problem balloon configuration shown on the balloon
// status page and stored in the contest document.
type BalloonColor struct {
	Name  string `bson:"name" json:"name"`
	Color string `bson:"color" json:"color"`
}

// Balloon is one pending or delivered balloon.
type Balloon struct {
	ID     bson.ObjectID `bson:"_id" json:"id"`
	UID    int64         `bson:"uid" json:"uid"`
	PID    int64         `bson:"pid" json:"pid"`
	First  bool          `bson:"first,omitempty" json:"first"`
	Sent   int64         `bson:"sent,omitempty" json:"sent,omitempty"`
	SentAt *time.Time    `bson:"sentAt,omitempty" json:"sentAt,omitempty"`
}

// Balloons lists the balloons of a contest, newest first.
func (r *Repository) Balloons(ctx context.Context, tid string) ([]Balloon, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	cursor, err := r.balloons.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docId":    oid,
	}, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Balloon, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// AddBalloon records the first accepted submission of a user and problem,
// mirroring the upstream balloon creation on judge completion.
func (r *Repository) AddBalloon(ctx context.Context, contestID bson.ObjectID, uid, pid int64) error {
	if uid <= 0 || pid <= 0 {
		return nil
	}
	existing, err := r.balloons.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem,
		"docId":    contestID,
		"uid":      uid,
		"pid":      pid,
	})
	if err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	first, err := r.balloons.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem,
		"docId":    contestID,
		"pid":      pid,
	})
	if err != nil {
		return err
	}
	_, err = r.balloons.InsertOne(ctx, bson.M{
		"_id":      bson.NewObjectID(),
		"domainId": DomainSystem,
		"docId":    contestID,
		"uid":      uid,
		"pid":      pid,
		"first":    first == 0,
	})
	return err
}

// MarkBalloonSent marks a balloon as delivered by the given user.
func (r *Repository) MarkBalloonSent(ctx context.Context, tid string, bid bson.ObjectID, uid int64) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	now := time.Now()
	result, err := r.balloons.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docId":    oid,
		"_id":      bid,
		"sent":     bson.M{"$exists": false},
	}, bson.M{"$set": bson.M{"sent": uid, "sentAt": now}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// SetBalloonColors stores the per-problem balloon configuration.
func (r *Repository) SetBalloonColors(ctx context.Context, tid string, colors map[int64]BalloonColor) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	stored := make(map[string]BalloonColor, len(colors))
	for pid, color := range colors {
		stored[formatInt(pid)] = color
	}
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}, bson.M{"$set": bson.M{"balloon": stored}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
