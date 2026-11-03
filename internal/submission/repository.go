package submission

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

var ErrNotFound = errors.New("record not found")

const (
	StatusWaiting          = 0
	StatusAccepted         = 1
	StatusCanceled         = 9
	StatusHacked           = 11
	StatusJudging          = 20
	StatusCompiling        = 21
	StatusHackSuccessful   = 32
	StatusHackUnsuccessful = 33
)

type ProblemInfo struct {
	DocID    int64  `json:"docId"`
	PID      string `json:"pid"`
	Title    string `json:"title"`
	Hackable bool   `json:"hackable,omitempty"`
}

type UserInfo struct {
	ID          int64  `bson:"_id" json:"id"`
	Username    string `bson:"uname" json:"username"`
	DisplayName string `bson:"displayName" json:"displayName,omitempty"`
}

type Record struct {
	ID            bson.ObjectID     `bson:"_id" json:"id"`
	UID           int64             `bson:"uid" json:"uid"`
	PID           int64             `bson:"pid" json:"pid"`
	DomainID      string            `bson:"domainId" json:"domainId"`
	Lang          string            `bson:"lang" json:"lang"`
	Code          string            `bson:"code" json:"code,omitempty"`
	Status        int               `bson:"status" json:"status"`
	Score         int               `bson:"score" json:"score"`
	Time          float64           `bson:"time" json:"time"`
	Memory        int               `bson:"memory" json:"memory"`
	JudgeTexts    []any             `bson:"judgeTexts" json:"judgeTexts,omitempty"`
	CompilerTexts []string          `bson:"compilerTexts" json:"compilerTexts,omitempty"`
	TestCases     []any             `bson:"testCases" json:"testCases,omitempty"`
	Judger        *int64            `bson:"judger" json:"judger,omitempty"`
	JudgeAt       *time.Time        `bson:"judgeAt" json:"judgeAt,omitempty"`
	Rejudged      bool              `bson:"rejudged" json:"rejudged"`
	Type          string            `bson:"type,omitempty" json:"type,omitempty"`
	Source        string            `bson:"source" json:"source,omitempty"`
	Contest       *bson.ObjectID    `bson:"contest,omitempty" json:"contest,omitempty"`
	Files         map[string]string `bson:"files,omitempty" json:"files,omitempty"`
	HackTarget    *bson.ObjectID    `bson:"hackTarget,omitempty" json:"hackTarget,omitempty"`
	Length        int               `bson:"length" json:"length,omitempty"`
	Problem       *ProblemInfo      `bson:"-" json:"problem,omitempty"`
	Submitter     *UserInfo         `bson:"-" json:"submitter,omitempty"`
	CreatedAt     time.Time         `bson:"-" json:"createdAt"`
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
	vusers       *mongo.Collection
	problems     *problem.Repository
	store        *judge.FileStore
	acceptedHook AcceptedHook
	updateHook   func(RecordUpdate)
}

