package contest

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/syuctacm/syuct-oj/internal/problem"
)

const (
	DocumentTypeContest = 30
	DomainSystem        = "system"
)

var ErrNotFound = fmt.Errorf("contest not found")

type EditInput struct {
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	BeginAt      time.Time  `json:"beginAt"`
	EndAt        time.Time  `json:"endAt"`
	Rule         string     `json:"rule"`
	PIDs         []int64    `json:"pids"`
	Duration     int        `json:"duration"`
	Rated        bool       `json:"rated"`
	PenaltySince *time.Time `json:"penaltySince,omitempty"`
	LockAt       *time.Time `json:"lockAt,omitempty"`
	Unlocked     bool       `json:"unlocked"`
}

type Contest struct {
	ID           bson.ObjectID `bson:"_id" json:"id"`
	DocID        bson.ObjectID `bson:"docId" json:"docId"`
	Title        string        `bson:"title" json:"title"`
	Content      string        `bson:"content" json:"content,omitempty"`
	BeginAt      time.Time     `bson:"beginAt" json:"beginAt"`
	EndAt        time.Time     `bson:"endAt" json:"endAt"`
	Rule         string        `bson:"rule" json:"rule"`
	Owner        int64         `bson:"owner" json:"owner"`
	Attend       int64         `bson:"attend" json:"attend"`
	PIDs         []int64       `bson:"pids" json:"pids"`
	Duration     int           `bson:"duration" json:"duration"`
	Rated        bool          `bson:"rated" json:"rated"`
	PenaltySince *time.Time    `bson:"penaltySince,omitempty" json:"penaltySince,omitempty"`
	LockAt       *time.Time    `bson:"lockAt,omitempty" json:"lockAt,omitempty"`
	Unlocked     bool          `bson:"unlocked,omitempty" json:"unlocked,omitempty"`
}

type ProblemCell struct {
	Solved   bool `json:"solved"`
	Attempts int  `json:"attempts"`
	Time     int  `json:"time"`
}

type ScoreboardRow struct {
	Rank     int                    `json:"rank"`
	UID      int64                  `json:"uid"`
	Solved   int                    `json:"solved"`
	Penalty  int                    `json:"penalty"`
	Problems map[string]ProblemCell `json:"problems"`
}

type Repository struct {
	coll     *mongo.Collection
	records  *mongo.Collection
	problems *problem.Repository
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		coll:     db.Collection("document"),
		records:  db.Collection("record"),
		problems: problem.NewRepository(db),
	}
}

func (r *Repository) Create(ctx context.Context, owner int64, input EditInput) (*Contest, error) {
	if err := r.validateEdit(ctx, input); err != nil {
		return nil, err
	}
	oid := bson.NewObjectID()
	duration := input.Duration
	if duration <= 0 {
		duration = int(input.EndAt.Sub(input.BeginAt).Hours())
	}
	document := bson.M{
		"_id": oid, "docId": oid, "domainId": DomainSystem, "docType": DocumentTypeContest,
		"title": strings.TrimSpace(input.Title), "content": input.Content,
		"beginAt": input.BeginAt, "endAt": input.EndAt, "rule": input.Rule,
		"owner": owner, "attend": int64(0), "pids": input.PIDs, "duration": duration,
		"rated": input.Rated, "unlocked": input.Unlocked,
	}
	if input.PenaltySince != nil {
		document["penaltySince"] = *input.PenaltySince
	}
	if input.LockAt != nil {
		document["lockAt"] = *input.LockAt
	}
	if _, err := r.coll.InsertOne(ctx, document); err != nil {
		return nil, err
	}
	return r.Get(ctx, oid.Hex())
}

func (r *Repository) Update(ctx context.Context, tid string, input EditInput) (*Contest, error) {
	if err := r.validateEdit(ctx, input); err != nil {
		return nil, err
	}
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	duration := input.Duration
	if duration <= 0 {
		duration = int(input.EndAt.Sub(input.BeginAt).Hours())
	}
	set := bson.M{
		"title": strings.TrimSpace(input.Title), "content": input.Content,
		"beginAt": input.BeginAt, "endAt": input.EndAt, "rule": input.Rule,
		"pids": input.PIDs, "duration": duration, "rated": input.Rated,
		"unlocked": input.Unlocked,
	}
	unset := bson.M{}
	if input.PenaltySince != nil {
		set["penaltySince"] = *input.PenaltySince
	} else {
		unset["penaltySince"] = ""
	}
	if input.LockAt != nil {
		set["lockAt"] = *input.LockAt
	} else {
		unset["lockAt"] = ""
	}
	update := bson.M{"$set": set}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	result, err := r.coll.UpdateOne(ctx,
		bson.M{"domainId": DomainSystem, "docType": DocumentTypeContest, "$or": []bson.M{{"docId": oid}, {"_id": oid}}},
		update,
	)
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, tid)
}

