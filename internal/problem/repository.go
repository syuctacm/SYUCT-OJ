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

	documentTypeDiscussion      = 21
	documentTypeDiscussionReply = 22
)

var (
	ErrNotFound     = fmt.Errorf("problem not found")
	ErrDuplicatePID = fmt.Errorf("problem pid already exists")
)

type ProblemFile struct {
	ID           string    `bson:"_id" json:"-"`
	Name         string    `bson:"name" json:"-"`
	Size         int64     `bson:"size" json:"-"`
	ETag         string    `bson:"etag" json:"-"`
	LastModified time.Time `bson:"lastModified" json:"-"`
}

type Problem struct {
	ID              bson.ObjectID `bson:"_id" json:"id"`
	DocID           int64         `bson:"docId" json:"docId"`
	PID             string        `bson:"pid" json:"pid"`
	Title           string        `bson:"title" json:"title"`
	Content         string        `bson:"content" json:"content,omitempty"`
	Html            bool          `bson:"html" json:"html,omitempty"`
	Tags            []string      `bson:"tag" json:"tags"`
	Difficulty      int           `bson:"difficulty" json:"difficulty,omitempty"`
	NSubmit         int64         `bson:"nSubmit" json:"nSubmit"`
	NAccept         int64         `bson:"nAccept" json:"nAccept"`
	Hidden          bool          `bson:"hidden" json:"hidden"`
	Sort            string        `bson:"sort" json:"sort,omitempty"`
	Owner           int64         `bson:"owner" json:"owner,omitempty"`
	Config          any           `bson:"config" json:"-"`
	Data            []ProblemFile `bson:"data" json:"-"`
	AdditionalFiles []ProblemFile `bson:"additional_file" json:"-"`
	UserStatus      *UserStatus   `bson:"-" json:"userStatus,omitempty"`
	ProblemType     string        `bson:"-" json:"problemType,omitempty"`
	TimeLimit       string        `bson:"-" json:"timeLimit,omitempty"`
	MemoryLimit     string        `bson:"-" json:"memoryLimit,omitempty"`
	TimeMin         string        `bson:"-" json:"timeMin,omitempty"`
	TimeMax         string        `bson:"-" json:"timeMax,omitempty"`
	MemoryMin       string        `bson:"-" json:"memoryMin,omitempty"`
	MemoryMax       string        `bson:"-" json:"memoryMax,omitempty"`
	Starred         bool          `bson:"-" json:"starred,omitempty"`
	Langs           []string      `bson:"-" json:"langs,omitempty"`
}

type UserStatus struct {
	Status int    `json:"status"`
	RID    string `json:"rid"`
}

type EditInput struct {
	PID        string
	Title      string
	Content    string
	Tags       []string
	Difficulty int
	Hidden     bool
	Config     string
	// Html marks the content as pre-rendered HTML; the FPS importer sets it,
	// the edit form never does.
	Html bool
}

type ListOptions struct {
	Page       int
	Limit      int
	Query      string
	Tag        string
	Difficulty *int
	Sort       string
	UID        int64
	// IncludeHidden lists hidden problems too; only administrators get it.
	IncludeHidden bool
}

type ListResult struct {
	Items []Problem `json:"items"`
	Total int64     `json:"total"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
}

type TagStat struct {
	Tag      string `json:"tag"`
	Count    int64  `json:"count"`
	IsSource bool   `json:"isSource"`
}

type TagRankingItem struct {
	Rank        int    `json:"rank"`
	UID         int64  `json:"uid"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Solved      int64  `json:"solved"`
	Submissions int64  `json:"submissions"`
}

type TagRanking struct {
	Tag          string           `json:"tag"`
	ProblemCount int64            `json:"problemCount"`
	TotalUsers   int64            `json:"totalUsers"`
	MatchedCount int64            `json:"matchedCount"`
	Page         int              `json:"page"`
	Limit        int              `json:"limit"`
	Items        []TagRankingItem `json:"items"`
}

type Repository struct {
	coll        *mongo.Collection
	records     *mongo.Collection
	recordStats *mongo.Collection
	tasks       *mongo.Collection
	users       *mongo.Collection
	counters    *mongo.Collection
	statuses    *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		coll: db.Collection("document"), records: db.Collection("record"), users: db.Collection("user"),
		recordStats: db.Collection("record.stat"), tasks: db.Collection("task"),
		counters: db.Collection("counter"), statuses: db.Collection("document.status"),
	}
}