func NewRepository(db *mongo.Database, problems *problem.Repository) *Repository {
	return &Repository{
		records:  db.Collection("record"),
		tasks:    db.Collection("task"),
		users:    db.Collection("user"),
		vusers:   db.Collection("vuser"),
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
	savedRecord, err := r.Get(ctx, rid.Hex())
	if err == nil {
		r.emit(savedRecord)
	}
	return savedRecord, err
}

// CreateGenerate queues hydrojudge's native generate task. The generator and
// standard solution are existing testdata source files; generated cases are
// uploaded back through /judge/upload before the task ends.
func (r *Repository) CreateGenerate(ctx context.Context, uid int64, pid, gen, std string) (*Record, error) {
	pdoc, err := r.problems.GetForEditor(ctx, pid)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(pdoc.Data))
	for _, file := range pdoc.Data {
		names[file.Name] = true
	}
	if !names[gen] || !names[std] {
		return nil, fmt.Errorf("generator and standard solution must be existing testdata files")
	}
	config, err := configMap(pdoc.Config)
	if err != nil {
		return nil, err
	}
	rid := bson.NewObjectID()
	now := time.Now().UTC()
	record := bson.M{
		"_id": rid, "uid": uid, "pid": pdoc.DocID, "domainId": "system",
		"lang": "_", "code": gen + "\n" + std, "status": StatusWaiting,
		"type": "generate", "score": 0, "time": 0, "memory": 0,
		"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{},
		"judger": nil, "judgeAt": nil, "rejudged": false,
		"source": fmt.Sprintf("system/%d", pdoc.DocID), "createdAt": now,
	}
	if _, err := r.records.InsertOne(ctx, record); err != nil {
		return nil, err
	}
	task := bson.M{
		"_id": bson.NewObjectID(), "type": "generate", "rid": rid,
		"lang": "_", "code": gen + "\n" + std, "uid": uid, "pid": pdoc.DocID,
		"domainId": "system", "data": pdoc.Data,
		"source": fmt.Sprintf("system/%d", pdoc.DocID), "config": config,
		"meta": bson.M{"problemOwner": pdoc.Owner}, "priority": 10,
		"trusted": false, "input": []string{},
	}
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	savedRecord, err := r.Get(ctx, rid.Hex())
	if err == nil {
		r.emit(savedRecord)
	}
	return savedRecord, err
}

// CreatePretest queues hydrojudge's native pretest flow. The supplied inputs
// replace the problem's testdata for this run and never affect problem stats.
func (r *Repository) CreatePretest(ctx context.Context, uid int64, pid, lang, code string, input []string) (*Record, error) {
	pdoc, err := r.problems.GetForEditor(ctx, pid)
	if err != nil {
		return nil, err
	}
	if lang == "" || code == "" {
		return nil, fmt.Errorf("lang and code are required")
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("pretest input is required")
	}
	rid := bson.NewObjectID()
	now := time.Now().UTC()
	pretestContest := bson.NilObjectID
	record := bson.M{
		"_id": rid, "uid": uid, "pid": pdoc.DocID, "domainId": "system",
		"lang": lang, "code": code, "status": StatusWaiting,
		"type": "pretest", "contest": pretestContest, "input": input,
		"score": 0, "time": 0, "memory": 0,
		"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{},
		"judger": nil, "judgeAt": nil, "rejudged": false,
		"source": fmt.Sprintf("system/%d", pdoc.DocID), "createdAt": now,
	}
	if _, err := r.records.InsertOne(ctx, record); err != nil {
		return nil, err
	}
	task, err := buildJudgeTask(pdoc, rid, uid, lang, code, &pretestContest)
	if err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	task["type"] = "pretest"
	task["input"] = input
	task["priority"] = -20
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		return nil, err
	}
	return r.Get(ctx, rid.Hex())
}

