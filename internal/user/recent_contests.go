package user

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ContestBrief is the small projection used by the user profile's recent
// activity tab. It intentionally contains no submission or score data.
type ContestBrief struct {
	DocID bson.ObjectID `bson:"docId" json:"docId"`
	Title string        `bson:"title" json:"title"`
	Rule  string        `bson:"rule" json:"rule"`
}

// RecentContests returns contests and homeworks the user has attended. Hydro
// stores both in document.status with docType=30 and attend=1.
func (r *Repository) RecentContests(ctx context.Context, uid int64, limit int64) ([]ContestBrief, error) {
	if uid <= 0 {
		return []ContestBrief{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	cursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": "system", "docType": 30, "uid": uid, "attend": 1,
	}, options.Find().
		SetProjection(bson.M{"docId": 1}).
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	type status struct {
		DocID bson.ObjectID `bson:"docId"`
	}
	statuses := make([]status, 0)
	if err := cursor.All(ctx, &statuses); err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, 0, len(statuses))
	seen := make(map[bson.ObjectID]bool, len(statuses))
	for _, item := range statuses {
		if item.DocID.IsZero() || seen[item.DocID] {
			continue
		}
		seen[item.DocID] = true
		ids = append(ids, item.DocID)
	}
	if len(ids) == 0 {
		return []ContestBrief{}, nil
	}
	cursor, err = r.documents.Find(ctx, bson.M{
		"domainId": "system", "docType": 30, "docId": bson.M{"$in": ids},
	}, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "rule": 1}).
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]ContestBrief, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}