func (r *Repository) Create(ctx context.Context, owner int64, input EditInput) (*Problem, error) {
	pid := strings.TrimSpace(input.PID)
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if pid != "" {
		count, err := r.coll.CountDocuments(ctx, bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "pid": pid})
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, ErrDuplicatePID
		}
	}
	docID, err := r.nextDocID(ctx)
	if err != nil {
		return nil, err
	}
	if pid == "" {
		// Hydro leaves the pid empty and falls back to the docId; the rewrite
		// keeps a readable pid of the same shape as the upstream `sort` value.
		pid = fmt.Sprintf("P%d", docID)
	}
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	document := bson.M{
		"_id": bson.NewObjectID(), "domainId": DomainSystem, "docType": DocumentTypeProblem,
		"docId": docID, "pid": pid, "title": title, "content": input.Content, "html": input.Html,
		"owner": owner, "tag": tags, "difficulty": input.Difficulty, "hidden": input.Hidden,
		"nSubmit": int64(0), "nAccept": int64(0), "config": input.Config,
		"data": []ProblemFile{}, "additional_file": []ProblemFile{},
	}
	if _, err := r.coll.InsertOne(ctx, document); err != nil {
		return nil, err
	}
	return r.GetForEditor(ctx, strconv.FormatInt(docID, 10))
}

func (r *Repository) Update(ctx context.Context, docID int64, input EditInput) (*Problem, error) {
	pid := strings.TrimSpace(input.PID)
	title := strings.TrimSpace(input.Title)
	if pid == "" || title == "" {
		return nil, fmt.Errorf("pid and title are required")
	}
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeProblem,
		"pid": pid, "docId": bson.M{"$ne": docID},
	})
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrDuplicatePID
	}
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	result, err := r.coll.UpdateOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID},
		bson.M{"$set": bson.M{
			"pid": pid, "title": title, "content": input.Content, "tag": tags,
			"difficulty": input.Difficulty, "hidden": input.Hidden, "config": input.Config,
		}},
	)
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, ErrNotFound
	}
	return r.GetForEditor(ctx, strconv.FormatInt(docID, 10))
}

func (r *Repository) nextDocID(ctx context.Context) (int64, error) {
	var latest Problem
	err := r.coll.FindOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem},
		options.FindOne().SetSort(bson.D{{Key: "docId", Value: -1}}).SetProjection(bson.M{"docId": 1}),
	).Decode(&latest)
	if err != nil && err != mongo.ErrNoDocuments {
		return 0, err
	}
	if _, err := r.counters.UpdateOne(ctx,
		bson.M{"_id": "problem"},
		bson.M{"$max": bson.M{"seq": latest.DocID}},
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		return 0, err
	}
	var counter struct {
		Seq int64 `bson:"seq"`
	}
	if err := r.counters.FindOneAndUpdate(ctx,
		bson.M{"_id": "problem"},
		bson.M{"$inc": bson.M{"seq": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&counter); err != nil {
		return 0, err
	}
	return counter.Seq, nil
}

func (r *Repository) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	opts = normalizeListOptions(opts)
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
	}
	if !opts.IncludeHidden {
		filter["hidden"] = bson.M{"$ne": true}
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

// Delete removes a problem together with the data Hydro deletes through its
// problem/delete hooks: statuses, stored files metadata, records, judge tasks,
// solutions and the problem's discussions. Physical files are removed by the
// handler through the file store.
func (r *Repository) Delete(ctx context.Context, docID int64) error {
	result, err := r.coll.DeleteOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	_, _ = r.statuses.DeleteMany(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID})
	_, _ = r.records.DeleteMany(ctx, bson.M{"domainId": DomainSystem, "pid": docID})
	_, _ = r.recordStats.DeleteMany(ctx, bson.M{"domainId": DomainSystem, "pid": docID})
	_, _ = r.tasks.DeleteMany(ctx, bson.M{"domainId": DomainSystem, "pid": docID})
	_, _ = r.coll.DeleteMany(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeProblemSolution,
		"parentType": DocumentTypeProblem, "parentId": docID,
	})
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem, "docType": documentTypeDiscussion,
		"parentType": DocumentTypeProblem, "parentId": docID,
	}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err == nil {
		var discussions []struct {
			ID bson.ObjectID `bson:"_id"`
		}
		if err := cursor.All(ctx, &discussions); err == nil && len(discussions) > 0 {
			ids := make([]bson.ObjectID, 0, len(discussions))
			for _, item := range discussions {
				ids = append(ids, item.ID)
			}
			_, _ = r.coll.DeleteMany(ctx, bson.M{
				"domainId": DomainSystem, "docType": documentTypeDiscussionReply,
				"parentType": documentTypeDiscussion, "parentId": bson.M{"$in": ids},
			})
			_, _ = r.coll.DeleteMany(ctx, bson.M{
				"domainId": DomainSystem, "docType": documentTypeDiscussion,
				"_id": bson.M{"$in": ids},
			})
		}
		_ = cursor.Close(ctx)
	}
	return nil
}

