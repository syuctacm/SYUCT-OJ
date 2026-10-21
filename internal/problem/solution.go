package problem

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DocumentTypeProblemSolution is Hydro's TYPE_PROBLEM_SOLUTION.
const DocumentTypeProblemSolution = 11

// SolutionReply is one nested comment under a solution.
type SolutionReply struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	Owner     int64         `bson:"owner" json:"owner"`
	Content   string        `bson:"content" json:"content"`
	Edited    bool          `bson:"edited" json:"edited,omitempty"`
	CreatedAt time.Time     `bson:"-" json:"createdAt"`
}

// Solution is a user written solution for a problem.
type Solution struct {
	ID        bson.ObjectID   `bson:"_id" json:"id"`
	PID       int64           `bson:"parentId" json:"pid"`
	Owner     int64           `bson:"owner" json:"owner"`
	Content   string          `bson:"content" json:"content"`
	Vote      int64           `bson:"vote" json:"vote"`
	Edited    bool            `bson:"edited" json:"edited,omitempty"`
	Replies   []SolutionReply `bson:"reply" json:"replies,omitempty"`
	CreatedAt time.Time       `bson:"-" json:"createdAt"`
}

func (r *Repository) solutionFilter(extra bson.M) bson.M {
	filter := bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeProblemSolution,
		"parentType": DocumentTypeProblem,
	}
	for key, value := range extra {
		filter[key] = value
	}
	return filter
}

func (s *Solution) touch() {
	s.CreatedAt = s.ID.Timestamp()
	for i := range s.Replies {
		s.Replies[i].CreatedAt = s.Replies[i].ID.Timestamp()
	}
}

// ListSolutions lists the solutions of a problem ordered by votes.
func (r *Repository) ListSolutions(ctx context.Context, pid int64, page, limit int) ([]Solution, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	filter := r.solutionFilter(bson.M{"parentId": pid})
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "vote", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page-1)*limit)).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	items := make([]Solution, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].touch()
	}
	return items, total, nil
}

// Solution loads one solution.
func (r *Repository) Solution(ctx context.Context, sid bson.ObjectID) (*Solution, error) {
	var item Solution
	if err := r.coll.FindOne(ctx, r.solutionFilter(bson.M{"_id": sid})).Decode(&item); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item.touch()
	return &item, nil
}

// SolutionReply loads one reply together with its parent solution.
func (r *Repository) SolutionReply(ctx context.Context, sid, rid bson.ObjectID) (*Solution, *SolutionReply, error) {
	item, err := r.Solution(ctx, sid)
	if err != nil {
		return nil, nil, err
	}
	for i := range item.Replies {
		if item.Replies[i].ID == rid {
			return item, &item.Replies[i], nil
		}
	}
	return nil, nil, ErrNotFound
}

