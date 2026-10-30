package training

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/syuctacm/syuct-oj/internal/filemeta"
)

const (
	DocumentTypeTraining = 40
	DocumentTypeProblem  = 10
	DomainSystem         = "system"
	statusAccepted       = 1
)

var ErrNotFound = fmt.Errorf("training not found")

type Node struct {
	ID          int64   `bson:"_id" json:"id"`
	Title       string  `bson:"title" json:"title"`
	Content     string  `bson:"content,omitempty" json:"content,omitempty"`
	RequireNids []int64 `bson:"requireNids" json:"requireNids"`
	Pids        []int64 `bson:"pids" json:"pids"`
}

type EditNode struct {
	ID          any    `json:"_id"`
	Title       string `json:"title"`
	Content     string `json:"content,omitempty"`
	RequireNids []any  `json:"requireNids"`
	Pids        []any  `json:"pids"`
}

type EditInput struct {
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Description string     `json:"description"`
	Pin         bool       `json:"pin"`
	DAG         []EditNode `json:"dag"`
}

type Training struct {
	ID           bson.ObjectID   `bson:"_id" json:"id"`
	DocID        bson.ObjectID   `bson:"docId" json:"docId"`
	Title        string          `bson:"title" json:"title"`
	Description  string          `bson:"description" json:"description,omitempty"`
	Content      string          `bson:"content" json:"content,omitempty"`
	Owner        int64           `bson:"owner" json:"owner"`
	Attend       int64           `bson:"attend" json:"attend"`
	Pin          bool            `bson:"pin" json:"pin"`
	Files        []filemeta.File `bson:"files,omitempty" json:"files,omitempty"`
	DAG          []Node          `bson:"-" json:"dag,omitempty"`
	SectionCount int             `bson:"-" json:"sectionCount"`
	ProblemCount int             `bson:"-" json:"problemCount"`
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
	RID        string `json:"rid,omitempty"`
	Status     int    `json:"status,omitempty"`
	Score      int    `json:"score,omitempty"`
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
	EnrolledUsers []int64   `json:"enrolledUsers"`
	Sections      []Section `json:"sections"`
}

type rawTraining struct {
	ID          bson.ObjectID   `bson:"_id"`
	DocID       bson.ObjectID   `bson:"docId"`
	Title       string          `bson:"title"`
	Description string          `bson:"description"`
	Content     string          `bson:"content"`
	Owner       int64           `bson:"owner"`
	Attend      int64           `bson:"attend"`
	Pin         any             `bson:"pin"`
	Files       []filemeta.File `bson:"files"`
	DAG         []rawNode       `bson:"dag"`
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

func (r *Repository) Create(ctx context.Context, owner int64, input EditInput) (*Training, error) {
	dag, err := r.normalizeDAG(ctx, input)
	if err != nil {
		return nil, err
	}
	oid := bson.NewObjectID()
	document := bson.M{
		"_id": oid, "docId": oid, "domainId": DomainSystem, "docType": DocumentTypeTraining,
		"title": strings.TrimSpace(input.Title), "content": input.Content, "description": input.Description,
		"owner": owner, "attend": int64(0), "pin": input.Pin, "dag": dag,
	}
	if _, err := r.documents.InsertOne(ctx, document); err != nil {
		return nil, err
	}
	return r.Get(ctx, oid.Hex())
}

func (r *Repository) Update(ctx context.Context, tid string, input EditInput) (*Training, error) {
	dag, err := r.normalizeDAG(ctx, input)
	if err != nil {
		return nil, err
	}
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	result, err := r.documents.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}, bson.M{"$set": bson.M{
		"title": strings.TrimSpace(input.Title), "content": input.Content,
		"description": input.Description, "pin": input.Pin, "dag": dag,
	}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, tid)
}

func (r *Repository) normalizeDAG(ctx context.Context, input EditInput) ([]Node, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if utf8.RuneCountInString(input.Content) > 500 {
		return nil, fmt.Errorf("introduction must not exceed 500 characters")
	}
	if len(input.DAG) == 0 {
		return nil, fmt.Errorf("dag must have at least one node")
	}
	ids := make(map[int64]bool, len(input.DAG))
	nodeIDs := make([]int64, len(input.DAG))
	for index, node := range input.DAG {
		id, ok := number(node.ID)
		if !ok || id <= 0 {
			return nil, fmt.Errorf("each dag node needs a positive _id")
		}
		if strings.TrimSpace(node.Title) == "" {
			return nil, fmt.Errorf("dag node %d title is required", id)
		}
		if ids[id] {
			return nil, fmt.Errorf("duplicate dag node _id %d", id)
		}
		ids[id] = true
		nodeIDs[index] = id
	}
	dag := make([]Node, 0, len(input.DAG))
	for index, node := range input.DAG {
		id := nodeIDs[index]
		requires := make([]int64, 0, len(node.RequireNids))
		seenRequires := make(map[int64]bool, len(node.RequireNids))
		for _, rawRequired := range node.RequireNids {
			required, ok := number(rawRequired)
			if !ok {
				return nil, fmt.Errorf("dag node %d has invalid required node %v", id, rawRequired)
			}
			if required == id {
				return nil, fmt.Errorf("dag node %d cannot require itself", id)
			}
			if !ids[required] {
				return nil, fmt.Errorf("required node _id %d does not exist", required)
			}
			if !seenRequires[required] {
				seenRequires[required] = true
				requires = append(requires, required)
			}
		}
		pids, err := r.resolveProblemDocIDs(ctx, node.Pids)
		if err != nil {
			return nil, fmt.Errorf("dag node %d: %w", id, err)
		}
		if len(pids) == 0 {
			return nil, fmt.Errorf("dag node %d must contain at least one problem", id)
		}
		dag = append(dag, Node{
			ID: id, Title: strings.TrimSpace(node.Title), Content: node.Content,
			RequireNids: requires, Pids: pids,
		})
	}
	return dag, nil
}

