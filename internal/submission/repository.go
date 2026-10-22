package submission

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	Time          float64        `bson:"time" json:"time"`
	Memory        int            `bson:"memory" json:"memory"`
	JudgeTexts    []any          `bson:"judgeTexts" json:"judgeTexts,omitempty"`
	CompilerTexts []string       `bson:"compilerTexts" json:"compilerTexts,omitempty"`
	TestCases     []any          `bson:"testCases" json:"testCases,omitempty"`
	Judger        *int64         `bson:"judger" json:"judger,omitempty"`
	JudgeAt       *time.Time     `bson:"judgeAt" json:"judgeAt,omitempty"`
	Rejudged      bool           `bson:"rejudged" json:"rejudged"`
	Source        string         `bson:"source" json:"source,omitempty"`
	Contest       *bson.ObjectID `bson:"contest,omitempty" json:"contest,omitempty"`
	Length        int            `bson:"length" json:"length,omitempty"`
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

// AcceptedHook is called after a submission is accepted; the contest package
// uses it to create balloons.
type AcceptedHook func(ctx context.Context, contestID bson.ObjectID, uid, pid int64)

type Repository struct {
	records      *mongo.Collection
	tasks        *mongo.Collection
	users        *mongo.Collection
	problems     *problem.Repository
	acceptedHook AcceptedHook
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

	task, err := buildJudgeTask(pdoc, rid, uid, lang, code, contestObjectID)
	if err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	if err := r.problems.IncStats(ctx, pdoc.DocID, 1, 0); err != nil {
		slog.Warn("unable to update problem submission counter", "pid", pdoc.DocID, "error", err)
	}
	return r.Get(ctx, rid.Hex())
}