// IncStats adjusts the submission counters shown on the problem list and the
// problem sidebar. The accept counter is clamped at zero so a rejudge cannot
// drive it negative.
func (r *Repository) IncStats(ctx context.Context, docID int64, submitDelta, acceptDelta int64) error {
	set := bson.M{}
	if submitDelta != 0 {
		set["nSubmit"] = bson.M{"$add": []any{
			bson.M{"$ifNull": []any{"$nSubmit", int64(0)}},
			submitDelta,
		}}
	}
	if acceptDelta != 0 {
		set["nAccept"] = bson.M{"$max": []any{int64(0), bson.M{"$add": []any{
			bson.M{"$ifNull": []any{"$nAccept", int64(0)}},
			acceptDelta,
		}}}}
	}
	if len(set) == 0 {
		return nil
	}
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID},
		mongo.Pipeline{bson.D{{Key: "$set", Value: set}}})
	return err
}

// Star stores the per-user star flag on the problem status document, the same
// place Hydro keeps `problem.setStar`.
func (r *Repository) Star(ctx context.Context, docID int64, uid int64, star bool) error {
	if uid <= 0 {
		return fmt.Errorf("invalid user id")
	}
	_, err := r.statuses.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"docId":    docID,
		"uid":      uid,
	}, bson.M{"$set": bson.M{"star": star}}, options.UpdateOne().SetUpsert(true))
	return err
}