// CreateSolution stores a new solution.
func (r *Repository) CreateSolution(ctx context.Context, pid, owner int64, content string) (*Solution, error) {
	item := Solution{
		ID:        bson.NewObjectID(),
		PID:       pid,
		Owner:     owner,
		Content:   content,
		CreatedAt: time.Now(),
	}
	if _, err := r.coll.InsertOne(ctx, bson.M{
		"_id":        item.ID,
		"domainId":   DomainSystem,
		"docType":    DocumentTypeProblemSolution,
		"parentType": DocumentTypeProblem,
		"parentId":   pid,
		"owner":      owner,
		"content":    content,
		"vote":       int64(0),
		"reply":      []any{},
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateSolution replaces the content of one solution.
func (r *Repository) UpdateSolution(ctx context.Context, sid bson.ObjectID, content string) error {
	result, err := r.coll.UpdateOne(ctx, r.solutionFilter(bson.M{"_id": sid}),
		bson.M{"$set": bson.M{"content": content, "edited": true}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSolution removes a solution and the per-user vote documents.
func (r *Repository) DeleteSolution(ctx context.Context, sid bson.ObjectID) error {
	result, err := r.coll.DeleteOne(ctx, r.solutionFilter(bson.M{"_id": sid}))
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	_, _ = r.statuses.DeleteMany(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblemSolution,
		"docId":    sid,
	})
	return nil
}

// VoteSolution stores the caller's vote and keeps the counter in sync, using
// the same "difference from the previous vote" semantics as Hydro.
func (r *Repository) VoteSolution(ctx context.Context, sid bson.ObjectID, uid, value int64) (int64, error) {
	item, err := r.Solution(ctx, sid)
	if err != nil {
		return 0, err
	}
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblemSolution,
		"docId":    sid,
		"uid":      uid,
	}
	var before struct {
		Vote int64 `bson:"vote"`
	}
	if err := r.statuses.FindOne(ctx, filter).Decode(&before); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return 0, err
	}
	if _, err := r.statuses.UpdateOne(ctx, filter,
		bson.M{"$set": bson.M{"vote": value}}, options.UpdateOne().SetUpsert(true)); err != nil {
		return 0, err
	}
	delta := value - before.Vote
	if delta == 0 {
		return item.Vote, nil
	}
	if _, err := r.coll.UpdateOne(ctx, r.solutionFilter(bson.M{"_id": sid}),
		bson.M{"$inc": bson.M{"vote": delta}}); err != nil {
		return 0, err
	}
	return item.Vote + delta, nil
}

// SolutionVotes reports the caller's votes for the given solutions.
func (r *Repository) SolutionVotes(ctx context.Context, uid int64, sids []bson.ObjectID) (map[string]int64, error) {
	result := make(map[string]int64, len(sids))
	if uid <= 0 || len(sids) == 0 {
		return result, nil
	}
	cursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeProblemSolution,
		"uid":      uid,
		"docId":    bson.M{"$in": sids},
	}, options.Find().SetProjection(bson.M{"docId": 1, "vote": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var status struct {
			DocID bson.ObjectID `bson:"docId"`
			Vote  int64         `bson:"vote"`
		}
		if err := cursor.Decode(&status); err != nil {
			return nil, err
		}
		result[status.DocID.Hex()] = status.Vote
	}
	return result, cursor.Err()
}

// AddSolutionReply appends a nested reply.
func (r *Repository) AddSolutionReply(ctx context.Context, sid bson.ObjectID, owner int64, content string) (*Solution, error) {
	reply := SolutionReply{ID: bson.NewObjectID(), Owner: owner, Content: content}
	result := r.coll.FindOneAndUpdate(ctx, r.solutionFilter(bson.M{"_id": sid}),
		bson.M{"$push": bson.M{"reply": reply}}, options.FindOneAndUpdate().SetReturnDocument(options.After))
	var item Solution
	if err := result.Decode(&item); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item.touch()
	return &item, nil
}

// UpdateSolutionReply replaces the content of one reply.
func (r *Repository) UpdateSolutionReply(ctx context.Context, sid, rid bson.ObjectID, content string) error {
	result, err := r.coll.UpdateOne(ctx, r.solutionFilter(bson.M{"_id": sid, "reply._id": rid}),
		bson.M{"$set": bson.M{"reply.$.content": content, "reply.$.edited": true}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSolutionReply removes one reply.
func (r *Repository) DeleteSolutionReply(ctx context.Context, sid, rid bson.ObjectID) error {
	result, err := r.coll.UpdateOne(ctx, r.solutionFilter(bson.M{"_id": sid}),
		bson.M{"$pull": bson.M{"reply": bson.M{"_id": rid}}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 || result.ModifiedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// SolutionStats reports how many solutions a user wrote and how many votes they
// received, which the profile page shows as "liked solutions".
func (r *Repository) SolutionStats(ctx context.Context, uid int64) (int64, int64, error) {
	cursor, err := r.coll.Find(ctx, r.solutionFilter(bson.M{"owner": uid}),
		options.Find().SetProjection(bson.M{"vote": 1}))
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)
	var count, liked int64
	for cursor.Next(ctx) {
		var item struct {
			Vote int64 `bson:"vote"`
		}
		if err := cursor.Decode(&item); err != nil {
			return 0, 0, err
		}
		count++
		liked += item.Vote
	}
	return count, liked, cursor.Err()
}
