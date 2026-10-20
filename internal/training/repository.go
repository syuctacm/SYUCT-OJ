package training

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DocumentTypeTraining = 40
	DocumentTypeProblem  = 10
	DomainSystem         = "system"
)

var ErrNotFound = fmt.Errorf("training not found")

type Node struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Content     string  `json:"content,omitempty"`
	RequireNids []int64 `json:"requireNids"`
	Pids        []int64 `json:"pids"`
}

type Training struct {
	ID           bson.ObjectID `bson:"_id" json:"id"`
	DocID        bson.ObjectID `bson:"docId" json:"docId"`
	Title        string        `bson:"title" json:"title"`
	Description  string        `bson:"description" json:"description,omitempty"`
	Content      string        `bson:"content" json:"content,omitempty"`
	Owner        int64         `bson:"owner" json:"owner"`
	Attend       int64         `bson:"attend" json:"attend"`
	Pin          bool          `bson:"pin" json:"pin"`
	DAG          []Node        `bson:"-" json:"dag,omitempty"`
	SectionCount int           `bson:"-" json:"sectionCount"`
	ProblemCount int           `bson:"-" json:"problemCount"`
}

type Problem struct {
	DocID      int64  `json:"docId"`
	PID        string `json:"pid"`
	Title      string `json:"title"`
	Difficulty int    `json:"difficulty,omitempty"`
	NSubmit    int64  `json:"nSubmit"`
	NAccept    int64  `json:"nAccept"`
	Tried      bool   `json:"tried"`
	Solved     bool   `json:"solved"`
	Missing    bool   `json:"missing"`
}