// Starred returns the problems a user starred, newest first.
func (r *Repository) Starred(ctx context.Context, uid int64, limit int) ([]Problem, error) {
	if uid <= 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	cursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"uid":      uid,
		"star":     true,
	}, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var statuses []struct {
		DocID int64 `bson:"docId"`
	}
	if err := cursor.All(ctx, &statuses); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(statuses))
	for _, item := range statuses {
		ids = append(ids, item.DocID)
	}
	if len(ids) == 0 {
		return []Problem{}, nil
	}
	items, err := r.GetManyByDocIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]Problem, len(items))
	for _, item := range items {
		byID[item.DocID] = item
	}
	ordered := make([]Problem, 0, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok && !item.Hidden {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
}

// StarredIDs reports which of the given problems the user starred.
func (r *Repository) StarredIDs(ctx context.Context, uid int64, docIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(docIDs))
	if uid <= 0 || len(docIDs) == 0 {
		return result, nil
	}
	cursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"uid":      uid,
		"docId":    bson.M{"$in": docIDs},
		"star":     true,
	}, options.Find().SetProjection(bson.M{"docId": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var status struct {
			DocID int64 `bson:"docId"`
		}
		if err := cursor.Decode(&status); err != nil {
			return nil, err
		}
		result[status.DocID] = true
	}
	return result, cursor.Err()
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

func (r *Repository) UpdateConfig(ctx context.Context, docID int64, config string) error {
	result, err := r.coll.UpdateOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID},
		bson.M{"$set": bson.M{"config": config}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateContent rewrites the statement, used by the FPS importer after inline
// images have been moved into the problem's additional files.
func (r *Repository) UpdateContent(ctx context.Context, docID int64, content string) error {
	result, err := r.coll.UpdateOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID},
		bson.M{"$set": bson.M{"content": content}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpsertFile(ctx context.Context, docID int64, kind string, file ProblemFile) error {
	field, err := problemFileField(kind)
	if err != nil {
		return err
	}
	filter := bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID}
	if _, err := r.coll.UpdateOne(ctx, filter, bson.M{"$pull": bson.M{field: bson.M{"name": file.Name}}}); err != nil {
		return err
	}
	result, err := r.coll.UpdateOne(ctx, filter, bson.M{"$push": bson.M{field: file}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteFiles(ctx context.Context, docID int64, kind string, names []string) error {
	field, err := problemFileField(kind)
	if err != nil {
		return err
	}
	result, err := r.coll.UpdateOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": docID},
		bson.M{"$pull": bson.M{field: bson.M{"name": bson.M{"$in": names}}}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func problemFileField(kind string) (string, error) {
	switch kind {
	case "testdata":
		return "data", nil
	case "additional":
		return "additional_file", nil
	default:
		return "", fmt.Errorf("invalid problem file kind %q", kind)
	}
}

func (r *Repository) GetManyByDocIDs(ctx context.Context, ids []int64) ([]Problem, error) {
	if len(ids) == 0 {
		return []Problem{}, nil
	}
	projection := bson.M{
		"docId": 1, "pid": 1, "title": 1, "tag": 1,
		"difficulty": 1, "nSubmit": 1, "nAccept": 1, "hidden": 1, "sort": 1, "config": 1,
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
	for i := range found {
		applyConfigSummary(&found[i])
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

func (r *Repository) ValidateDocIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one problem is required")
	}
	unique := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("invalid problem docId %d", id)
		}
		unique[id] = struct{}{}
	}
	values := make([]int64, 0, len(unique))
	for id := range unique {
		values = append(values, id)
	}
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeProblem,
		"docId": bson.M{"$in": values},
	})
	if err != nil {
		return err
	}
	if count != int64(len(values)) {
		return fmt.Errorf("one or more problems do not exist")
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, identifier string) (*Problem, error) {
	return r.get(ctx, identifier, false)
}

func (r *Repository) GetForEditor(ctx context.Context, identifier string) (*Problem, error) {
	return r.get(ctx, identifier, true)
}

func (r *Repository) get(ctx context.Context, identifier string, includeHidden bool) (*Problem, error) {
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
		"$or":      or,
	}
	if !includeHidden {
		filter["hidden"] = bson.M{"$ne": true}
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
	configString := func(keys ...string) string {
		for _, key := range keys {
			if value := config[key]; value != nil {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
		return ""
	}
	stripTimeUnit := func(value string) string {
		if strings.HasSuffix(strings.ToLower(value), "ms") {
			return value[:len(value)-2]
		}
		return value
	}
	stripMemoryUnit := func(value string) string {
		return strings.TrimRightFunc(value, func(r rune) bool {
			return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		})
	}
	if value := config["langs"]; value != nil {
		switch list := value.(type) {
		case []any:
			for _, item := range list {
				if name := strings.TrimSpace(fmt.Sprint(item)); name != "" && name != "<nil>" {
					p.Langs = append(p.Langs, name)
				}
			}
		case []string:
			p.Langs = append(p.Langs, list...)
		}
	}
	p.TimeMin = stripTimeUnit(configString("timeMin", "time"))
	p.TimeMax = stripTimeUnit(configString("timeMax", "time"))
	p.MemoryMin = stripMemoryUnit(configString("memoryMin", "memory"))
	p.MemoryMax = stripMemoryUnit(configString("memoryMax", "memory"))
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

var sourceTags = map[string]bool{
	"SYUCT": true, "牛客竞赛": true, "校赛": true, "第十三届校赛": true, "四省赛": true,
}

func (r *Repository) TagStats(ctx context.Context) ([]TagStat, error) {
	cursor, err := r.coll.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"domainId": DomainSystem, "docType": DocumentTypeProblem, "hidden": bson.M{"$ne": true}}}},
		{{Key: "$unwind", Value: "$tag"}},
		{{Key: "$group", Value: bson.M{"_id": "$tag", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var raw []struct {
		Tag   string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	algorithmTags := make([]TagStat, 0, len(raw))
	source := make([]TagStat, 0)
	for _, item := range raw {
		tag := strings.TrimSpace(item.Tag)
		if tag == "" {
			continue
		}
		value := TagStat{Tag: tag, Count: item.Count, IsSource: sourceTags[tag]}
		if value.IsSource {
			source = append(source, value)
		} else {
			algorithmTags = append(algorithmTags, value)
		}
	}
	return append(algorithmTags, source...), nil
}

func (r *Repository) TagRanking(ctx context.Context, tag string, page, limit int, query string) (*TagRanking, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	tag = strings.TrimSpace(tag)
	query = strings.TrimSpace(query)
	result := &TagRanking{Tag: tag, Page: page, Limit: limit, Items: []TagRankingItem{}}
	problemCursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeProblem, "tag": tag, "hidden": bson.M{"$ne": true},
	}, options.Find().SetProjection(bson.M{"docId": 1}))
	if err != nil {
		return nil, err
	}
	var problemDocs []struct {
		DocID int64 `bson:"docId"`
	}
	if err := problemCursor.All(ctx, &problemDocs); err != nil {
		problemCursor.Close(ctx)
		return nil, err
	}
	problemCursor.Close(ctx)
	docIDs := make([]int64, 0, len(problemDocs))
	for _, item := range problemDocs {
		docIDs = append(docIDs, item.DocID)
	}
	result.ProblemCount = int64(len(docIDs))
	if len(docIDs) == 0 {
		return result, nil
	}
	recordCursor, err := r.records.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"domainId": DomainSystem, "pid": bson.M{"$in": docIDs}, "status": 1}}},
		{{Key: "$group", Value: bson.M{"_id": "$uid", "solved": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.D{{Key: "solved", Value: -1}, {Key: "_id", Value: 1}}}},
	})
	if err != nil {
		return nil, err
	}
	var solved []struct {
		UID    int64 `bson:"_id"`
		Solved int64 `bson:"solved"`
	}
	if err := recordCursor.All(ctx, &solved); err != nil {
		recordCursor.Close(ctx)
		return nil, err
	}
	recordCursor.Close(ctx)
	result.TotalUsers = int64(len(solved))
	ranked := make([]TagRankingItem, 0, len(solved))
	for index, item := range solved {
		ranked = append(ranked, TagRankingItem{Rank: index + 1, UID: item.UID, Solved: item.Solved})
	}
	if query != "" {
		uidSet, err := r.matchUsers(ctx, query)
		if err != nil {
			return nil, err
		}
		filtered := make([]TagRankingItem, 0, len(ranked))
		for _, item := range ranked {
			if uidSet[item.UID] {
				filtered = append(filtered, item)
			}
		}
		ranked = filtered
	}
	result.MatchedCount = int64(len(ranked))
	start := (page - 1) * limit
	if start >= len(ranked) {
		return result, nil
	}
	end := start + limit
	if end > len(ranked) {
		end = len(ranked)
	}
	pageItems := ranked[start:end]
	uids := make([]int64, 0, len(pageItems))
	for _, item := range pageItems {
		uids = append(uids, item.UID)
	}
	var users []struct {
		ID          int64  `bson:"_id"`
		Username    string `bson:"uname"`
		DisplayName string `bson:"displayName"`
	}
	userCursor, err := r.users.Find(ctx, bson.M{"_id": bson.M{"$in": uids}}, options.Find().SetProjection(bson.M{"uname": 1, "displayName": 1}))
	if err != nil {
		return nil, err
	}
	if err := userCursor.All(ctx, &users); err != nil {
		userCursor.Close(ctx)
		return nil, err
	}
	userCursor.Close(ctx)
	userMap := make(map[int64]TagRankingItem, len(users))
	for _, item := range users {
		userMap[item.ID] = TagRankingItem{UID: item.ID, Username: item.Username, DisplayName: item.DisplayName}
	}
	var submissions []struct {
		UID   int64 `bson:"_id"`
		Count int64 `bson:"count"`
	}
	submissionCursor, err := r.records.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"domainId": DomainSystem, "pid": bson.M{"$in": docIDs}, "uid": bson.M{"$in": uids}}}},
		{{Key: "$group", Value: bson.M{"_id": "$uid", "count": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	if err := submissionCursor.All(ctx, &submissions); err != nil {
		submissionCursor.Close(ctx)
		return nil, err
	}
	submissionCursor.Close(ctx)
	submissionMap := make(map[int64]int64, len(submissions))
	for _, item := range submissions {
		submissionMap[item.UID] = item.Count
	}
	for _, item := range pageItems {
		if user, ok := userMap[item.UID]; ok {
			item.Username = user.Username
			item.DisplayName = user.DisplayName
		}
		item.Submissions = submissionMap[item.UID]
		if item.Submissions < item.Solved {
			item.Submissions = item.Solved
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (r *Repository) matchUsers(ctx context.Context, query string) (map[int64]bool, error) {
	result := map[int64]bool{}
	or := []bson.M{{"uname": bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"}}}
	if uid, err := strconv.ParseInt(query, 10, 64); err == nil {
		or = append(or, bson.M{"_id": uid})
	}
	cursor, err := r.users.Find(ctx, bson.M{"$or": or}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var users []struct {
		ID int64 `bson:"_id"`
	}
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	for _, item := range users {
		result[item.ID] = true
	}
	return result, nil
}
