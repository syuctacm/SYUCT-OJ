package submission

import (
	"context"
	"fmt"
	"strings"
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

type ProblemInfo struct {
	DocID int64  `json:"docId"`
	PID   string `json:"pid"`
	Title string `json:"title"`
}

type UserInfo struct {
	ID          int64  `bson:"_id" json:"id"`
	Username    string `bson:"uname" json:"username"`
	DisplayName string `bson:"displayName" json:"displayName,omitempty"`
}

type Record struct {
	ID            bson.ObjectID  `bson:"_id" json:"id"`
	UID           int64          `bson:"uid" json:"uid"`
	PID           int64          `bson:"pid" json:"pid"`
	DomainID      string         `bson:"domainId" json:"domainId"`
	Lang          string         `bson:"lang" json:"lang"`
	Code          string         `bson:"code" json:"code,omitempty"`
	Status        int            `bson:"status" json:"status"`
	Score         int            `bson:"score" json:"score"`
	Time          int            `bson:"time" json:"time"`
	Memory        int            `bson:"memory" json:"memory"`
	JudgeTexts    []any          `bson:"judgeTexts" json:"judgeTexts,omitempty"`
	CompilerTexts []string       `bson:"compilerTexts" json:"compilerTexts,omitempty"`
	TestCases     []any          `bson:"testCases" json:"testCases,omitempty"`
	Judger        *int64         `bson:"judger" json:"judger,omitempty"`
	JudgeAt       *time.Time     `bson:"judgeAt" json:"judgeAt,omitempty"`
	Rejudged      bool           `bson:"rejudged" json:"rejudged"`
	Source        string         `bson:"source" json:"source,omitempty"`
	Contest       *bson.ObjectID `bson:"contest,omitempty" json:"contest,omitempty"`
	Problem       *ProblemInfo   `bson:"-" json:"problem,omitempty"`
	Submitter     *UserInfo      `bson:"-" json:"submitter,omitempty"`
	CreatedAt     time.Time      `bson:"-" json:"createdAt"`
}

type ListResult struct {
	Items []Record `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
}

type ListFilter struct {
	Page      int
	Limit     int
	UID       int64
	PID       string
	ContestID string
	Lang      string
	Status    *int
}

type Repository struct {
	records  *mongo.Collection
	tasks    *mongo.Collection
	users    *mongo.Collection
	problems *problem.Repository
}

func NewRepository(db *mongo.Database, problems *problem.Repository) *Repository {
	return &Repository{
		records:  db.Collection("record"),
		tasks:    db.Collection("task"),
		users:    db.Collection("user"),
		problems: problems,
	}
}

func (r *Repository) Create(ctx context.Context, uid int64, pid, lang, code string, contestID ...string) (*Record, error) {
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
	var contestObjectID *bson.ObjectID
	if len(contestID) > 0 && contestID[0] != "" {
		oid, err := bson.ObjectIDFromHex(contestID[0])
		if err != nil {
			_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
			return nil, fmt.Errorf("invalid contest id: %w", err)
		}
		contestObjectID = &oid
		record["contest"] = oid
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
	if contestObjectID != nil {
		task["contest"] = *contestObjectID
	}
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	return r.Get(ctx, rid.Hex())
}

func (r *Repository) List(ctx context.Context, opts ListFilter) (*ListResult, error) {
	opts = normalizeListFilter(opts)
	filter := bson.M{"domainId": "system"}
	if opts.UID > 0 {
		filter["uid"] = opts.UID
	}
	if opts.PID != "" {
		pdoc, err := r.problems.Get(ctx, opts.PID)
		if err != nil {
			if err == problem.ErrNotFound {
				return &ListResult{Items: []Record{}, Page: opts.Page, Limit: opts.Limit}, nil
			}
			return nil, err
		}
		filter["pid"] = pdoc.DocID
	}
	if opts.ContestID != "" {
		oid, err := bson.ObjectIDFromHex(opts.ContestID)
		if err != nil {
			return nil, fmt.Errorf("invalid contest id")
		}
		filter["contest"] = oid
	}
	if opts.Lang != "" {
		filter["lang"] = opts.Lang
	}
	if opts.Status != nil {
		filter["status"] = *opts.Status
	}
	total, err := r.records.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	projection := bson.M{
		"_id": 1, "uid": 1, "pid": 1, "domainId": 1, "lang": 1,
		"status": 1, "score": 1, "time": 1, "memory": 1,
		"judgeAt": 1, "rejudged": 1, "source": 1, "contest": 1,
	}
	findOpts := options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64((opts.Page - 1) * opts.Limit)).
		SetLimit(int64(opts.Limit))
	cursor, err := r.records.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Record, 0, opts.Limit)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CreatedAt = items[i].ID.Timestamp()
	}
	if err := r.enrich(ctx, items); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total, Page: opts.Page, Limit: opts.Limit}, nil
}

func (r *Repository) ListByUser(ctx context.Context, uid int64, page, limit int) (*ListResult, error) {
	return r.List(ctx, ListFilter{Page: page, Limit: limit, UID: uid})
}

func (r *Repository) enrich(ctx context.Context, items []Record) error {
	if len(items) == 0 {
		return nil
	}
	pidSet := map[int64]bool{}
	uidSet := map[int64]bool{}
	for _, item := range items {
		pidSet[item.PID] = true
		uidSet[item.UID] = true
	}
	pids := make([]int64, 0, len(pidSet))
	for pid := range pidSet {
		pids = append(pids, pid)
	}
	problems, err := r.problems.GetManyByDocIDs(ctx, pids)
	if err != nil {
		return err
	}
	problemByID := make(map[int64]ProblemInfo, len(problems))
	for _, pdoc := range problems {
		problemByID[pdoc.DocID] = ProblemInfo{DocID: pdoc.DocID, PID: pdoc.PID, Title: pdoc.Title}
	}
	uids := make([]int64, 0, len(uidSet))
	for uid := range uidSet {
		uids = append(uids, uid)
	}
	userCursor, err := r.users.Find(ctx, bson.M{"_id": bson.M{"$in": uids}}, options.Find().SetProjection(bson.M{"uname": 1, "displayName": 1}))
	if err != nil {
		return err
	}
	defer userCursor.Close(ctx)
	var users []UserInfo
	if err := userCursor.All(ctx, &users); err != nil {
		return err
	}
	userByID := make(map[int64]UserInfo, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}
	for i := range items {
		if pdoc, ok := problemByID[items[i].PID]; ok {
			value := pdoc
			items[i].Problem = &value
		}
		if user, ok := userByID[items[i].UID]; ok {
			value := user
			items[i].Submitter = &value
		}
	}
	return nil
}

func normalizeListFilter(opts ListFilter) ListFilter {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.Limit < 1 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	opts.PID = strings.TrimSpace(opts.PID)
	opts.ContestID = strings.TrimSpace(opts.ContestID)
	opts.Lang = strings.TrimSpace(opts.Lang)
	return opts
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
	record.CreatedAt = record.ID.Timestamp()
	items := []Record{record}
	if err := r.enrich(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
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