// CreateHack queues hydrojudge's native hack mode using another user's
// accepted code and a user-supplied input.
func (r *Repository) CreateHack(ctx context.Context, uid int64, pid, targetRID string, input []byte, contestID ...string) (*Record, error) {
	if r.store == nil {
		return nil, fmt.Errorf("hack storage is unavailable")
	}
	pdoc, err := r.problems.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	if !pdoc.Hackable {
		return nil, fmt.Errorf("this problem is not hackable")
	}
	targetOID, err := bson.ObjectIDFromHex(strings.TrimSpace(targetRID))
	if err != nil {
		return nil, fmt.Errorf("invalid target submission")
	}
	target, err := r.Get(ctx, targetOID.Hex())
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("target submission not found")
		}
		return nil, err
	}
	if target.PID != pdoc.DocID {
		return nil, fmt.Errorf("target submission belongs to another problem")
	}
	if target.UID == uid {
		return nil, fmt.Errorf("you cannot hack your own submission")
	}
	if target.Status != StatusAccepted {
		return nil, fmt.Errorf("you cannot hack an unsuccessful submission")
	}
	if target.Type == "hack" || len(target.Files) > 0 {
		return nil, fmt.Errorf("hack records cannot be hacked")
	}
	var requestedContest *bson.ObjectID
	if len(contestID) > 0 && strings.TrimSpace(contestID[0]) != "" {
		oid, err := bson.ObjectIDFromHex(strings.TrimSpace(contestID[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid contest id")
		}
		requestedContest = &oid
	}
	if target.Contest == nil && requestedContest != nil {
		return nil, fmt.Errorf("target submission does not belong to this contest")
	}
	if target.Contest != nil && (requestedContest == nil || *target.Contest != *requestedContest) {
		return nil, fmt.Errorf("target submission belongs to another contest")
	}
	acceptedFilter := bson.M{
		"domainId": "system", "uid": uid, "pid": pdoc.DocID, "status": StatusAccepted,
		"type": bson.M{"$nin": []string{"hack", "generate", "pretest"}},
	}
	if requestedContest != nil {
		acceptedFilter["contest"] = *requestedContest
	} else {
		acceptedFilter["$or"] = []bson.M{{"contest": bson.M{"$exists": false}}, {"contest": nil}}
	}
	accepted, err := r.records.CountDocuments(ctx, acceptedFilter)
	if err != nil {
		return nil, err
	}
	if accepted == 0 {
		return nil, fmt.Errorf("you must accept this problem before hacking")
	}
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, fmt.Errorf("hack input is required")
	}
	rid := bson.NewObjectID()
	inputID := rid.Hex()
	if err := r.store.SaveHackInput(inputID, bytes.NewReader(input)); err != nil {
		return nil, err
	}
	cleanupInput := true
	defer func() {
		if cleanupInput {
			_ = r.store.DeleteHackInput(inputID)
		}
	}()
	now := time.Now().UTC()
	hackRef := "hack-" + inputID
	record := bson.M{
		"_id": rid, "uid": uid, "pid": pdoc.DocID, "domainId": "system",
		"lang": target.Lang, "code": target.Code, "status": StatusWaiting, "type": "hack",
		"hackTarget": targetOID, "files": bson.M{"hack": hackRef},
		"score": 0, "time": 0, "memory": 0,
		"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{},
		"judger": nil, "judgeAt": nil, "rejudged": false,
		"source": fmt.Sprintf("system/%d", pdoc.DocID), "createdAt": now,
	}
	if requestedContest != nil {
		record["contest"] = *requestedContest
	}
	if _, err := r.records.InsertOne(ctx, record); err != nil {
		return nil, err
	}
	cleanupRecord := true
	defer func() {
		if cleanupRecord {
			_, _ = r.records.DeleteOne(ctx, bson.M{"_id": rid})
		}
	}()
	task, err := buildJudgeTask(pdoc, rid, uid, target.Lang, target.Code, requestedContest)
	if err != nil {
		return nil, err
	}
	task["type"] = "hack"
	task["files"] = bson.M{"hack": hackRef}
	task["meta"] = bson.M{"problemOwner": pdoc.Owner, "hackTarget": targetOID}
	task["priority"] = 10
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		return nil, err
	}
	cleanupRecord = false
	cleanupInput = false
	saved, err := r.Get(ctx, rid.Hex())
	if err == nil {
		r.emit(saved)
	}
	return saved, err
}