type Section struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content,omitempty"`
	RequireNids []int64   `json:"requireNids"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	Problems    []Problem `json:"problems"`
}

type Detail struct {
	Training
	Enrolled      bool      `json:"enrolled"`
	Completed     bool      `json:"completed"`
	DoneProblems  int       `json:"doneProblems"`
	TotalProblems int       `json:"totalProblems"`
	Sections      []Section `json:"sections"`
}

type rawTraining struct {
	ID          bson.ObjectID `bson:"_id"`
	DocID       bson.ObjectID `bson:"docId"`
	Title       string        `bson:"title"`
	Description string        `bson:"description"`
	Content     string        `bson:"content"`
	Owner       int64         `bson:"owner"`
	Attend      int64         `bson:"attend"`
	Pin         bool          `bson:"pin"`
	DAG         []rawNode     `bson:"dag"`
}

type rawNode struct {
	ID          any    `bson:"_id"`
	Title       string `bson:"title"`
	Content     string `bson:"content"`
	RequireNids []any  `bson:"requireNids"`
	Pids        []any  `bson:"pids"`
}

type problemDocument struct {
	DocID      int64  `bson:"docId"`
	PID        string `bson:"pid"`
	Title      string `bson:"title"`
	Difficulty int    `bson:"difficulty"`
	NSubmit    int64  `bson:"nSubmit"`
	NAccept    int64  `bson:"nAccept"`
}

type Repository struct {
	documents *mongo.Collection
	statuses  *mongo.Collection
	records   *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		documents: db.Collection("document"),
		statuses:  db.Collection("document.status"),
		records:   db.Collection("record"),
	}
}

func (r *Repository) List(ctx context.Context) ([]Training, error) {
	cursor, err := r.documents.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
	}, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "description": 1, "owner": 1, "attend": 1, "pin": 1, "dag": 1}).
		SetSort(bson.D{{Key: "pin", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rawItems []rawTraining
	if err := cursor.All(ctx, &rawItems); err != nil {
		return nil, err
	}
	items := make([]Training, 0, len(rawItems))
	for _, raw := range rawItems {
		item := raw.toTraining()
		item.SectionCount = len(item.DAG)
		item.ProblemCount = len(uniquePids(item.DAG))
		item.DAG = nil
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, tid string) (*Training, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	item, err := r.getByObjectID(ctx, oid)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) Detail(ctx context.Context, tid string, uid int64) (*Detail, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	pids := uniquePids(item.DAG)
	enrolled := false
	if uid > 0 {
		enrolled, err = r.isEnrolled(ctx, item.DocID, uid)
		if err != nil {
			return nil, err
		}
	}
	donePids, progressPids := map[int64]bool{}, map[int64]bool{}
	if uid > 0 {
		donePids, progressPids, err = r.userProblemStatus(ctx, uid, pids)
		if err != nil {
			return nil, err
		}
	}
	problems, err := r.problems(ctx, pids)
	if err != nil {
		return nil, err
	}

	doneNids := map[int64]bool{}
	sections := make([]Section, 0, len(item.DAG))
	for _, node := range item.DAG {
		doneCount := 0
		sectionProblems := make([]Problem, 0, len(node.Pids))
		for _, pid := range node.Pids {
			if donePids[pid] {
				doneCount++
			}
			if p, ok := problems[pid]; ok {
				p.Tried = progressPids[pid] || donePids[pid]
				p.Solved = donePids[pid]
				sectionProblems = append(sectionProblems, p)
			} else {
				sectionProblems = append(sectionProblems, Problem{DocID: pid, Title: "题目不可用", Missing: true})
			}
		}
		status := sectionStatus(node, doneNids, donePids, progressPids)
		progress := 100
		if len(node.Pids) > 0 {
			progress = int(math.Floor(100 * float64(doneCount) / float64(len(node.Pids))))
		}
		if status == "done" {
			doneNids[node.ID] = true
		}
		sections = append(sections, Section{
			ID: node.ID, Title: node.Title, Content: node.Content,
			RequireNids: node.RequireNids, Status: status, Progress: progress,
			Problems: sectionProblems,
		})
	}
	doneProblems := 0
	for _, pid := range pids {
		if donePids[pid] {
			doneProblems++
		}
	}
	return &Detail{
		Training:      *item,
		Enrolled:      enrolled,
		Completed:     len(item.DAG) > 0 && len(doneNids) == len(item.DAG),
		DoneProblems:  doneProblems,
		TotalProblems: len(pids),
		Sections:      sections,
	}, nil
}

func (r *Repository) Enroll(ctx context.Context, tid string, uid int64) (bool, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return false, err
	}
	result, err := r.statuses.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
		"docId":    item.DocID,
		"uid":      uid,
	}, bson.M{"$set": bson.M{"enroll": 1}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return false, err
	}
	enrolledNow := result.UpsertedCount > 0 || result.ModifiedCount > 0
	if enrolledNow {
		if _, err := r.documents.UpdateOne(ctx, bson.M{"_id": item.ID}, bson.M{"$inc": bson.M{"attend": 1}}); err != nil {
			return false, err
		}
	}
	return enrolledNow, nil
}

func (r *Repository) getByObjectID(ctx context.Context, oid bson.ObjectID) (*Training, error) {
	var raw rawTraining
	err := r.documents.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}).Decode(&raw)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item := raw.toTraining()
	item.SectionCount = len(item.DAG)
	item.ProblemCount = len(uniquePids(item.DAG))
	return &item, nil
}

func (r *Repository) isEnrolled(ctx context.Context, tid bson.ObjectID, uid int64) (bool, error) {
	count, err := r.statuses.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
		"docId":    tid,
		"uid":      uid,
		"enroll":   1,
	})
	return count > 0, err
}

func (r *Repository) userProblemStatus(ctx context.Context, uid int64, pids []int64) (map[int64]bool, map[int64]bool, error) {
	done := map[int64]bool{}
	progress := map[int64]bool{}
	if len(pids) == 0 {
		return done, progress, nil
	}
	cursor, err := r.records.Find(ctx, bson.M{
		"uid": uid,
		"pid": bson.M{"$in": pids},
	}, options.Find().SetProjection(bson.M{"pid": 1, "status": 1}))
	if err != nil {
		return nil, nil, err
	}
	defer cursor.Close(ctx)
	var records []struct {
		PID    int64 `bson:"pid"`
		Status int   `bson:"status"`
	}
	if err := cursor.All(ctx, &records); err != nil {
		return nil, nil, err
	}
	for _, record := range records {
		if record.Status == 1 {
			done[record.PID] = true
		} else {
			progress[record.PID] = true
		}
	}
	return done, progress, nil
}

func (r *Repository) problems(ctx context.Context, pids []int64) (map[int64]Problem, error) {
	result := map[int64]Problem{}
	if len(pids) == 0 {
		return result, nil
	}
	cursor, err := r.documents.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblem,
		"hidden":   bson.M{"$ne": true},
		"docId":    bson.M{"$in": pids},
	}, options.Find().SetProjection(bson.M{
		"docId": 1, "pid": 1, "title": 1, "difficulty": 1, "nSubmit": 1, "nAccept": 1,
	}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []problemDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, doc := range docs {
		result[doc.DocID] = Problem{
			DocID: doc.DocID, PID: doc.PID, Title: doc.Title,
			Difficulty: doc.Difficulty, NSubmit: doc.NSubmit, NAccept: doc.NAccept,
		}
	}
	return result, nil
}

func (raw rawTraining) toTraining() Training {
	dag := make([]Node, 0, len(raw.DAG))
	for _, node := range raw.DAG {
		id, _ := number(node.ID)
		requires := make([]int64, 0, len(node.RequireNids))
		for _, value := range node.RequireNids {
			if n, ok := number(value); ok {
				requires = append(requires, n)
			}
		}
		pids := make([]int64, 0, len(node.Pids))
		for _, value := range node.Pids {
			if n, ok := number(value); ok {
				pids = append(pids, n)
			}
		}
		dag = append(dag, Node{
			ID: id, Title: node.Title, Content: node.Content,
			RequireNids: requires, Pids: pids,
		})
	}
	return Training{
		ID: raw.ID, DocID: raw.DocID, Title: raw.Title, Description: raw.Description,
		Content: raw.Content, Owner: raw.Owner, Attend: raw.Attend, Pin: raw.Pin, DAG: dag,
	}
}

func sectionStatus(node Node, doneNids map[int64]bool, donePids, progressPids map[int64]bool) string {
	for _, nid := range node.RequireNids {
		if !doneNids[nid] {
			return "invalid"
		}
	}
	doneCount := 0
	attempted := false
	for _, pid := range node.Pids {
		if donePids[pid] {
			doneCount++
		}
		if donePids[pid] || progressPids[pid] {
			attempted = true
		}
	}
	if len(node.Pids) == 0 || doneCount == len(node.Pids) {
		return "done"
	}
	if attempted {
		return "progress"
	}
	return "open"
}

func uniquePids(dag []Node) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0)
	for _, node := range dag {
		for _, pid := range node.Pids {
			if !seen[pid] {
				seen[pid] = true
				result = append(result, pid)
			}
		}
	}
	return result
}

func number(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if math.Trunc(n) == n {
			return int64(n), true
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}