// buildJudgeTask assembles the hydrojudge task shared by submit and rejudge.
func buildJudgeTask(pdoc *problem.Problem, rid bson.ObjectID, uid int64, lang, code string, contest *bson.ObjectID) (bson.M, error) {
	config, err := configMap(pdoc.Config)
	if err != nil {
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
	if contest != nil {
		task["contest"] = *contest
	}
	return task, nil
}

// RejudgeProblem resets every record of a problem and queues a fresh judge task
// for each one, mirroring Hydro's record.rejudge.
func (r *Repository) RejudgeProblem(ctx context.Context, pid string) (int64, error) {
	// Administrators may rejudge hidden problems, so resolve through the editor
	// lookup that includes them.
	pdoc, err := r.problems.GetForEditor(ctx, pid)
	if err != nil {
		return 0, err
	}
	cursor, err := r.records.Find(ctx,
		bson.M{"domainId": "system", "pid": pdoc.DocID},
		options.Find().SetProjection(bson.M{"_id": 1, "uid": 1, "lang": 1, "code": 1, "status": 1, "contest": 1}))
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var records []struct {
		ID      bson.ObjectID  `bson:"_id"`
		UID     int64          `bson:"uid"`
		Lang    string         `bson:"lang"`
		Code    string         `bson:"code"`
		Status  int            `bson:"status"`
		Contest *bson.ObjectID `bson:"contest"`
	}
	if err := cursor.All(ctx, &records); err != nil {
		return 0, err
	}
	var accepted int64
	for _, record := range records {
		if record.Status == StatusAccepted {
			accepted++
		}
	}
	if _, err := r.records.UpdateMany(ctx,
		bson.M{"domainId": "system", "pid": pdoc.DocID},
		bson.M{"$set": bson.M{
			"status": StatusWaiting, "score": 0, "time": 0, "memory": 0,
			"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{},
			"judger": nil, "judgeAt": nil, "rejudged": true, "progress": 0,
		}}); err != nil {
		return 0, err
	}
	for _, record := range records {
		task, err := buildJudgeTask(pdoc, record.ID, record.UID, record.Lang, record.Code, record.Contest)
		if err != nil {
			return 0, err
		}
		_, _ = r.tasks.DeleteMany(ctx, bson.M{"rid": record.ID})
		if _, err := r.tasks.InsertOne(ctx, task); err != nil {
			return 0, err
		}
	}
	if accepted > 0 {
		if err := r.problems.IncStats(ctx, pdoc.DocID, 0, -accepted); err != nil {
			slog.Warn("unable to adjust problem accept counter after rejudge", "pid", pdoc.DocID, "error", err)
		}
	}
	return int64(len(records)), nil
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

// SetAcceptedHook registers the callback fired for accepted submissions.
func (r *Repository) SetAcceptedHook(hook AcceptedHook) {
	r.acceptedHook = hook
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
	var current struct {
		PID    int64 `bson:"pid"`
		Status int   `bson:"status"`
	}
	if err := r.records.FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"pid": 1, "status": 1})).Decode(&current); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
	}
	set := bson.M{"judgeAt": time.Now().UTC()}
	for _, key := range []string{"status", "score", "time", "memory", "progress", "judger", "subtasks"} {
		if value, ok := payload[key]; ok {
			set[key] = normalizeJudgeField(key, value)
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
	if _, err = r.records.UpdateOne(ctx, bson.M{"_id": oid}, update); err != nil {
		return err
	}
	if status, ok := set["status"]; ok && current.PID > 0 {
		wasAccepted := current.Status == StatusAccepted
		nowAccepted := isAccepted(status)
		delta := int64(0)
		if !wasAccepted && nowAccepted {
			delta = 1
		} else if wasAccepted && !nowAccepted {
			delta = -1
		}
		if delta != 0 {
			if err := r.problems.IncStats(ctx, current.PID, 0, delta); err != nil {
				slog.Warn("unable to update problem accept counter", "pid", current.PID, "error", err)
			}
		}
	}
	if hook := r.acceptedHook; hook != nil && isAccepted(set["status"]) {
		var record struct {
			UID     int64          `bson:"uid"`
			PID     int64          `bson:"pid"`
			Contest *bson.ObjectID `bson:"contest"`
		}
		if err := r.records.FindOne(ctx, bson.M{"_id": oid}, options.FindOne().
			SetProjection(bson.M{"uid": 1, "pid": 1, "contest": 1})).Decode(&record); err == nil {
			if record.Contest != nil && record.UID > 0 && record.PID > 0 {
				hook(ctx, *record.Contest, record.UID, record.PID)
			}
		}
	}
	return nil
}

func isAccepted(value any) bool {
	switch number := value.(type) {
	case int:
		return number == 1
	case int32:
		return number == 1
	case int64:
		return number == 1
	case float64:
		return number == 1
	default:
		return false
	}
}

func normalizeJudgeField(key string, value any) any {
	switch key {
	case "status", "score", "memory":
		switch number := value.(type) {
		case float32:
			return int(number)
		case float64:
			return int(number)
		case int32:
			return int(number)
		case int64:
			return int(number)
		}
	case "time", "progress":
		switch number := value.(type) {
		case float32:
			return float64(number)
		case int:
			return float64(number)
		case int32:
			return float64(number)
		case int64:
			return float64(number)
		}
	case "judger":
		switch number := value.(type) {
		case float32:
			return int64(number)
		case float64:
			return int64(number)
		case int:
			return int64(number)
		case int32:
			return int64(number)
		}
	}
	return value
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

// StatisticsSorts are the sort keys the problem statistics page accepts,
// mirroring Hydro's record.STAT_QUERY.
var StatisticsSorts = []string{"time", "memory", "length", "date"}

// StatisticsFilter selects one page of a problem's submissions plus the
// aggregated per-status and per-score counters.
type StatisticsFilter struct {
	DocID     int64
	Lang      string
	Sort      string
	Direction int
	Page      int
	Limit     int
}

type StatisticsResult struct {
	Items []Record
	Total int64
	Stats map[string]int64
}

func normalizeStatisticsFilter(filter StatisticsFilter) StatisticsFilter {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}
	valid := false
	for _, sort := range StatisticsSorts {
		if filter.Sort == sort {
			valid = true
			break
		}
	}
	if !valid {
		filter.Sort = "time"
	}
	if filter.Direction != -1 {
		filter.Direction = 1
	}
	return filter
}

// Statistics returns the submission list of one problem, sorted and filtered
// like the upstream statistics page, together with the status/score counters
// the charts need.
func (r *Repository) Statistics(ctx context.Context, filter StatisticsFilter) (*StatisticsResult, error) {
	filter = normalizeStatisticsFilter(filter)
	match := bson.M{"domainId": "system", "pid": filter.DocID}
	if filter.Lang != "" {
		match["lang"] = filter.Lang
	}
	sortKey := filter.Sort
	sort := bson.D{{Key: sortKey, Value: filter.Direction}, {Key: "_id", Value: -1}}
	if sortKey == "date" {
		sort = bson.D{{Key: "_id", Value: filter.Direction}}
	}
	pipeline := []bson.M{
		{"$match": match},
		// `length` is the code size shown in the table; the code itself stays
		// out of the payload.
		{"$addFields": bson.M{"length": bson.M{"$strLenCP": bson.M{"$ifNull": []any{"$code", ""}}}}},
		{"$project": bson.M{"code": 0, "judgeTexts": 0, "compilerTexts": 0, "testCases": 0}},
		{"$sort": sort},
		{"$facet": bson.M{
			"items": []any{
				bson.M{"$skip": int64((filter.Page - 1) * filter.Limit)},
				bson.M{"$limit": int64(filter.Limit)},
			},
			"total": []any{bson.M{"$count": "value"}},
		}},
	}
	cursor, err := r.records.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var page struct {
		Items []Record `bson:"items"`
		Total []struct {
			Value int64 `bson:"value"`
		} `bson:"total"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&page); err != nil {
			return nil, err
		}
	}
	result := &StatisticsResult{Items: page.Items, Stats: map[string]int64{}}
	if len(page.Total) > 0 {
		result.Total = page.Total[0].Value
	}
	for i := range result.Items {
		result.Items[i].CreatedAt = result.Items[i].ID.Timestamp()
	}
	if err := r.enrich(ctx, result.Items); err != nil {
		return nil, err
	}
	stats, err := r.problemStatistics(ctx, filter.DocID)
	if err != nil {
		return nil, err
	}
	result.Stats = stats
	return result, nil
}

// problemStatistics aggregates the status counters and the 0..100 score
// histogram, the same numbers Hydro's problemStat script stores on the problem.
func (r *Repository) problemStatistics(ctx context.Context, docID int64) (map[string]int64, error) {
	group := bson.M{"_id": nil, "total": bson.M{"$sum": 1}}
	for name, status := range map[string]int{
		"AC": StatusAccepted, "WA": 2, "TLE": 3, "MLE": 4,
		"RE": 6, "CE": 7, "SE": 8, "IGN": 9,
	} {
		group[name] = bson.M{"$sum": bson.M{"$cond": []any{
			bson.M{"$eq": []any{"$status", status}}, 1, 0,
		}}}
	}
	for score := 0; score <= 100; score++ {
		group[fmt.Sprintf("s%d", score)] = bson.M{"$sum": bson.M{"$cond": []any{
			bson.M{"$and": []any{
				bson.M{"$gte": []any{"$score", score}},
				bson.M{"$lt": []any{"$score", score + 1}},
			}}, 1, 0,
		}}}
	}
	cursor, err := r.records.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"domainId": "system", "pid": docID}},
		{"$group": group},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	stats := map[string]int64{}
	if !cursor.Next(ctx) {
		return stats, cursor.Err()
	}
	var raw bson.M
	if err := cursor.Decode(&raw); err != nil {
		return nil, err
	}
	for key, value := range raw {
		if key == "_id" {
			continue
		}
		switch current := value.(type) {
		case int32:
			stats[key] = int64(current)
		case int64:
			stats[key] = current
		case float64:
			stats[key] = int64(current)
		}
	}
	return stats, cursor.Err()
}