// OpenHackInput returns the stored input for a hack record after checking that
// the record actually owns a hack file.
func (r *Repository) OpenHackInput(ctx context.Context, rid string) (io.ReadCloser, error) {
	if r.store == nil {
		return nil, fmt.Errorf("hack storage is unavailable")
	}
	record, err := r.Get(ctx, rid)
	if err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(record.Files["hack"])
	id, ok := strings.CutPrefix(strings.SplitN(ref, "#", 2)[0], "hack-")
	if !ok || id == "" {
		return nil, fmt.Errorf("submission has no hack input")
	}
	return r.store.OpenHackInput(id)
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
		bson.M{"domainId": "system", "pid": pdoc.DocID, "type": bson.M{"$nin": []string{"generate", "pretest"}}},
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
		bson.M{"domainId": "system", "pid": pdoc.DocID, "type": bson.M{"$nin": []string{"generate", "pretest"}}},
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

// RejudgeRecord resets one record and queues a new judge task, mirroring the
// upstream record detail Rejudge action.
func (r *Repository) RejudgeRecord(ctx context.Context, rid string) (*Record, error) {
	oid, err := bson.ObjectIDFromHex(rid)
	if err != nil {
		return nil, ErrNotFound
	}
	var current struct {
		UID     int64          `bson:"uid"`
		PID     int64          `bson:"pid"`
		Lang    string         `bson:"lang"`
		Code    string         `bson:"code"`
		Status  int            `bson:"status"`
		Type    string         `bson:"type"`
		Contest *bson.ObjectID `bson:"contest"`
	}
	if err := r.records.FindOne(ctx, bson.M{"_id": oid}).Decode(&current); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if current.Type == "generate" || current.Type == "pretest" {
		return nil, fmt.Errorf("synthetic records cannot be rejudged")
	}
	if current.Type == "hack" {
		return nil, fmt.Errorf("hack records cannot be rejudged")
	}
	pdoc, err := r.problems.GetForEditor(ctx, strconv.FormatInt(current.PID, 10))
	if err != nil {
		return nil, err
	}
	task, err := buildJudgeTask(pdoc, oid, current.UID, current.Lang, current.Code, current.Contest)
	if err != nil {
		return nil, err
	}
	if _, err := r.records.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"status": StatusWaiting, "score": 0, "time": 0, "memory": 0,
		"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{}, "subtasks": bson.M{},
		"judger": nil, "judgeAt": nil, "rejudged": true, "progress": 0,
	}}); err != nil {
		return nil, err
	}
	if _, err := r.tasks.DeleteMany(ctx, bson.M{"rid": oid}); err != nil {
		return nil, err
	}
	if _, err := r.tasks.InsertOne(ctx, task); err != nil {
		return nil, err
	}
	if current.Status == StatusAccepted {
		if err := r.problems.IncStats(ctx, current.PID, 0, -1); err != nil {
			slog.Warn("unable to adjust problem accept counter after record rejudge", "pid", current.PID, "error", err)
		}
	}
	record, err := r.Get(ctx, rid)
	if err == nil {
		r.emit(record)
	}
	return record, err
}

// CancelRecord marks one record as canceled and removes its pending judge task.
func (r *Repository) CancelRecord(ctx context.Context, rid string) (*Record, error) {
	oid, err := bson.ObjectIDFromHex(rid)
	if err != nil {
		return nil, ErrNotFound
	}
	var current struct {
		PID    int64  `bson:"pid"`
		Status int    `bson:"status"`
		Type   string `bson:"type"`
	}
	if err := r.records.FindOne(ctx, bson.M{"_id": oid}, options.FindOne().SetProjection(bson.M{"pid": 1, "status": 1, "type": 1})).Decode(&current); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if current.Type == "generate" || current.Type == "pretest" {
		return nil, fmt.Errorf("synthetic records cannot be canceled")
	}
	if current.Type == "hack" {
		return nil, fmt.Errorf("hack records cannot be canceled")
	}
	if _, err := r.records.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"status": StatusCanceled, "score": 0, "time": 0, "memory": 0,
		"testCases": []any{bson.M{
			"id": 0, "subtaskId": 0, "status": StatusCanceled,
			"score": 0, "time": 0, "memory": 0, "message": "score canceled",
		}},
		"subtasks": bson.M{},
	}}); err != nil {
		return nil, err
	}
	if _, err := r.tasks.DeleteMany(ctx, bson.M{"rid": oid}); err != nil {
		return nil, err
	}
	if current.Status == StatusAccepted {
		if err := r.problems.IncStats(ctx, current.PID, 0, -1); err != nil {
			slog.Warn("unable to adjust problem accept counter after record cancel", "pid", current.PID, "error", err)
		}
	}
	record, err := r.Get(ctx, rid)
	if err == nil {
		r.emit(record)
	}
	return record, err
}

