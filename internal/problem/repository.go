package problem

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DocumentTypeProblem = 10
	DomainSystem        = "system"
)

var ErrNotFound = fmt.Errorf("problem not found")

type Problem struct {
	ID         bson.ObjectID `bson:"_id" json:"id"`
	DocID      int64         `bson:"docId" json:"docId"`
	PID        string        `bson:"pid" json:"pid"`
	Title      string        `bson:"title" json:"title"`
	Content    string        `bson:"content" json:"content,omitempty"`
	Tags       []string      `bson:"tag" json:"tags"`
	Difficulty int           `bson:"difficulty" json:"difficulty,omitempty"`
	NSubmit    int64         `bson:"nSubmit" json:"nSubmit"`
	NAccept    int64         `bson:"nAccept" json:"nAccept"`
	Hidden     bool          `bson:"hidden" json:"hidden"`
	Sort       string        `bson:"sort" json:"sort,omitempty"`
	Owner      int64         `bson:"owner" json:"owner,omitempty"`
}

type ListOptions struct {
	Page       int
	Limit      int
	Query      string
	Tag        string
	Difficulty *int
}

type ListResult struct {
	Items []Problem `json:"items"`
	Total int64     `json:"total"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
}

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("document")}
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
	findOpts := options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "sort", Value: 1}, {Key: "docId", Value: 1}}).
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
	return &ListResult{Items: items, Total: total, Page: opts.Page, Limit: opts.Limit}, nil
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
	return &p, nil
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
	return opts
}
