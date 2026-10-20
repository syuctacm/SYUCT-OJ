package judge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ResultHandler func(map[string]any) error

type TaskPump struct {
	tasks         *mongo.Collection
	hub           *Hub
	interval      time.Duration
	leaseDuration time.Duration
	resultHandler ResultHandler
}

func NewTaskPump(db *mongo.Database, hub *Hub) *TaskPump {
	return &TaskPump{
		tasks:         db.Collection("task"),
		hub:           hub,
		interval:      250 * time.Millisecond,
		leaseDuration: 2 * time.Minute,
	}
}

func (p *TaskPump) SetResultHandler(handler ResultHandler) {
	p.resultHandler = handler
}

func (p *TaskPump) SetLeaseDuration(duration time.Duration) {
	if duration > 0 {
		p.leaseDuration = duration
	}
}

func (p *TaskPump) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !p.hub.HasStarted() {
				continue
			}
			task, err := p.leaseNext(ctx)
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue
			}
			if err != nil {
				slog.Warn("unable to lease judge task", "error", err)
				continue
			}
			if !p.hub.Enqueue(task) {
				p.release(ctx, task)
			}
		}
	}
}

func (p *TaskPump) leaseNext(ctx context.Context) (bson.M, error) {
	now := time.Now().UTC()
	filter := bson.M{
		"type": "judge",
		"$or": []bson.M{
			{"leaseUntil": bson.M{"$exists": false}},
			{"leaseUntil": bson.M{"$lte": now}},
		},
	}
	active := p.hub.ActiveRIDs()
	if len(active) > 0 {
		activeOIDs := make([]bson.ObjectID, 0, len(active))
		for _, rawRID := range active {
			if oid, err := bson.ObjectIDFromHex(rawRID); err == nil {
				activeOIDs = append(activeOIDs, oid)
			}
		}
		if len(activeOIDs) > 0 {
			filter["rid"] = bson.M{"$nin": activeOIDs}
		}
	}
	update := bson.M{
		"$set": bson.M{
			"leaseUntil":  now.Add(p.leaseDuration),
			"leasedAt":    now,
			"heartbeatAt": now,
		},
		"$inc": bson.M{"attempts": 1},
	}
	var task bson.M
	err := p.tasks.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "_id", Value: 1}}).
		SetReturnDocument(options.After)).
		Decode(&task)
	return task, err
}

func (p *TaskPump) release(ctx context.Context, task bson.M) {
	id, ok := task["_id"]
	if !ok {
		return
	}
	filter := bson.M{"_id": id}
	if leasedAt, ok := task["leasedAt"]; ok {
		filter["leasedAt"] = leasedAt
	}
	if _, err := p.tasks.UpdateOne(ctx, filter, bson.M{
		"$unset": bson.M{"leaseUntil": "", "leasedAt": "", "heartbeatAt": ""},
	}); err != nil {
		slog.Warn("unable to release judge task lease", "task", id, "error", err)
	}
}

func (p *TaskPump) HandleResult(ctx context.Context, payload map[string]any) error {
	key, _ := payload["key"].(string)
	rawRID, _ := payload["rid"].(string)
	rid, err := bson.ObjectIDFromHex(rawRID)
	if err != nil {
		return fmt.Errorf("invalid judge result rid %q: %w", rawRID, err)
	}
	var task bson.M
	err = p.tasks.FindOne(ctx, bson.M{"rid": rid}).Decode(&task)
	if errors.Is(err, mongo.ErrNoDocuments) {
		slog.Info("ignoring result for completed or unknown judge task", "key", key, "rid", rawRID)
		return nil
	}
	if err != nil {
		return err
	}
	if p.resultHandler != nil {
		if err := p.resultHandler(payload); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	switch key {
	case "next":
		_, err = p.tasks.UpdateOne(ctx, bson.M{"rid": rid}, bson.M{
			"$set": bson.M{
				"leaseUntil":  now.Add(p.leaseDuration),
				"heartbeatAt": now,
			},
		})
	case "end":
		_, err = p.tasks.DeleteOne(ctx, bson.M{"rid": rid})
	}
	return err
}