func (r *Repository) List(ctx context.Context, opts ListFilter) (*ListResult, error) {
	opts = normalizeListFilter(opts)
	filter := bson.M{"domainId": "system", "type": bson.M{"$nin": []string{"generate", "pretest"}}}
	if opts.UID != 0 {
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
		"judgeAt": 1, "rejudged": 1, "source": 1, "contest": 1, "files": 1, "hackTarget": 1,
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
		problemByID[pdoc.DocID] = ProblemInfo{DocID: pdoc.DocID, PID: pdoc.PID, Title: pdoc.Title, Hackable: pdoc.Hackable}
	}
	uids := make([]int64, 0, len(uidSet))
	for uid := range uidSet {
		uids = append(uids, uid)
	}
	var users []UserInfo
	userIDs := make([]int64, 0, len(uids))
	teamIDs := make([]int64, 0, len(uids))
	for _, uid := range uids {
		if uid > 0 {
			userIDs = append(userIDs, uid)
		} else if uid < 0 {
			teamIDs = append(teamIDs, uid)
		}
	}
	if len(userIDs) > 0 {
		userCursor, err := r.users.Find(ctx, bson.M{"_id": bson.M{"$in": userIDs}}, options.Find().SetProjection(bson.M{"uname": 1, "displayName": 1}))
		if err != nil {
			return err
		}
		if err := userCursor.All(ctx, &users); err != nil {
			_ = userCursor.Close(ctx)
			return err
		}
		_ = userCursor.Close(ctx)
	}
	if len(teamIDs) > 0 {
		teamCursor, err := r.vusers.Find(ctx, bson.M{"_id": bson.M{"$in": teamIDs}}, options.Find().SetProjection(bson.M{"uname": 1, "displayName": 1}))
		if err != nil {
			return err
		}
		if err := teamCursor.All(ctx, &users); err != nil {
			_ = teamCursor.Close(ctx)
			return err
		}
		_ = teamCursor.Close(ctx)
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

// SetUpdateHook registers a callback for lightweight record status updates.
func (r *Repository) SetUpdateHook(hook func(RecordUpdate)) {
	r.updateHook = hook
}

func (r *Repository) emit(record *Record) {
	if record == nil || r.updateHook == nil {
		return
	}
	r.updateHook(RecordUpdate{
		RID: record.ID.Hex(), UID: record.UID, PID: record.PID, Lang: record.Lang,
		Status: record.Status, Score: record.Score, Time: record.Time,
		Memory: record.Memory, JudgeAt: record.JudgeAt,
	})
}

func (r *Repository) emitUpdate(update RecordUpdate) {
	if r.updateHook != nil {
		r.updateHook(update)
	}
}

// SetAcceptedHook registers the callback fired for accepted submissions.
func (r *Repository) SetAcceptedHook(hook AcceptedHook) {
	r.acceptedHook = hook
}

func (r *Repository) SetFileStore(store *judge.FileStore) {
	r.store = store
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
		UID    int64  `bson:"uid"`
		PID    int64  `bson:"pid"`
		Lang   string `bson:"lang"`
		Status int    `bson:"status"`
		Type   string `bson:"type"`
	}
	if err := r.records.FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"uid": 1, "pid": 1, "lang": 1, "status": 1, "type": 1})).Decode(&current); err != nil {
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
	if current.Type != "generate" && current.Type != "pretest" {
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
	}
	if current.Type == "hack" {
		if status, ok := toInt(set["status"]); ok && status == StatusHackSuccessful {
			if err := r.applySuccessfulHack(ctx, oid); err != nil {
				slog.Warn("unable to apply successful hack", "rid", rawRID, "error", err)
				_, _ = r.records.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$push": bson.M{
					"judgeTexts": bson.M{"message": "Unable to apply hack: " + err.Error()},
				}})
			}
		}
	}
	event := RecordUpdate{
		RID: rawRID, UID: current.UID, PID: current.PID, Lang: current.Lang,
		Status: current.Status, Score: 0, Time: 0, Memory: 0,
	}
	if value, ok := set["status"]; ok {
		if number, ok := toInt(value); ok {
			event.Status = number
		}
	}
	if value, ok := set["score"]; ok {
		if number, ok := toInt(value); ok {
			event.Score = number
		}
	}
	if value, ok := set["memory"]; ok {
		if number, ok := toInt(value); ok {
			event.Memory = number
		}
	}
	if value, ok := set["time"]; ok {
		if number, ok := toFloat(value); ok {
			event.Time = number
		}
	}
	if value, ok := set["judgeAt"].(time.Time); ok {
		event.JudgeAt = &value
	}
	if current.Type != "pretest" {
		r.emitUpdate(event)
	}
	if hook := r.acceptedHook; hook != nil && current.Type != "generate" && current.Type != "pretest" && isAccepted(set["status"]) {
		var record struct {
			UID     int64          `bson:"uid"`
			PID     int64          `bson:"pid"`
			Contest *bson.ObjectID `bson:"contest"`
		}
		if err := r.records.FindOne(ctx, bson.M{"_id": oid}, options.FindOne().
			SetProjection(bson.M{"uid": 1, "pid": 1, "contest": 1})).Decode(&record); err == nil {
			if record.Contest != nil && record.UID != 0 && record.PID > 0 {
				hook(ctx, *record.Contest, record.UID, record.PID)
			}
		}
	}
	return nil
}

