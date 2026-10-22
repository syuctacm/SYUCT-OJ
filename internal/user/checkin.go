package user

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Checkin struct {
	CheckedIn bool   `json:"checkedIn"`
	Luck      int64  `json:"luck"`
	Time      string `json:"time,omitempty"`
	CountNow  int64  `json:"countNow"`
	CountAll  int64  `json:"countAll"`
}

func (r *Repository) GetCheckin(ctx context.Context, uid int64, today string) (*Checkin, error) {
	account, err := r.FindByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &Checkin{
		CheckedIn: account.CheckinTime == today,
		Luck:      account.CheckinLuck,
		Time:      account.CheckinTime,
		CountNow:  account.CheckinCountNow,
		CountAll:  account.CheckinCountAll,
	}, nil
}

// Checkin atomically records one check-in per day and updates the streak.
func (r *Repository) Checkin(ctx context.Context, uid int64, today, yesterday string, luck int64) (*Checkin, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$set", Value: bson.M{
			"checkin_luck": luck,
			"checkin_time": today,
			"checkin_cnt_now": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$checkin_time", yesterday}},
				bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$checkin_cnt_now", 0}}, 1}},
				1,
			}},
			"checkin_cnt_all": bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$checkin_cnt_all", 0}}, 1}},
		}}},
	}
	filter := bson.M{
		"_id": uid,
		"$or": []bson.M{
			{"checkin_time": bson.M{"$exists": false}},
			{"checkin_time": bson.M{"$ne": today}},
		},
	}
	var account User
	err := r.collection.FindOneAndUpdate(ctx, filter, pipeline,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&account)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return r.GetCheckin(ctx, uid, today)
	}
	if err != nil {
		return nil, err
	}
	return &Checkin{
		CheckedIn: true,
		Luck:      account.CheckinLuck,
		Time:      account.CheckinTime,
		CountNow:  account.CheckinCountNow,
		CountAll:  account.CheckinCountAll,
	}, nil
}