func (r *Repository) resolveProblemDocIDs(ctx context.Context, values []any) ([]int64, error) {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]bool, len(values))
	for _, value := range values {
		id, err := r.resolveProblemDocID(ctx, value)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result, nil
}

func (r *Repository) resolveProblemDocID(ctx context.Context, value any) (int64, error) {
	if id, ok := number(value); ok {
		if id <= 0 {
			return 0, fmt.Errorf("invalid problem %v", value)
		}
		count, err := r.documents.CountDocuments(ctx, bson.M{
			"domainId": DomainSystem, "docType": DocumentTypeProblem, "docId": id,
		})
		if err != nil {
			return 0, err
		}
		if count == 0 {
			return 0, fmt.Errorf("problem %d does not exist", id)
		}
		return id, nil
	}
	identifier := strings.TrimSpace(fmt.Sprint(value))
	if identifier == "" {
		return 0, fmt.Errorf("invalid empty problem")
	}
	var doc struct {
		DocID int64 `bson:"docId"`
	}
	err := r.documents.FindOne(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeProblem, "pid": identifier,
	}, options.FindOne().SetProjection(bson.M{"docId": 1})).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, fmt.Errorf("problem %s does not exist", identifier)
		}
		return 0, err
	}
	return doc.DocID, nil
}

func (r *Repository) FileOwner(ctx context.Context, tid string) (int64, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return 0, err
	}
	return item.Owner, nil
}