func (r *Repository) applySuccessfulHack(ctx context.Context, hackRID bson.ObjectID) error {
	if r.store == nil {
		return fmt.Errorf("hack storage is unavailable")
	}
	hack, err := r.Get(ctx, hackRID.Hex())
	if err != nil {
		return err
	}
	if hack.HackTarget == nil {
		return fmt.Errorf("hack target is missing")
	}
	inputRef := strings.TrimSpace(hack.Files["hack"])
	inputID, ok := strings.CutPrefix(strings.SplitN(inputRef, "#", 2)[0], "hack-")
	if !ok || inputID == "" {
		return fmt.Errorf("hack input is missing")
	}
	pdoc, err := r.problems.GetForEditor(ctx, strconv.FormatInt(hack.PID, 10))
	if err != nil {
		return err
	}
	reader, err := r.store.OpenHackInput(inputID)
	if err != nil {
		return err
	}
	input, readErr := io.ReadAll(io.LimitReader(reader, 2<<20+1))
	closeErr := reader.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(input) > 2<<20 {
		return fmt.Errorf("hack input is too large")
	}
	inputName := "hack-" + hackRID.Hex() + ".in"
	configText, err := appendHackCase(pdoc.Config, inputName)
	if err != nil {
		return err
	}
	stored, err := r.store.PutProblemFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), inputName, bytes.NewReader(input))
	if err != nil {
		return err
	}
	file := problem.ProblemFile{
		ID:           fmt.Sprintf("problem/%s/%d/testdata/%s", problem.DomainSystem, pdoc.DocID, inputName),
		Name:         inputName,
		Size:         stored.Size,
		ETag:         stored.ETag,
		LastModified: stored.LastModified,
	}
	if err := r.problems.UpsertFile(ctx, pdoc.DocID, "testdata", file); err != nil {
		_ = r.store.DeleteProblemFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), inputName)
		return err
	}
	files := pdoc.Data[:0]
	for _, existing := range pdoc.Data {
		if existing.Name != inputName {
			files = append(files, existing)
		}
	}
	pdoc.Data = append(files, file)
	if err := r.problems.UpdateConfig(ctx, pdoc.DocID, configText); err != nil {
		_ = r.problems.DeleteFiles(ctx, pdoc.DocID, "testdata", []string{inputName})
		_ = r.store.DeleteProblemFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), inputName)
		return err
	}
	pdoc.Config = configText
	return r.queueHackRejudge(ctx, pdoc, inputName, hack.Contest)
}

