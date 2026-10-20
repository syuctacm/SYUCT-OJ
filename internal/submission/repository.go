package submission

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/syuctacm/syuct-oj/internal/problem"
)

const (
	StatusWaiting   = 0
	StatusAccepted  = 1
	StatusJudging   = 20
	StatusCompiling = 21
)

type Record struct {
	ID            bson.ObjectID `bson:"_id" json:"id"`
	UID           int64         `bson:"uid" json:"uid"`
	PID           int64         `bson:"pid" json:"pid"`
	DomainID      string        `bson:"domainId" json:"domainId"`
	Lang          string        `bson:"lang" json:"lang"`
	Code          string        `bson:"code" json:"code,omitempty"`
	Status        int           `bson:"status" json:"status"`
	Score         int           `bson:"score" json:"score"`
	Time          int           `bson:"time" json:"time"`
	Memory        int           `bson:"memory" json:"memory"`
	JudgeTexts    []any         `bson:"judgeTexts" json:"judgeTexts,omitempty"`
	CompilerTexts []string      `bson:"compilerTexts" json:"compilerTexts,omitempty"`
	TestCases     []any         `bson:"testCases" json:"testCases,omitempty"`
	Judger        *int64        `bson:"judger" json:"judger,omitempty"`
	JudgeAt       *time.Time    `bson:"judgeAt" json:"judgeAt,omitempty"`
	Rejudged      bool          `bson:"rejudged" json:"rejudged"`
	Source        string        `bson:"source" json:"source,omitempty"`
}

type ListResult struct {
	Items []Record `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
}

type Repository struct {
	records  *mongo.Collection
	tasks    *mongo.Collection
	problems *problem.Repository
}

func NewRepository(db *mongo.Database, problems *problem.Repository) *Repository {
	return &Repository{
		records:  db.Collection("record"),
		tasks:    db.Collection("task"),
		problems: problems,
	}
}

func (r *Repository) Create(ctx context.Context, uid int64, pid, lang, code string) (*Record, error) {
	pdoc, err := r.problems.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	if lang == "" || code == "" {
		return nil, fmt.Errorf("lang and code are required")
	}
	rid := bson.NewObjectID()
	now := time.Now().UTC()
	record := bson.M{
		"_id": rid, "uid": uid, "pid": pdoc.DocID, "domainId": "system",
		"lang": lang, "code": code, "status": StatusWaiting,
		"score": 0, "time": 0, "memory": 0,
		"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{},
		"judger": nil, "judgeAt": nil, "rejudged": false,
		"source": fmt.Sprintf("system/%d", pdoc.DocID), "createdAt": now,
	}
	if _, err := r.records.InsertOne(ctx, record); err != nil {
		return nil, err
	}

	config, err := configMap(pdoc.Config)
	if err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	task := bson.M{
		"_id": bson.NewObjectID(), "type": "judge", "rid": rid,
		"lang": lang, "code": code, "uid": uid, "pid": pdoc.DocID,
		"domainId": "system", "data": pdoc.Data,
		"source": fmt.Sprintf("system/%d", pdoc.DocID), "config": config,
		"meta": bson.M{"problemOwner": pdoc.Owner}, "priority": 0,
		"trusted": false,
	}
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	return r.Get(ctx, rid.Hex())
}

func (r *Repository) ListByUser(ctx context.Context, uid int64, page, limit int) (*ListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	filter := bson.M{"uid": uid}
	total, err := r.records.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	projection := bson.M{
		"_id": 1, "uid": 1, "pid": 1, "domainId": 1, "lang": 1,
		"status": 1, "score": 1, "time": 1, "memory": 1,
		"judgeAt": 1, "rejudged": 1, "source": 1,
	}
	findOpts := options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))
	cursor, err := r.records.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Record, 0, limit)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (r *Repository) Get(ctx context.Context, rid string) (*Record, error) {
	oid, err := bson.ObjectIDFromHex(rid)
	if err != nil {
		return nil, fmt.Errorf("invalid record id")
	}
	var record Record
	if err := r.records.FindOne(ctx, bson.M{"_id": oid}).Decode(&record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Repository) ApplyJudgeResult(ctx context.Context, payload map[string]any) error {
	rawRID, ok := payload["rid"].(string)
	if !ok {
		return fmt.Errorf("judge result has no rid")
	}
	oid, err := bson.ObjectIDFromHex(rawRID)
	if err != nil {
		return fmt.Errorf("invalid judge result rid: %w", err)
	}
	set := bson.M{"judgeAt": time.Now().UTC()}
	for _, key := range []string{"status", "score", "time", "memory", "progress", "judger", "subtasks"} {
		if value, ok := payload[key]; ok {
			set[key] = value
		}
	}
	push := bson.M{}
	if value, ok := payload["message"]; ok && value != nil {
		push["judgeTexts"] = value
	}
	if value, ok := payload["compilerText"]; ok && value != nil && value != "" {
		push["compilerTexts"] = value
	}
	if value, ok := payload["case"]; ok && value != nil {
		push["testCases"] = value
	}
	if value, ok := payload["cases"]; ok && value != nil {
		switch cases := value.(type) {
		case []any:
			push["testCases"] = bson.M{"$each": cases}
		default:
			push["testCases"] = cases
		}
	}
	update := bson.M{"$set": set}
	if len(push) > 0 {
		update["$push"] = push
	}
	_, err = r.records.UpdateOne(ctx, bson.M{"_id": oid}, update)
	return err
}

func configMap(value any) (map[string]any, error) {
	switch config := value.(type) {
	case nil:
		return map[string]any{}, nil
	case string:
		result := map[string]any{}
		if err := yaml.Unmarshal([]byte(config), &result); err != nil {
			return nil, fmt.Errorf("parse problem config: %w", err)
		}
		return result, nil
	case bson.M:
		return config, nil
	case bson.D:
		result := make(map[string]any, len(config))
		for _, item := range config {
			result[item.Key] = item.Value
		}
		return result, nil
	case map[string]any:
		return config, nil
	default:
		return nil, fmt.Errorf("unsupported problem config type %T", value)
	}
}
