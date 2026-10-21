package contest

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Print task statuses, matching Hydro's PrintTaskStatus.
const (
	PrintPending  = "pending"
	PrintPrinting = "printing"
	PrintPrinted  = "printed"
	PrintFailed   = "failed"

	// DocumentTypeContestPrint is Hydro's TYPE_CONTEST_PRINT.
	DocumentTypeContestPrint = 32
)

// PrintTask is one queued print request of a contest.
type PrintTask struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	Owner     int64         `bson:"owner" json:"owner"`
	Title     string        `bson:"title" json:"title"`
	Content   string        `bson:"content" json:"content"`
	Status    string        `bson:"status" json:"status"`
	CreatedAt time.Time     `bson:"-" json:"createdAt"`
}

// AddPrintTask queues a print request for a contest attendee.
func (r *Repository) AddPrintTask(ctx context.Context, tid string, owner int64, title, content string) (*PrintTask, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	task := PrintTask{
		ID:      bson.NewObjectID(),
		Owner:   owner,
		Title:   strings.TrimSpace(title),
		Content: content,
		Status:  PrintPending,
	}
	if task.Title == "" {
		task.Title = "file"
	}
	if _, err := r.coll.InsertOne(ctx, bson.M{
		"_id":        task.ID,
		"domainId":   DomainSystem,
		"docType":    DocumentTypeContestPrint,
		"docId":      task.ID,
		"parentType": DocumentTypeContest,
		"parentId":   oid,
		"owner":      owner,
		"title":      task.Title,
		"content":    task.Content,
		"status":     task.Status,
	}); err != nil {
		return nil, err
	}
	task.CreatedAt = task.ID.Timestamp()
	return &task, nil
}

// PrintTasks lists the print queue of a contest. When owner is positive only
// that user's tasks are returned.
func (r *Repository) PrintTasks(ctx context.Context, tid string, owner int64) ([]PrintTask, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	filter := bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeContestPrint,
		"parentType": DocumentTypeContest,
		"parentId":   oid,
	}
	if owner > 0 {
		filter["owner"] = owner
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]PrintTask, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CreatedAt = items[i].ID.Timestamp()
	}
	return items, nil
}

// AllocatePrintTask atomically moves the oldest pending task to printing, which
// is how the kiosk claims work.
func (r *Repository) AllocatePrintTask(ctx context.Context, tid string) (*PrintTask, error) {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return nil, ErrNotFound
	}
	result := r.coll.FindOneAndUpdate(ctx, bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeContestPrint,
		"parentType": DocumentTypeContest,
		"parentId":   oid,
		"status":     PrintPending,
	}, bson.M{"$set": bson.M{"status": PrintPrinting}},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "_id", Value: 1}}).SetReturnDocument(options.After))
	var task PrintTask
	if err := result.Decode(&task); err != nil {
		return nil, nil
	}
	task.CreatedAt = task.ID.Timestamp()
	return &task, nil
}

// UpdatePrintTask changes the status of one queued task.
func (r *Repository) UpdatePrintTask(ctx context.Context, tid string, id bson.ObjectID, status string) error {
	oid, err := bson.ObjectIDFromHex(tid)
	if err != nil {
		return ErrNotFound
	}
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeContestPrint,
		"parentType": DocumentTypeContest,
		"parentId":   oid,
		"_id":        id,
	}, bson.M{"$set": bson.M{"status": status}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