func (r *Repository) Files(ctx context.Context, tid string) ([]filemeta.File, error) {
	item, err := r.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	files := append([]filemeta.File(nil), item.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func (r *Repository) UpsertFile(ctx context.Context, tid string, file filemeta.File) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	if err := filemeta.Upsert(ctx, r.documents, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeTraining,
		"$or": []bson.M{{"docId": oid}, {"_id": oid}},
	}, "files", file); err != nil {
		if err == filemeta.ErrNotFound {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) DeleteFiles(ctx context.Context, tid string, names []string) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	if err := filemeta.Delete(ctx, r.documents, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeTraining,
		"$or": []bson.M{{"docId": oid}, {"_id": oid}},
	}, "files", names); err != nil {
		if err == filemeta.ErrNotFound {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) List(ctx context.Context) ([]Training, error) {
	items, _, err := r.ListFiltered(ctx, "", 1, 1000)
	return items, err
}

func (r *Repository) ListFiltered(ctx context.Context, query string, page, pageSize int) ([]Training, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
	}
	if query = strings.TrimSpace(query); query != "" {
		filter["title"] = bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"}
	}
	total, err := r.documents.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.documents.Find(ctx, filter, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "content": 1, "description": 1, "owner": 1, "attend": 1, "pin": 1, "dag": 1}).
		SetSort(bson.D{{Key: "pin", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page-1)*pageSize)).
		SetLimit(int64(pageSize)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var rawItems []rawTraining
	if err := cursor.All(ctx, &rawItems); err != nil {
		return nil, 0, err
	}
	items := make([]Training, 0, len(rawItems))
	for _, raw := range rawItems {
		item := raw.toTraining()
		item.SectionCount = len(item.DAG)
		item.ProblemCount = len(uniquePids(item.DAG))
		item.DAG = nil
		items = append(items, item)
	}
	return items, total, nil
}

// ListProgress returns lightweight enrollment and completion data for a list
// of trainings using batched status and record queries.
type ListProgress struct {
	Enrolled      bool `json:"enrolled"`
	Completed     bool `json:"completed"`
	DoneProblems  int  `json:"doneProblems"`
	TotalProblems int  `json:"totalProblems"`
}

func (r *Repository) ListProgress(ctx context.Context, uid int64, items []Training) (map[string]ListProgress, error) {
	result := make(map[string]ListProgress, len(items))
	if uid <= 0 || len(items) == 0 {
		return result, nil
	}
	docIDs := make([]bson.ObjectID, 0, len(items))
	for _, item := range items {
		docIDs = append(docIDs, item.DocID)
	}
	enrolled := make(map[bson.ObjectID]bool)
	statusCursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeTraining,
		"uid": uid, "docId": bson.M{"$in": docIDs}, "enroll": 1,
	}, options.Find().SetProjection(bson.M{"docId": 1}))
	if err != nil {
		return nil, err
	}
	var statusRows []struct {
		DocID bson.ObjectID `bson:"docId"`
	}
	if err := statusCursor.All(ctx, &statusRows); err != nil {
		_ = statusCursor.Close(ctx)
		return nil, err
	}
	_ = statusCursor.Close(ctx)
	for _, row := range statusRows {
		enrolled[row.DocID] = true
	}
	docCursor, err := r.documents.Find(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeTraining,
		"docId": bson.M{"$in": docIDs},
	}, options.Find().SetProjection(bson.M{"docId": 1, "dag": 1}))
	if err != nil {
		return nil, err
	}
	var rawDocs []rawTraining
	if err := docCursor.All(ctx, &rawDocs); err != nil {
		_ = docCursor.Close(ctx)
		return nil, err
	}
	_ = docCursor.Close(ctx)
	pidSet := make(map[int64]bool)
	pidsByDoc := make(map[bson.ObjectID][]int64, len(rawDocs))
	for _, raw := range rawDocs {
		item := raw.toTraining()
		pids := uniquePids(item.DAG)
		pidsByDoc[item.DocID] = pids
		for _, pid := range pids {
			pidSet[pid] = true
		}
	}
	allPids := make([]int64, 0, len(pidSet))
	for pid := range pidSet {
		allPids = append(allPids, pid)
	}
	records, err := r.userProblemStatus(ctx, uid, allPids)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		pids := pidsByDoc[item.DocID]
		done := 0
		for _, pid := range pids {
			if records[pid].Status == statusAccepted {
				done++
			}
		}
		result[item.DocID.Hex()] = ListProgress{
			Enrolled: enrolled[item.DocID], Completed: len(pids) > 0 && done == len(pids),
			DoneProblems: done, TotalProblems: len(pids),
		}
	}
	return result, nil
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
	records := map[int64]problemRecord{}
	if uid > 0 {
		records, err = r.userProblemStatus(ctx, uid, pids)
		if err != nil {
			return nil, err
		}
	}
	donePids, progressPids := map[int64]bool{}, map[int64]bool{}
	for pid, record := range records {
		if record.Status == statusAccepted {
			donePids[pid] = true
		} else {
			progressPids[pid] = true
		}
	}
	problems, err := r.problems(ctx, pids)
	if err != nil {
		return nil, err
	}
	enrolledUsers, err := r.enrolledUsers(ctx, item.DocID)
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
				if record, ok := records[pid]; ok {
					p.RID = record.RID
					p.Status = record.Status
					p.Score = record.Score
				}
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
		EnrolledUsers: enrolledUsers,
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

type problemRecord struct {
	RID    string
	Status int
	Score  int
}

// userProblemStatus returns the latest submission of the user for each problem.
func (r *Repository) userProblemStatus(ctx context.Context, uid int64, pids []int64) (map[int64]problemRecord, error) {
	records := map[int64]problemRecord{}
	if len(pids) == 0 {
		return records, nil
	}
	cursor, err := r.records.Find(ctx, bson.M{
		"uid":  uid,
		"pid":  bson.M{"$in": pids},
		"type": bson.M{"$nin": []string{"pretest", "generate"}},
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
		if _, ok := records[item.PID]; ok {
			continue
		}
		records[item.PID] = problemRecord{RID: item.ID.Hex(), Status: item.Status, Score: item.Score}
	}
	return records, nil
}

// enrolledUsers lists the users enrolled in the training.
func (r *Repository) enrolledUsers(ctx context.Context, tid bson.ObjectID) ([]int64, error) {
	cursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeTraining,
		"docId":    tid,
		"enroll":   1,
	}, options.Find().
		SetProjection(bson.M{"uid": 1}).
		SetSort(bson.D{{Key: "uid", Value: 1}}).
		SetLimit(500))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []struct {
		UID int64 `bson:"uid"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	uids := make([]int64, 0, len(docs))
	for _, doc := range docs {
		if doc.UID > 0 {
			uids = append(uids, doc.UID)
		}
	}
	return uids, nil
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
	pin := false
	switch value := raw.Pin.(type) {
	case bool:
		pin = value
	default:
		if pinNumber, ok := number(value); ok {
			pin = pinNumber != 0
		}
	}
	return Training{
		ID: raw.ID, DocID: raw.DocID, Title: raw.Title, Description: raw.Description,
		Content: raw.Content, Owner: raw.Owner, Attend: raw.Attend, Pin: pin, Files: raw.Files, DAG: dag,
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
