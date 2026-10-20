package problem

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"
)

const (
	DocumentTypeProblem = 10
	DomainSystem        = "system"
)

var ErrNotFound = fmt.Errorf("problem not found")

type ProblemFile struct {
	ID           string    `bson:"_id" json:"-"`
	Name         string    `bson:"name" json:"-"`
	Size         int64     `bson:"size" json:"-"`
	ETag         string    `bson:"etag" json:"-"`
	LastModified time.Time `bson:"lastModified" json:"-"`
}

type Problem struct {
	ID          bson.ObjectID `bson:"_id" json:"id"`
	DocID       int64         `bson:"docId" json:"docId"`
	PID         string        `bson:"pid" json:"pid"`
	Title       string        `bson:"title" json:"title"`
	Content     string        `bson:"content" json:"content,omitempty"`
	Tags        []string      `bson:"tag" json:"tags"`
	Difficulty  int           `bson:"difficulty" json:"difficulty,omitempty"`
	NSubmit     int64         `bson:"nSubmit" json:"nSubmit"`
	NAccept     int64         `bson:"nAccept" json:"nAccept"`
	Hidden      bool          `bson:"hidden" json:"hidden"`
	Sort        string        `bson:"sort" json:"sort,omitempty"`
	Owner       int64         `bson:"owner" json:"owner,omitempty"`
	Config      any           `bson:"config" json:"-"`
	Data        []ProblemFile `bson:"data" json:"-"`
	UserStatus  *UserStatus   `bson:"-" json:"userStatus,omitempty"`
	ProblemType string        `bson:"-" json:"problemType,omitempty"`
	TimeLimit   string        `bson:"-" json:"timeLimit,omitempty"`
	MemoryLimit string        `bson:"-" json:"memoryLimit,omitempty"`
}

type UserStatus struct {
	Status int    `json:"status"`
	RID    string `json:"rid"`
}

type ListOptions struct {
	Page       int
	Limit      int
	Query      string
	Tag        string
	Difficulty *int
	Sort       string
	UID        int64
}

type ListResult struct {
	Items []Problem `json:"items"`
	Total int64     `json:"total"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
}

type Repository struct {
	coll    *mongo.Collection
	records *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("document"), records: db.Collection("record")}
}

func (r *Repository) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	opts = normalizeListOptions(opts)
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"hidden":   bson.M{"$ne": true},
	}
	if opts.Query != "" {
		pattern := regexp.QuoteMeta(opts.Query)
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": pattern, "$options": "i"}},
			{"pid": bson.M{"$regex": pattern, "$options": "i"}},
		}
	}
	if opts.Tag != "" {
		filter["tag"] = opts.Tag
	}
	if opts.Difficulty != nil {
		filter["difficulty"] = *opts.Difficulty
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	projection := bson.M{
		"docId": 1, "pid": 1, "title": 1, "tag": 1,
		"difficulty": 1, "nSubmit": 1, "nAccept": 1,
		"hidden": 1, "sort": 1, "owner": 1,
	}
	sort := bson.D{{Key: "sort", Value: 1}, {Key: "docId", Value: 1}}
	if opts.Sort == "recent" {
		sort = bson.D{{Key: "_id", Value: -1}}
	}
	findOpts := options.Find().
		SetProjection(projection).
		SetSort(sort).
		SetSkip(int64((opts.Page - 1) * opts.Limit)).
		SetLimit(int64(opts.Limit))
	cursor, err := r.coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Problem, 0, opts.Limit)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if opts.UID > 0 {
		if err := r.fillUserStatus(ctx, opts.UID, items); err != nil {
			return nil, err
		}
	}
	return &ListResult{Items: items, Total: total, Page: opts.Page, Limit: opts.Limit}, nil
}

func (r *Repository) fillUserStatus(ctx context.Context, uid int64, items []Problem) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.DocID)
	}
	cursor, err := r.records.Find(ctx, bson.M{
		"uid":      uid,
		"domainId": DomainSystem,
		"pid":      bson.M{"$in": ids},
	}, options.Find().
		SetProjection(bson.M{"_id": 1, "pid": 1, "status": 1}).
		SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	var records []struct {
		ID     bson.ObjectID `bson:"_id"`
		PID    int64         `bson:"pid"`
		Status int           `bson:"status"`
	}
	if err := cursor.All(ctx, &records); err != nil {
		return err
	}
	byPID := make(map[int64]UserStatus, len(items))
	for _, record := range records {
		if _, exists := byPID[record.PID]; !exists {
			byPID[record.PID] = UserStatus{Status: record.Status, RID: record.ID.Hex()}
		}
	}
	for i := range items {
		if status, ok := byPID[items[i].DocID]; ok {
			items[i].UserStatus = &status
		}
	}
	return nil
}

func (r *Repository) GetManyByDocIDs(ctx context.Context, ids []int64) ([]Problem, error) {
	if len(ids) == 0 {
		return []Problem{}, nil
	}
	projection := bson.M{
		"docId": 1, "pid": 1, "title": 1, "tag": 1,
		"difficulty": 1, "nSubmit": 1, "nAccept": 1, "hidden": 1, "sort": 1,
	}
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"hidden":   bson.M{"$ne": true},
		"docId":    bson.M{"$in": ids},
	}, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	found := make([]Problem, 0, len(ids))
	if err := cursor.All(ctx, &found); err != nil {
		return nil, err
	}
	byID := make(map[int64]Problem, len(found))
	for _, p := range found {
		byID[p.DocID] = p
	}
	result := make([]Problem, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *Repository) Get(ctx context.Context, identifier string) (*Problem, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrNotFound
	}
	or := []bson.M{{"pid": identifier}}
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		or = append(or, bson.M{"docId": id})
	}
	if oid, err := bson.ObjectIDFromHex(identifier); err == nil {
		or = append(or, bson.M{"_id": oid})
	}
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"hidden":   bson.M{"$ne": true},
		"$or":      or,
	}
	var p Problem
	err := r.coll.FindOne(ctx, filter).Decode(&p)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	applyConfigSummary(&p)
	return &p, nil
}

func applyConfigSummary(p *Problem) {
	var config map[string]any
	switch value := p.Config.(type) {
	case string:
		_ = yaml.Unmarshal([]byte(value), &config)
	case bson.M:
		config = map[string]any(value)
	case bson.D:
		config = make(map[string]any, len(value))
		for _, item := range value {
			config[item.Key] = item.Value
		}
	case map[string]any:
		config = value
	}
	if config == nil {
		return
	}
	p.ProblemType = strings.TrimSpace(fmt.Sprint(config["type"]))
	if p.ProblemType == "<nil>" {
		p.ProblemType = ""
	}
	if value := config["time"]; value != nil {
		p.TimeLimit = strings.TrimSpace(fmt.Sprint(value))
	}
	if value := config["memory"]; value != nil {
		p.MemoryLimit = strings.TrimSpace(fmt.Sprint(value))
	}
}

func normalizeListOptions(opts ListOptions) ListOptions {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.Limit < 1 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	opts.Query = strings.TrimSpace(opts.Query)
	opts.Tag = strings.TrimSpace(opts.Tag)
	opts.Sort = strings.TrimSpace(opts.Sort)
	if opts.Sort != "recent" {
		opts.Sort = "default"
	}
	return opts
}
