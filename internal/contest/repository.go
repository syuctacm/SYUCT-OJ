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

	"github.com/syuctacm/syuct-oj/internal/filemeta"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

const (
	DocumentTypeContest       = 30
	DocumentTypeClarification = 31
	DomainSystem              = "system"
)

var (
	ErrNotFound                    = fmt.Errorf("contest not found")
	ErrClarificationNotFound       = fmt.Errorf("clarification not found")
	ErrInvalidClarificationSubject = fmt.Errorf("invalid clarification subject")
	ErrEmptyClarificationContent   = fmt.Errorf("empty clarification content")
)

type EditInput struct {
	Title                string             `json:"title"`
	Content              string             `json:"content"`
	BeginAt              time.Time          `json:"beginAt"`
	EndAt                time.Time          `json:"endAt"`
	Rule                 string             `json:"rule"`
	PIDs                 []int64            `json:"pids"`
	Duration             int                `json:"duration"`
	Rated                bool               `json:"rated"`
	PenaltySince         *time.Time         `json:"penaltySince,omitempty"`
	PenaltyRules         map[string]float64 `json:"penaltyRules,omitempty"`
	LockAt               *time.Time         `json:"lockAt,omitempty"`
	Unlocked             bool               `json:"unlocked"`
	Maintainer           []int64            `json:"maintainer,omitempty"`
	Assign               []string           `json:"assign,omitempty"`
	Langs                []string           `json:"langs,omitempty"`
	AutoHide             *bool              `json:"autoHide,omitempty"`
	AllowViewCode        *bool              `json:"allowViewCode,omitempty"`
	AllowPrint           *bool              `json:"allowPrint,omitempty"`
	AllowTeam            *bool              `json:"allowTeam,omitempty"`
	KeepScoreboardHidden *bool              `json:"keepScoreboardHidden,omitempty"`
	InvitationCode       *string            `json:"invitationCode,omitempty"`
}

type Contest struct {
	ID                   bson.ObjectID      `bson:"_id" json:"id"`
	DocID                bson.ObjectID      `bson:"docId" json:"docId"`
	Title                string             `bson:"title" json:"title"`
	Content              string             `bson:"content" json:"content,omitempty"`
	BeginAt              time.Time          `bson:"beginAt" json:"beginAt"`
	EndAt                time.Time          `bson:"endAt" json:"endAt"`
	Rule                 string             `bson:"rule" json:"rule"`
	Owner                int64              `bson:"owner" json:"owner"`
	Attend               int64              `bson:"attend" json:"attend"`
	PIDs                 []int64            `bson:"pids" json:"pids"`
	Duration             int                `bson:"duration" json:"duration"`
	Rated                bool               `bson:"rated" json:"rated"`
	PenaltySince         *time.Time         `bson:"penaltySince,omitempty" json:"penaltySince,omitempty"`
	PenaltyRules         map[string]float64 `bson:"penaltyRules,omitempty" json:"penaltyRules,omitempty"`
	LockAt               *time.Time         `bson:"lockAt,omitempty" json:"lockAt,omitempty"`
	Unlocked             bool               `bson:"unlocked,omitempty" json:"unlocked,omitempty"`
	Maintainer           []int64            `bson:"maintainer,omitempty" json:"maintainer,omitempty"`
	Assign               []string           `bson:"assign,omitempty" json:"assign,omitempty"`
	Langs                []string           `bson:"langs,omitempty" json:"langs,omitempty"`
	AutoHide             bool               `bson:"autoHide,omitempty" json:"autoHide"`
	AllowViewCode        bool               `bson:"allowViewCode,omitempty" json:"allowViewCode"`
	AllowPrint           bool               `bson:"allowPrint,omitempty" json:"allowPrint"`
	AllowTeam            bool               `bson:"allowTeam,omitempty" json:"allowTeam"`
	KeepScoreboardHidden bool               `bson:"keepScoreboardHidden,omitempty" json:"keepScoreboardHidden"`
	Files                []filemeta.File    `bson:"files,omitempty" json:"files,omitempty"`
	PrivateFiles         []filemeta.File    `bson:"privateFiles,omitempty" json:"-"`
	InvitationCode       string             `bson:"_code,omitempty" json:"invitationCode,omitempty"`
}

type ProblemCell struct {
	Solved   bool   `json:"solved"`
	Attempts int    `json:"attempts"`
	Time     int    `json:"time"`
	RID      string `json:"rid,omitempty"`
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
	statuses *mongo.Collection
	problems *problem.Repository
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		coll:     db.Collection("document"),
		records:  db.Collection("record"),
		statuses: db.Collection("document.status"),
		problems: problem.NewRepository(db),
	}
}