func appendHackCase(value any, inputName string) (string, error) {
	config, err := configMap(value)
	if err != nil {
		return "", err
	}
	rawSubtasks, ok := config["subtasks"]
	if !ok {
		return "", fmt.Errorf("problem has no subtasks")
	}
	encoded, err := yaml.Marshal(rawSubtasks)
	if err != nil {
		return "", err
	}
	var subtasks []map[string]any
	if err := yaml.Unmarshal(encoded, &subtasks); err != nil {
		return "", err
	}
	if len(subtasks) == 0 {
		return "", fmt.Errorf("problem has no subtasks")
	}
	last := subtasks[len(subtasks)-1]
	var cases []map[string]any
	if rawCases, ok := last["cases"]; ok && rawCases != nil {
		encodedCases, err := yaml.Marshal(rawCases)
		if err != nil {
			return "", err
		}
		if err := yaml.Unmarshal(encodedCases, &cases); err != nil {
			return "", err
		}
	}
	for _, item := range cases {
		if strings.TrimSpace(fmt.Sprint(item["input"])) == inputName {
			last["cases"] = cases
			config["subtasks"] = subtasks
			encodedConfig, err := yaml.Marshal(config)
			return string(encodedConfig), err
		}
	}
	cases = append(cases, map[string]any{"input": inputName, "output": "/dev/null"})
	last["cases"] = cases
	subtasks[len(subtasks)-1] = last
	config["subtasks"] = subtasks
	encodedConfig, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(encodedConfig), nil
}

func (r *Repository) queueHackRejudge(ctx context.Context, pdoc *problem.Problem, inputName string, contestID ...*bson.ObjectID) error {
	filter := bson.M{
		"domainId": "system", "pid": pdoc.DocID, "status": StatusAccepted,
		"type": bson.M{"$nin": []string{"hack", "generate", "pretest"}},
	}
	if len(contestID) > 0 && contestID[0] != nil {
		filter["contest"] = *contestID[0]
	} else {
		filter["$or"] = []bson.M{{"contest": bson.M{"$exists": false}}, {"contest": nil}}
	}
	var records []struct {
		ID      bson.ObjectID  `bson:"_id"`
		UID     int64          `bson:"uid"`
		Lang    string         `bson:"lang"`
		Code    string         `bson:"code"`
		Contest *bson.ObjectID `bson:"contest"`
	}
	cursor, err := r.records.Find(ctx, filter, options.Find().SetProjection(bson.M{
		"_id": 1, "uid": 1, "lang": 1, "code": 1, "contest": 1,
	}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &records); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if _, err := r.records.UpdateMany(ctx, filter, bson.M{"$set": bson.M{
		"status": StatusWaiting, "score": 0, "time": 0, "memory": 0,
		"judgeTexts": []any{}, "compilerTexts": []string{}, "testCases": []any{}, "subtasks": bson.M{},
		"judger": nil, "judgeAt": nil, "rejudged": true, "progress": 0,
	}}); err != nil {
		return err
	}
	if err := r.problems.IncStats(ctx, pdoc.DocID, 0, -int64(len(records))); err != nil {
		return err
	}
	for _, record := range records {
		task, err := buildJudgeTask(pdoc, record.ID, record.UID, record.Lang, record.Code, record.Contest)
		if err != nil {
			return err
		}
		task["meta"] = bson.M{"problemOwner": pdoc.Owner, "hackRejudge": inputName}
		task["priority"] = -50
		if _, err := r.tasks.DeleteMany(ctx, bson.M{"rid": record.ID}); err != nil {
			return err
		}
		if _, err := r.tasks.InsertOne(ctx, task); err != nil {
			return err
		}
		r.emitUpdate(RecordUpdate{RID: record.ID.Hex(), UID: record.UID, PID: pdoc.DocID, Lang: record.Lang, Status: StatusWaiting})
	}
	return nil
}

func toInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	case float64:
		return int(number), true
	default:
		return 0, false
	}
}

func toFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	default:
		return 0, false
	}
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
	match := bson.M{"domainId": "system", "pid": filter.DocID, "type": bson.M{"$nin": []string{"generate", "pretest"}}}
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
		{"$match": bson.M{"domainId": "system", "pid": docID, "type": bson.M{"$nin": []string{"generate", "pretest"}}}},
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