func (r *Repository) validateEdit(ctx context.Context, input EditInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if !validRule(input.Rule) {
		return fmt.Errorf("invalid contest rule %q", input.Rule)
	}
	if input.BeginAt.IsZero() || input.EndAt.IsZero() || !input.EndAt.After(input.BeginAt) {
		return fmt.Errorf("end time must be after begin time")
	}
	if input.EndAt.Sub(input.BeginAt) > 30*24*time.Hour {
		return fmt.Errorf("contest duration must not exceed 30 days")
	}
	if input.LockAt != nil && (input.LockAt.Before(input.BeginAt) || input.LockAt.After(input.EndAt)) {
		return fmt.Errorf("lock time must be inside the contest window")
	}
	return r.problems.ValidateDocIDs(ctx, input.PIDs)
}

func validRule(rule string) bool {
	switch rule {
	case "acm", "oi", "ioi", "strictioi", "ledo":
		return true
	default:
		return false
	}
}

func (r *Repository) List(ctx context.Context) ([]Contest, error) {
	return r.ListFiltered(ctx, "", "")
}

func (r *Repository) ListFiltered(ctx context.Context, query, rule string) ([]Contest, error) {
	projection := bson.M{
		"docId": 1, "title": 1, "beginAt": 1, "endAt": 1,
		"rule": 1, "owner": 1, "attend": 1, "pids": 1, "duration": 1, "rated": 1,
	}
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"rule":     bson.M{"$ne": "homework"},
	}
	if query = strings.TrimSpace(query); query != "" {
		filter["title"] = bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"}
	}
	if rule = strings.TrimSpace(rule); rule != "" {
		filter["rule"] = rule
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "beginAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Contest, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) ListHomework(ctx context.Context) ([]Contest, error) {
	return r.ListHomeworkFiltered(ctx, "")
}

func (r *Repository) ListHomeworkFiltered(ctx context.Context, query string) ([]Contest, error) {
	projection := bson.M{
		"docId": 1, "title": 1, "beginAt": 1, "endAt": 1,
		"rule": 1, "owner": 1, "attend": 1, "pids": 1, "duration": 1, "rated": 1, "penaltySince": 1,
	}
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"rule":     "homework",
	}
	if query = strings.TrimSpace(query); query != "" {
		filter["title"] = bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"}
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "endAt", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Contest, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetHomework(ctx context.Context, tid string) (*Contest, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	if item.Rule != "homework" {
		return nil, ErrNotFound
	}
	return item, nil
}

func (r *Repository) Problems(ctx context.Context, tid string) (*Contest, []problem.Problem, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, nil, err
	}
	problems, err := r.problems.GetManyByDocIDs(ctx, item.PIDs)
	if err != nil {
		return nil, nil, err
	}
	return item, problems, nil
}

func (r *Repository) Scoreboard(ctx context.Context, tid string) (*Contest, []ScoreboardRow, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, nil, err
	}
	allowed := make(map[int64]struct{}, len(item.PIDs))
	for _, pid := range item.PIDs {
		allowed[pid] = struct{}{}
	}
	cursor, err := r.records.Find(ctx, bson.M{"contest": item.DocID, "domainId": DomainSystem},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, nil, err
	}
	defer cursor.Close(ctx)
	type userState struct {
		uid     int64
		solved  int
		penalty int
		cells   map[string]ProblemCell
		wrong   map[int64]int
	}
	users := make(map[int64]*userState)
	for cursor.Next(ctx) {
		var record bson.M
		if err := cursor.Decode(&record); err != nil {
			return nil, nil, err
		}
		uid, ok := toInt64(record["uid"])
		if !ok {
			continue
		}
		pid, ok := toInt64(record["pid"])
		if !ok {
			continue
		}
		if _, ok := allowed[pid]; !ok {
			continue
		}
		status, _ := toInt64(record["status"])
		state := users[uid]
		if state == nil {
			state = &userState{uid: uid, cells: make(map[string]ProblemCell), wrong: make(map[int64]int)}
			users[uid] = state
		}
		key := strconv.FormatInt(pid, 10)
		cell := state.cells[key]
		if cell.Solved {
			continue
		}
		rid, _ := record["_id"].(bson.ObjectID)
		if item.LockAt != nil && !item.Unlocked && rid.Timestamp().After(*item.LockAt) {
			continue
		}
		if status == 1 {
			minutes := int(rid.Timestamp().Sub(item.BeginAt).Minutes())
			if minutes < 0 {
				minutes = 0
			}
			cell.Solved = true
			cell.Attempts = state.wrong[pid]
			cell.Time = minutes
			state.cells[key] = cell
			state.solved++
			state.penalty += minutes + 20*state.wrong[pid]
			continue
		}
		if status >= 2 && status <= 8 {
			state.wrong[pid]++
			cell.Attempts = state.wrong[pid]
			state.cells[key] = cell
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, nil, err
	}
	rows := make([]ScoreboardRow, 0, len(users))
	for _, state := range users {
		rows = append(rows, ScoreboardRow{
			UID: state.uid, Solved: state.solved, Penalty: state.penalty, Problems: state.cells,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Solved != rows[j].Solved {
			return rows[i].Solved > rows[j].Solved
		}
		if rows[i].Penalty != rows[j].Penalty {
			return rows[i].Penalty < rows[j].Penalty
		}
		return rows[i].UID < rows[j].UID
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return item, rows, nil
}

func (r *Repository) Get(ctx context.Context, tid string) (*Contest, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	var contest Contest
	err = r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}).Decode(&contest)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &contest, nil
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	}
	return 0, false
}