// Attended reports whether the user has attended the contest.
func (r *Repository) Attended(ctx context.Context, tid string, uid int64) (bool, error) {
	if uid <= 0 {
		return false, nil
	}
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return false, ErrNotFound
	}
	count, err := r.statuses.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"docId":    oid,
		"uid":      uid,
		"attend":   1,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Attend marks the user as an attendee of the contest. It is idempotent and
// increments the attendee counter only on the first attendance.
func (r *Repository) Attend(ctx context.Context, tid string, uid int64) (bool, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return false, err
	}
	result, err := r.statuses.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"docId":    item.DocID,
		"uid":      uid,
	}, bson.M{"$set": bson.M{"attend": 1}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return false, err
	}
	changed := result.UpsertedCount > 0 || result.ModifiedCount > 0
	if changed {
		if _, err := r.coll.UpdateOne(ctx, bson.M{"_id": item.ID}, bson.M{"$inc": bson.M{"attend": 1}}); err != nil {
			return false, err
		}
	}
	return changed, nil
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
	if input.PenaltyRules != nil {
		document["penaltyRules"] = input.PenaltyRules
	}
	if input.Maintainer != nil {
		document["maintainer"] = input.Maintainer
	}
	if input.Assign != nil {
		document["assign"] = input.Assign
	}
	if input.Langs != nil {
		document["langs"] = input.Langs
	}
	if input.AutoHide != nil {
		document["autoHide"] = *input.AutoHide
	}
	if input.AllowViewCode != nil {
		document["allowViewCode"] = *input.AllowViewCode
	}
	if input.AllowPrint != nil {
		document["allowPrint"] = *input.AllowPrint
	}
	if input.AllowTeam != nil {
		document["allowTeam"] = *input.AllowTeam
	}
	if input.KeepScoreboardHidden != nil {
		document["keepScoreboardHidden"] = *input.KeepScoreboardHidden
	}
	if input.InvitationCode != nil && *input.InvitationCode != "" {
		document["_code"] = *input.InvitationCode
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
	if input.PenaltyRules != nil {
		set["penaltyRules"] = input.PenaltyRules
	}
	if input.Maintainer != nil {
		set["maintainer"] = input.Maintainer
	}
	if input.Assign != nil {
		set["assign"] = input.Assign
	}
	if input.Langs != nil {
		set["langs"] = input.Langs
	}
	if input.AutoHide != nil {
		set["autoHide"] = *input.AutoHide
	}
	if input.AllowViewCode != nil {
		set["allowViewCode"] = *input.AllowViewCode
	}
	if input.AllowPrint != nil {
		set["allowPrint"] = *input.AllowPrint
	}
	if input.AllowTeam != nil {
		set["allowTeam"] = *input.AllowTeam
	}
	if input.KeepScoreboardHidden != nil {
		set["keepScoreboardHidden"] = *input.KeepScoreboardHidden
	}
	if input.InvitationCode != nil {
		if *input.InvitationCode == "" {
			unset["_code"] = ""
		} else {
			set["_code"] = *input.InvitationCode
		}
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
	maxDuration := 30 * 24 * time.Hour
	if input.Rule == "homework" {
		maxDuration = 366 * 24 * time.Hour
	}
	if input.EndAt.Sub(input.BeginAt) > maxDuration {
		return fmt.Errorf("contest duration must not exceed %d days", int(maxDuration/(24*time.Hour)))
	}
	if input.PenaltySince != nil && (input.PenaltySince.Before(input.BeginAt) || input.PenaltySince.After(input.EndAt)) {
		return fmt.Errorf("penalty time must be inside the homework window")
	}
	if input.LockAt != nil && (input.LockAt.Before(input.BeginAt) || input.LockAt.After(input.EndAt)) {
		return fmt.Errorf("lock time must be inside the contest window")
	}
	return r.problems.ValidateDocIDs(ctx, input.PIDs)
}

func validRule(rule string) bool {
	switch rule {
	case "acm", "oi", "ioi", "strictioi", "ledo", "homework":
		return true
	default:
		return false
	}
}

func (r *Repository) List(ctx context.Context) ([]Contest, error) {
	items, _, err := r.ListFiltered(ctx, "", "", 1, 1000)
	return items, err
}

func (r *Repository) ListFiltered(ctx context.Context, query, rule string, page, pageSize int) ([]Contest, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
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
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "beginAt", Value: -1}}).
		SetSkip(int64((page-1)*pageSize)).
		SetLimit(int64(pageSize)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	items := make([]Contest, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) ListHomework(ctx context.Context) ([]Contest, error) {
	items, _, err := r.ListHomeworkFiltered(ctx, "", 1, 1000)
	return items, err
}

func (r *Repository) ListHomeworkFiltered(ctx context.Context, query string, page, pageSize int) ([]Contest, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
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
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "endAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page-1)*pageSize)).
		SetLimit(int64(pageSize)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	items := make([]Contest, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
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

// ProblemStatus is the latest submission of a user for one problem.
type ProblemStatus struct {
	RID    string `json:"rid"`
	Status int    `json:"status"`
	Score  int    `json:"score"`
}

// UserProblemStatus returns the latest submission of the user for each problem.
func (r *Repository) UserProblemStatus(ctx context.Context, uid int64, pids []int64) (map[int64]ProblemStatus, error) {
	result := map[int64]ProblemStatus{}
	if uid <= 0 || len(pids) == 0 {
		return result, nil
	}
	cursor, err := r.records.Find(ctx, bson.M{
		"uid": uid,
		"pid": bson.M{"$in": pids},
	}, options.Find().
		SetProjection(bson.M{"pid": 1, "status": 1, "score": 1}).
		SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []struct {
		ID     bson.ObjectID `bson:"_id"`
		PID    int64         `bson:"pid"`
		Status int           `bson:"status"`
		Score  int           `bson:"score"`
	}
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, ok := result[item.PID]; ok {
			continue
		}
		result[item.PID] = ProblemStatus{RID: item.ID.Hex(), Status: item.Status, Score: item.Score}
	}
	return result, nil
}

// ClarificationReply is a jury reply under a contest clarification.
type ClarificationReply struct {
	ID      bson.ObjectID `bson:"_id" json:"id"`
	Owner   int64         `bson:"owner" json:"owner"`
	Content string        `bson:"content" json:"content"`
}

// Clarification is a contest question or jury announcement. It follows the
// Hydro `document` layout: docType=31, parentType=30, parentId=contest docId.
type Clarification struct {
	ID       bson.ObjectID        `bson:"_id" json:"id"`
	DocID    bson.ObjectID        `bson:"docId" json:"docId"`
	ParentID bson.ObjectID        `bson:"parentId" json:"parentId"`
	Owner    int64                `bson:"owner" json:"owner"`
	Subject  int64                `bson:"subject" json:"subject"`
	Content  string               `bson:"content" json:"content"`
	Reply    []ClarificationReply `bson:"reply" json:"reply"`
}

// ListClarifications returns the clarifications visible to the user. When all
// is true (contest owner, maintainer or admin) every clarification is returned,
// otherwise only jury announcements and the user's own questions.
func (r *Repository) ListClarifications(ctx context.Context, tid string, uid int64, all bool) ([]Clarification, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	filter := bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeClarification,
		"parentType": DocumentTypeContest,
		"parentId":   item.DocID,
	}
	if !all {
		filter["owner"] = bson.M{"$in": []int64{0, uid}}
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Clarification, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// AddClarification records a question from an attendee.
func (r *Repository) AddClarification(ctx context.Context, tid string, uid, subject int64, content string) (*Clarification, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrEmptyClarificationContent
	}
	if subject != 0 && subject != -1 {
		valid := false
		for _, pid := range item.PIDs {
			if pid == subject {
				valid = true
				break
			}
		}
		if !valid {
			return nil, ErrInvalidClarificationSubject
		}
	}
	id := bson.NewObjectID()
	clarification := Clarification{
		ID:       id,
		DocID:    id,
		ParentID: item.DocID,
		Owner:    uid,
		Subject:  subject,
		Content:  content,
		Reply:    []ClarificationReply{},
	}
	if _, err := r.coll.InsertOne(ctx, bson.M{
		"_id":        id,
		"docId":      id,
		"domainId":   DomainSystem,
		"docType":    DocumentTypeClarification,
		"parentType": DocumentTypeContest,
		"parentId":   item.DocID,
		"owner":      uid,
		"subject":    subject,
		"content":    content,
		"reply":      []ClarificationReply{},
	}); err != nil {
		return nil, err
	}
	return &clarification, nil
}

// AddClarificationReply appends a jury reply to a clarification.
func (r *Repository) AddClarificationReply(ctx context.Context, cid string, uid int64, content string) (*Clarification, error) {
	oid, err := bson.ObjectIDFromHex(cid)
	if err != nil {
		return nil, ErrClarificationNotFound
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrEmptyClarificationContent
	}
	reply := ClarificationReply{ID: bson.NewObjectID(), Owner: uid, Content: content}
	result := r.coll.FindOneAndUpdate(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeClarification,
		"$or":      []bson.M{{"_id": oid}, {"docId": oid}},
	}, bson.M{"$push": bson.M{"reply": reply}}, options.FindOneAndUpdate().SetReturnDocument(options.After))
	var clarification Clarification
	if err := result.Decode(&clarification); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrClarificationNotFound
		}
		return nil, err
	}
	return &clarification, nil
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
	// Seed attendees so users without submissions still appear on the scoreboard.
	attendeeCursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeContest,
		"docId":    item.DocID,
		"attend":   1,
	}, options.Find().SetProjection(bson.M{"uid": 1}))
	if err != nil {
		return nil, nil, err
	}
	defer attendeeCursor.Close(ctx)
	var attendees []struct {
		UID int64 `bson:"uid"`
	}
	if err := attendeeCursor.All(ctx, &attendees); err != nil {
		return nil, nil, err
	}
	for _, attendee := range attendees {
		if attendee.UID > 0 {
			users[attendee.UID] = &userState{uid: attendee.UID, cells: make(map[string]ProblemCell), wrong: make(map[int64]int)}
		}
	}
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
		rid, _ := record["_id"].(bson.ObjectID)
		if item.LockAt != nil && !item.Unlocked && rid.Timestamp().After(*item.LockAt) {
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
		if status == 1 {
			minutes := int(rid.Timestamp().Sub(item.BeginAt).Minutes())
			if minutes < 0 {
				minutes = 0
			}
			cell.Solved = true
			cell.Attempts = state.wrong[pid]
			cell.Time = minutes
			cell.RID = rid.Hex()
			state.cells[key] = cell
			state.solved++
			state.penalty += minutes + 20*state.wrong[pid]
			continue
		}
		if status >= 2 && status <= 8 {
			state.wrong[pid]++
			cell.Attempts = state.wrong[pid]
			cell.RID = rid.Hex()
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

func (r *Repository) FileOwner(ctx context.Context, tid string) (int64, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return 0, err
	}
	return item.Owner, nil
}

func (r *Repository) Files(ctx context.Context, tid string) ([]filemeta.File, error) {
	return r.files(ctx, tid, "files")
}

func (r *Repository) PrivateFiles(ctx context.Context, tid string) ([]filemeta.File, error) {
	return r.files(ctx, tid, "privateFiles")
}

func (r *Repository) files(ctx context.Context, tid, field string) ([]filemeta.File, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	var stored []filemeta.File
	if field == "privateFiles" {
		stored = item.PrivateFiles
	} else {
		stored = item.Files
	}
	files := append([]filemeta.File(nil), stored...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func (r *Repository) UpsertFile(ctx context.Context, tid string, file filemeta.File) error {
	return r.upsertFile(ctx, tid, "files", file)
}

func (r *Repository) UpsertPrivateFile(ctx context.Context, tid string, file filemeta.File) error {
	return r.upsertFile(ctx, tid, "privateFiles", file)
}

func (r *Repository) upsertFile(ctx context.Context, tid, field string, file filemeta.File) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	if err := filemeta.Upsert(ctx, r.coll, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeContest,
		"$or": []bson.M{{"docId": oid}, {"_id": oid}},
	}, field, file); err != nil {
		if err == filemeta.ErrNotFound {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) DeleteFiles(ctx context.Context, tid string, names []string) error {
	return r.deleteFiles(ctx, tid, "files", names)
}

func (r *Repository) DeletePrivateFiles(ctx context.Context, tid string, names []string) error {
	return r.deleteFiles(ctx, tid, "privateFiles", names)
}

func (r *Repository) deleteFiles(ctx context.Context, tid, field string, names []string) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	if err := filemeta.Delete(ctx, r.coll, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeContest,
		"$or": []bson.M{{"docId": oid}, {"_id": oid}},
	}, field, names); err != nil {
		if err == filemeta.ErrNotFound {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// CanReadPrivateFiles reports whether the participant may download contest
// materials. Owners always can; participants only after the contest started.
func (r *Repository) CanReadPrivateFiles(ctx context.Context, tid string, uid int64) (bool, error) {
	if uid <= 0 {
		return false, nil
	}
	item, err := r.Get(ctx, tid)
	if err != nil {
		return false, err
	}
	if item.Owner == uid {
		return true, nil
	}
	if time.Now().Before(item.BeginAt) {
		return false, nil
	}
	return r.Attended(ctx, tid, uid)
}

// Unlock marks a locked scoreboard as unlocked so later submissions count.
func (r *Repository) Unlock(ctx context.Context, tid string) (*Contest, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	if _, err := r.coll.UpdateOne(ctx, bson.M{"_id": item.ID}, bson.M{"$set": bson.M{"unlocked": true}}); err != nil {
		return nil, err
	}
	item.Unlocked = true
	return item, nil
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
