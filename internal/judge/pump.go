package judge

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TaskPump struct {
	tasks    *mongo.Collection
	hub      *Hub
	interval time.Duration
}

func NewTaskPump(db *mongo.Database, hub *Hub) *TaskPump {
	return &TaskPump{
		tasks:    db.Collection("task"),
		hub:      hub,
		interval: 250 * time.Millisecond,
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
			var task bson.M
			err := p.tasks.FindOneAndDelete(ctx, bson.M{"type": "judge"}, options.FindOneAndDelete().
				SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "_id", Value: 1}})).
				Decode(&task)
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue
			}
			if err != nil {
				slog.Warn("unable to fetch judge task", "error", err)
				continue
			}
			p.hub.Enqueue(task)
		}
	}
}
