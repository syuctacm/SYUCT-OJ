package discussion

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DocumentTypeProblem         = 10
	DocumentTypeDiscussionNode  = 20
	DocumentTypeDiscussion      = 21
	DocumentTypeDiscussionReply = 22
	DocumentTypeContest         = 30
	DocumentTypeTraining        = 40
	DomainSystem                = "system"
	MaxTitleLength              = 200
	MaxContentLength            = 64 * 1024
	MaxReplyLength              = 64 * 1024
)

var (
	ErrNotFound     = fmt.Errorf("discussion not found")
	ErrNodeNotFound = fmt.Errorf("discussion node not found")
	ErrNodeExists   = fmt.Errorf("discussion node already exists")
	ErrNodeInUse    = fmt.Errorf("discussion node is still in use")
)

type NodeInput struct {
	ID       string `json:"docId"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Pic      string `json:"pic"`
}

type Discussion struct {
	ID          bson.ObjectID               `bson:"_id" json:"id"`
	DocID       bson.ObjectID               `bson:"docId" json:"docId"`
	Title       string                      `bson:"title" json:"title"`
	Content     string                      `bson:"content" json:"content,omitempty"`
	Owner       int64                       `bson:"owner" json:"owner"`
	UpdateAt    time.Time                   `bson:"updateAt" json:"updateAt"`
	NReply      int64                       `bson:"nReply" json:"nReply"`
	Views       int64                       `bson:"views" json:"views"`
	Pin         bool                        `bson:"pin" json:"pin"`
	Highlight   bool                        `bson:"highlight" json:"highlight"`
	Lock        bool                        `bson:"lock" json:"lock"`
	Edited      bool                        `bson:"edited" json:"edited"`
	ParentType  int64                       `bson:"parentType" json:"parentType"`
	ParentID    any                         `bson:"parentId" json:"parentId,omitempty"`
	ParentTitle string                      `bson:"-" json:"parentTitle,omitempty"`
	Starred     bool                        `bson:"-" json:"starred"`
	React       map[string]int64            `bson:"react,omitempty" json:"react,omitempty"`
	Reactions   map[string]map[string]int64 `bson:"-" json:"reactions,omitempty"`
}

type Node struct {
	DocID    string `bson:"docId" json:"docId"`
	Title    string `bson:"title" json:"title"`
	Category string `bson:"content" json:"category"`
	Pic      string `bson:"pic" json:"pic,omitempty"`
	Type     int64  `bson:"-" json:"type,omitempty"`
	Owner    int64  `bson:"owner" json:"owner,omitempty"`
	Rule     string `bson:"-" json:"rule,omitempty"`
}

type TailReply struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	Owner     int64         `bson:"owner" json:"owner"`
	Content   string        `bson:"content" json:"content"`
	Edited    bool          `bson:"edited" json:"edited"`
	CreatedAt time.Time     `bson:"-" json:"createdAt"`
}

type Reply struct {
	ID        bson.ObjectID    `bson:"_id" json:"id"`
	DocID     bson.ObjectID    `bson:"docId" json:"docId"`
	ParentID  bson.ObjectID    `bson:"parentId" json:"parentId"`
	Owner     int64            `bson:"owner" json:"owner"`
	Content   string           `bson:"content" json:"content"`
	Edited    bool             `bson:"edited" json:"edited"`
	Replies   []TailReply      `bson:"reply" json:"replies,omitempty"`
	React     map[string]int64 `bson:"react,omitempty" json:"react,omitempty"`
	CreatedAt time.Time        `bson:"-" json:"createdAt"`
}

type Repository struct {
	coll     *mongo.Collection
	statuses *mongo.Collection
	history  *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		coll:     db.Collection("document"),
		statuses: db.Collection("document.status"),
		history:  db.Collection("discussion.history"),
	}
}

// MarkViewed records the first view of a discussion by a signed-in user.
// Hydro keeps this marker in document.status, so one user only contributes one
// view even if the page is refreshed repeatedly.
func (r *Repository) MarkViewed(ctx context.Context, did bson.ObjectID, uid int64) (bool, error) {
	if uid <= 0 {
		return false, nil
	}
	base := bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"docId":    did,
		"uid":      uid,
	}
	result, err := r.statuses.UpdateOne(ctx,
		bson.M{
			"domainId": DomainSystem,
			"docType":  DocumentTypeDiscussion,
			"docId":    did,
			"uid":      uid,
			"view":     bson.M{"$ne": true},
		},
		bson.M{"$set": bson.M{"view": true}},
	)
	if err != nil {
		return false, err
	}
	if result.MatchedCount > 0 {
		return true, nil
	}

	// A status document may already exist with view=true. Do not create a
	// second status document for the same user in that case.
	count, err := r.statuses.CountDocuments(ctx, base)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	// The deterministic _id makes concurrent first views safe even when the
	// database was created without a unique status index.
	doc := bson.M{
		"_id":      fmt.Sprintf("discussion-view:%s:%d", did.Hex(), uid),
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"docId":    did,
		"uid":      uid,
		"view":     true,
	}
	if _, err := r.statuses.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// IncrementViews atomically increases the discussion view counter.
func (r *Repository) IncrementViews(ctx context.Context, did bson.ObjectID) error {
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"_id": did, "domainId": DomainSystem, "docType": DocumentTypeDiscussion,
	}, bson.M{"$inc": bson.M{"views": 1}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// Starred reports whether the user starred the discussion.
func (r *Repository) Starred(ctx context.Context, did bson.ObjectID, uid int64) (bool, error) {
	if uid <= 0 {
		return false, nil
	}
	count, err := r.statuses.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"docId":    did,
		"uid":      uid,
		"star":     true,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetStar stores the star flag on the user's discussion status document.
func (r *Repository) SetStar(ctx context.Context, did bson.ObjectID, uid int64, star bool) error {
	if uid <= 0 {
		return ErrNotFound
	}
	_, err := r.statuses.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"docId":    did,
		"uid":      uid,
	}, bson.M{"$set": bson.M{"star": star}}, options.UpdateOne().SetUpsert(true))
	return err
}

// React toggles one emoji reaction on a discussion or reply document and
// returns the document counters plus the caller's own reactions. The semantics
// mirror Hydro's discussion.react: the user status document keeps the flag and
// the document keeps the aggregate count.
func (r *Repository) React(ctx context.Context, docType int64, id bson.ObjectID, uid int64, emoji string, reverse bool) (map[string]int64, map[string]int64, error) {
	if uid <= 0 {
		return nil, nil, ErrNotFound
	}
	filter := bson.M{
		"domainId": DomainSystem,
		"docType":  docType,
		"docId":    id,
		"uid":      uid,
	}
	var sdoc struct {
		React map[string]int64 `bson:"react"`
	}
	if err := r.statuses.FindOne(ctx, filter).Decode(&sdoc); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil, err
	}
	self := sdoc.React
	if self == nil {
		self = map[string]int64{}
	}
	field := "react." + emoji
	current := self[emoji] > 0
	if current != !reverse {
		if reverse {
			if _, err := r.statuses.UpdateOne(ctx, filter, bson.M{"$unset": bson.M{field: ""}}); err != nil {
				return nil, nil, err
			}
			delete(self, emoji)
		} else {
			if _, err := r.statuses.UpdateOne(ctx, filter, bson.M{"$set": bson.M{field: int64(1)}}, options.UpdateOne().SetUpsert(true)); err != nil {
				return nil, nil, err
			}
			self[emoji] = 1
		}
		delta := int64(1)
		if reverse {
			delta = -1
		}
		if _, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{field: delta}}); err != nil {
			return nil, nil, err
		}
		if reverse {
			_, _ = r.coll.UpdateOne(ctx, bson.M{"_id": id, field: bson.M{"$lte": 0}}, bson.M{"$unset": bson.M{field: ""}})
		}
	}
	var doc struct {
		React map[string]int64 `bson:"react"`
	}
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		return nil, nil, err
	}
	if doc.React == nil {
		doc.React = map[string]int64{}
	}
	if len(doc.React) == 0 {
		// Keep the document clean once the last reaction is removed.
		_, _ = r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$unset": bson.M{"react": ""}})
	}
	return doc.React, self, nil
}

// ReplyBelongsTo reports whether the reply document belongs to the discussion.
func (r *Repository) ReplyBelongsTo(ctx context.Context, discussionID, replyID bson.ObjectID) (bool, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeDiscussionReply,
		"_id":        replyID,
		"parentType": DocumentTypeDiscussion,
		"parentId":   discussionID,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Reactions loads the caller's own reaction maps for the given documents,
// keyed by document id, so templates can mark reactions as active.
func (r *Repository) Reactions(ctx context.Context, uid int64, ids []bson.ObjectID) (map[string]map[string]int64, error) {
	result := map[string]map[string]int64{}
	if uid <= 0 || len(ids) == 0 {
		return result, nil
	}
	cursor, err := r.statuses.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"uid":      uid,
		"docId":    bson.M{"$in": ids},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var sdoc struct {
			DocID bson.ObjectID    `bson:"docId"`
			React map[string]int64 `bson:"react"`
		}
		if err := cursor.Decode(&sdoc); err != nil {
			return nil, err
		}
		if len(sdoc.React) > 0 {
			result[sdoc.DocID.Hex()] = sdoc.React
		}
	}
	return result, cursor.Err()
}

// HistoryEntry is one stored revision of a discussion, reply or tail reply.
type HistoryEntry struct {
	UID     int64     `bson:"uid" json:"uid"`
	Time    time.Time `bson:"time" json:"time"`
	Content string    `bson:"content" json:"content,omitempty"`
}

// addHistory records a revision. History is best effort: losing one entry must
// never fail the write that produced it.
func (r *Repository) addHistory(ctx context.Context, docID bson.ObjectID, uid int64, content string) {
	if r.history == nil || docID.IsZero() {
		return
	}
	_, _ = r.history.InsertOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docId":    docID,
		"uid":      uid,
		"content":  content,
		"time":     time.Now(),
	})
}

// History lists revisions of one document newest first. When at is set only the
// revision with that exact timestamp is returned.
func (r *Repository) History(ctx context.Context, docID bson.ObjectID, at *time.Time) ([]HistoryEntry, error) {
	filter := bson.M{"domainId": DomainSystem, "docId": docID}
	if at != nil {
		filter["time"] = *at
	}
	cursor, err := r.history.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "time", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]HistoryEntry, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// TargetContent returns the current content of a discussion, a reply or a tail
// reply, and reports whether the id belongs to the discussion.
func (r *Repository) TargetContent(ctx context.Context, discussionID, targetID bson.ObjectID) (string, error) {
	item, err := r.Get(ctx, discussionID.Hex())
	if err != nil {
		return "", err
	}
	if item.DocID == targetID {
		return item.Content, nil
	}
	reply, tail, _, err := r.findReply(ctx, discussionID, targetID)
	if err != nil {
		return "", err
	}
	if tail != nil {
		return tail.Content, nil
	}
	return reply.Content, nil
}

func (r *Repository) List(ctx context.Context) ([]Discussion, error) {
	items, _, err := r.ListFiltered(ctx, 0, "", 1, 1000)
	return items, err
}

// ListFiltered lists discussions, optionally limited to one parent
// (parentType/parentId), which is how the node pages scope their list.
func (r *Repository) ListFiltered(ctx context.Context, parentType int64, parentID string, page, pageSize int) ([]Discussion, int64, error) {
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
		"docType":  DocumentTypeDiscussion,
	}
	if parentType > 0 {
		filter["parentType"] = parentType
	}
	if parentID = strings.TrimSpace(parentID); parentID != "" {
		switch parentType {
		case DocumentTypeProblem:
			if value, err := strconv.ParseInt(parentID, 10, 64); err == nil {
				filter["parentId"] = value
			} else {
				filter["parentId"] = parentID
			}
		case DocumentTypeContest, DocumentTypeTraining:
			if oid, err := bson.ObjectIDFromHex(parentID); err == nil {
				filter["parentId"] = oid
			} else {
				filter["parentId"] = parentID
			}
		default:
			filter["parentId"] = parentID
		}
	}
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "owner": 1, "updateAt": 1, "nReply": 1, "views": 1, "pin": 1, "highlight": 1, "lock": 1, "edited": 1, "parentType": 1, "parentId": 1}).
		SetSort(bson.D{{Key: "pin", Value: -1}, {Key: "updateAt", Value: -1}, {Key: "docId", Value: -1}}).
		SetSkip(int64((page-1)*pageSize)).
		SetLimit(int64(pageSize)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	items := make([]Discussion, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	nodes, err := r.ListNodes(ctx)
	if err != nil {
		return nil, 0, err
	}
	titles := make(map[string]string, len(nodes))
	for _, node := range nodes {
		titles[node.DocID] = node.DisplayTitle()
	}
	for i := range items {
		if items[i].ParentType == DocumentTypeDiscussionNode {
			if nodeID, ok := items[i].ParentID.(string); ok {
				items[i].ParentTitle = titles[nodeID]
			}
		}
	}
	return items, total, nil
}

func (r *Repository) ListNodes(ctx context.Context) ([]Node, error) {
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussionNode,
	}, options.Find().
		SetProjection(bson.M{"docId": 1, "title": 1, "content": 1, "pic": 1}).
		SetSort(bson.D{{Key: "content", Value: 1}, {Key: "docId", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Node, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) CreateNode(ctx context.Context, owner int64, input NodeInput) (*Node, error) {
	_, err := r.coll.InsertOne(ctx, bson.M{
		"_id": input.ID, "domainId": DomainSystem, "docType": DocumentTypeDiscussionNode,
		"docId": input.ID, "title": input.Title, "content": input.Category,
		"pic": input.Pic, "owner": owner,
	})
	if mongo.IsDuplicateKeyError(err) {
		return nil, ErrNodeExists
	}
	if err != nil {
		return nil, err
	}
	return r.GetNode(ctx, input.ID)
}

func (r *Repository) UpdateNode(ctx context.Context, nodeID string, input NodeInput) (*Node, error) {
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeDiscussionNode,
		"$or": []bson.M{{"docId": nodeID}, {"_id": nodeID}},
	}, bson.M{"$set": bson.M{
		"title": input.Title, "content": input.Category, "pic": input.Pic,
	}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, ErrNodeNotFound
	}
	return r.GetNode(ctx, nodeID)
}

func (r *Repository) DeleteNode(ctx context.Context, nodeID string) error {
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeDiscussion,
		"parentType": DocumentTypeDiscussionNode, "parentId": nodeID,
	})
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrNodeInUse
	}
	result, err := r.coll.DeleteOne(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeDiscussionNode,
		"$or": []bson.M{{"docId": nodeID}, {"_id": nodeID}},
	})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNodeNotFound
	}
	return nil
}

func (r *Repository) GetNode(ctx context.Context, nodeID string) (*Node, error) {
	if nodeID == "" {
		return nil, ErrNodeNotFound
	}
	var node Node
	err := r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussionNode,
		"$or":      []bson.M{{"docId": nodeID}, {"_id": nodeID}},
	}).Decode(&node)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// ResolveNode resolves both global discussion nodes and virtual nodes backed by
// problems, contests/homework or trainings, mirroring Hydro.getVnode.
func (r *Repository) ResolveNode(ctx context.Context, nodeType, nodeID string) (*Node, error) {
	nodeType = strings.TrimSpace(nodeType)
	nodeID = strings.TrimSpace(nodeID)
	if nodeType == "" || nodeType == "node" {
		node, err := r.GetNode(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		node.Type = DocumentTypeDiscussionNode
		return node, nil
	}
	var docType int64
	switch nodeType {
	case "contest", "homework":
		docType = DocumentTypeContest
	case "training":
		docType = DocumentTypeTraining
	case "problem":
		docType = DocumentTypeProblem
	default:
		return nil, ErrNodeNotFound
	}
	filter := bson.M{"domainId": DomainSystem, "docType": docType}
	if docType == DocumentTypeProblem {
		or := []bson.M{{"pid": nodeID}}
		if value, err := strconv.ParseInt(nodeID, 10, 64); err == nil {
			or = append(or, bson.M{"docId": value})
		}
		filter["$or"] = or
	} else {
		oid, err := bson.ObjectIDFromHex(nodeID)
		if err != nil {
			return nil, ErrNodeNotFound
		}
		filter["$or"] = []bson.M{{"docId": oid}, {"_id": oid}}
	}
	var raw bson.M
	if err := r.coll.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{
		"docId": 1, "title": 1, "content": 1, "owner": 1, "pic": 1, "rule": 1,
	})).Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	if docType == DocumentTypeContest {
		rule, _ := raw["rule"].(string)
		if nodeType == "homework" && rule != "homework" {
			return nil, ErrNodeNotFound
		}
		if nodeType == "contest" && rule == "homework" {
			return nil, ErrNodeNotFound
		}
	}
	var docID string
	switch value := raw["docId"].(type) {
	case bson.ObjectID:
		docID = value.Hex()
	case int64:
		docID = strconv.FormatInt(value, 10)
	case int32:
		docID = strconv.FormatInt(int64(value), 10)
	case int:
		docID = strconv.Itoa(value)
	case string:
		docID = value
	default:
		docID = fmt.Sprint(value)
	}
	var owner int64
	switch value := raw["owner"].(type) {
	case int64:
		owner = value
	case int32:
		owner = int64(value)
	case int:
		owner = int64(value)
	case float64:
		owner = int64(value)
	}
	title, _ := raw["title"].(string)
	category, _ := raw["content"].(string)
	pic, _ := raw["pic"].(string)
	rule, _ := raw["rule"].(string)
	return &Node{
		DocID: docID, Title: title, Category: category, Pic: pic,
		Type: docType, Owner: owner, Rule: rule,
	}, nil
}

func (r *Repository) Create(ctx context.Context, node *Node, owner int64, title, content string, pin, highlight bool) (*Discussion, error) {
	now := time.Now()
	did := bson.NewObjectID()
	parentType := node.Type
	if parentType == 0 {
		parentType = DocumentTypeDiscussionNode
	}
	var parentID any = node.DocID
	if parentType == DocumentTypeProblem {
		parentID, _ = strconv.ParseInt(node.DocID, 10, 64)
	} else if parentType == DocumentTypeContest || parentType == DocumentTypeTraining {
		parentID, _ = bson.ObjectIDFromHex(node.DocID)
	}
	item := &Discussion{
		ID: did, DocID: did, Title: title, Content: content, Owner: owner,
		UpdateAt: now, Pin: pin, Highlight: highlight,
		ParentType: parentType, ParentID: parentID,
		ParentTitle: node.DisplayTitle(),
	}
	_, err := r.coll.InsertOne(ctx, bson.M{
		"_id": did, "domainId": DomainSystem, "docType": DocumentTypeDiscussion,
		"docId": did, "title": title, "content": content, "owner": owner,
		"editor": owner, "parentType": parentType,
		"parentId": parentID, "nReply": 0, "views": 0,
		"pin": pin, "highlight": highlight, "hidden": false,
		"updateAt": now, "sort": 100,
	})
	if err != nil {
		return nil, err
	}
	r.addHistory(ctx, did, owner, content)
	return item, nil
}

type EditInput struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Pin       bool   `json:"pin"`
	Highlight bool   `json:"highlight"`
	// Editor is filled by the handler and never read from the request body.
	Editor int64 `json:"-"`
}

func (r *Repository) Update(ctx context.Context, did string, input EditInput) (*Discussion, error) {
	oid, err := bson.ObjectIDFromHex(did)
	if err != nil {
		return nil, ErrNotFound
	}
	now := time.Now()
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}, bson.M{"$set": bson.M{
		"title": strings.TrimSpace(input.Title), "content": input.Content,
		"pin": input.Pin, "highlight": input.Highlight,
		"edited": true, "updateAt": now,
	}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, ErrNotFound
	}
	r.addHistory(ctx, oid, input.Editor, input.Content)
	return r.Get(ctx, did)
}

func (r *Repository) SetLock(ctx context.Context, did string, lock bool) (*Discussion, error) {
	oid, err := bson.ObjectIDFromHex(did)
	if err != nil {
		return nil, ErrNotFound
	}
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}, bson.M{"$set": bson.M{"lock": lock}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, did)
}

func (r *Repository) Delete(ctx context.Context, did string) error {
	oid, err := bson.ObjectIDFromHex(did)
	if err != nil {
		return ErrNotFound
	}
	result, err := r.coll.DeleteOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	_, err = r.coll.DeleteMany(ctx, bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeDiscussionReply,
		"parentType": DocumentTypeDiscussion,
		"parentId":   oid,
	})
	return err
}

func (r *Repository) Get(ctx context.Context, did string) (*Discussion, error) {
	oid, err := bson.ObjectIDFromHex(did)
	if err != nil {
		return nil, ErrNotFound
	}
	var item Discussion
	err = r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
		"$or":      []bson.M{{"docId": oid}, {"_id": oid}},
	}).Decode(&item)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) ListReplies(ctx context.Context, discussionID bson.ObjectID) ([]Reply, error) {
	cursor, err := r.coll.Find(ctx, bson.M{
		"domainId":   DomainSystem,
		"docType":    DocumentTypeDiscussionReply,
		"parentType": DocumentTypeDiscussion,
		"parentId":   discussionID,
	}, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Reply, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CreatedAt = items[i].ID.Timestamp()
		for j := range items[i].Replies {
			items[i].Replies[j].CreatedAt = items[i].Replies[j].ID.Timestamp()
		}
	}
	return items, nil
}

func (node Node) DisplayTitle() string {
	if node.Title != "" {
		return node.Title
	}
	return node.DocID
}

func (r *Repository) UpdateReply(ctx context.Context, discussionID, replyID bson.ObjectID, editor int64, content string) (*Reply, *TailReply, error) {
	reply, tail, isTail, err := r.findReply(ctx, discussionID, replyID)
	if err != nil {
		return nil, nil, err
	}
	if !isTail {
		result, err := r.coll.UpdateOne(ctx, bson.M{"_id": reply.ID, "docType": DocumentTypeDiscussionReply}, bson.M{"$set": bson.M{
			"content": content, "edited": true,
		}})
		if err != nil {
			return nil, nil, err
		}
		if result.MatchedCount == 0 {
			return nil, nil, ErrNotFound
		}
		reply.Content = content
		reply.Edited = true
		r.addHistory(ctx, reply.ID, editor, content)
		return reply, nil, nil
	}
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"_id": reply.ID, "docType": DocumentTypeDiscussionReply, "reply._id": tail.ID,
	}, bson.M{"$set": bson.M{
		"reply.$.content": content, "reply.$.edited": true,
	}})
	if err != nil {
		return nil, nil, err
	}
	if result.MatchedCount == 0 {
		return nil, nil, ErrNotFound
	}
	tail.Content = content
	tail.Edited = true
	r.addHistory(ctx, tail.ID, editor, content)
	return reply, tail, nil
}

func (r *Repository) DeleteReply(ctx context.Context, discussionID, replyID bson.ObjectID) error {
	reply, tail, isTail, err := r.findReply(ctx, discussionID, replyID)
	if err != nil {
		return err
	}
	if isTail {
		result, err := r.coll.UpdateOne(ctx, bson.M{
			"_id": reply.ID, "docType": DocumentTypeDiscussionReply, "reply._id": tail.ID,
		}, bson.M{"$pull": bson.M{"reply": bson.M{"_id": tail.ID}}})
		if err != nil {
			return err
		}
		if result.MatchedCount == 0 {
			return ErrNotFound
		}
		return nil
	}
	result, err := r.coll.DeleteOne(ctx, bson.M{"_id": reply.ID, "docType": DocumentTypeDiscussionReply})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": discussionID, "docType": DocumentTypeDiscussion}, bson.M{
		"$inc": bson.M{"nReply": -1},
		"$set": bson.M{"updateAt": time.Now()},
	})
	return err
}

func (r *Repository) ReplyOwner(ctx context.Context, discussionID, replyID bson.ObjectID) (int64, error) {
	reply, tail, isTail, err := r.findReply(ctx, discussionID, replyID)
	if err != nil {
		return 0, err
	}
	if isTail {
		return tail.Owner, nil
	}
	return reply.Owner, nil
}

func (r *Repository) findReply(ctx context.Context, discussionID, replyID bson.ObjectID) (*Reply, *TailReply, bool, error) {
	var reply Reply
	err := r.coll.FindOne(ctx, bson.M{
		"_id": replyID, "domainId": DomainSystem, "docType": DocumentTypeDiscussionReply,
		"parentType": DocumentTypeDiscussion, "parentId": discussionID,
	}).Decode(&reply)
	if err == nil {
		reply.CreatedAt = reply.ID.Timestamp()
		for i := range reply.Replies {
			reply.Replies[i].CreatedAt = reply.Replies[i].ID.Timestamp()
		}
		return &reply, nil, false, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, nil, false, err
	}
	err = r.coll.FindOne(ctx, bson.M{
		"domainId": DomainSystem, "docType": DocumentTypeDiscussionReply,
		"parentType": DocumentTypeDiscussion, "parentId": discussionID,
		"reply._id": replyID,
	}).Decode(&reply)
	if err == mongo.ErrNoDocuments {
		return nil, nil, false, ErrNotFound
	}
	if err != nil {
		return nil, nil, false, err
	}
	for i := range reply.Replies {
		if reply.Replies[i].ID == replyID {
			return &reply, &reply.Replies[i], true, nil
		}
	}
	return nil, nil, false, ErrNotFound
}

func (r *Repository) AddReply(ctx context.Context, discussionID bson.ObjectID, owner int64, content string) (*Reply, error) {
	now := time.Now()
	replyID := bson.NewObjectID()
	item := Reply{
		ID:        replyID,
		DocID:     replyID,
		ParentID:  discussionID,
		Owner:     owner,
		Content:   content,
		CreatedAt: now,
	}
	_, err := r.coll.InsertOne(ctx, bson.M{
		"_id":        replyID,
		"domainId":   DomainSystem,
		"docType":    DocumentTypeDiscussionReply,
		"docId":      replyID,
		"parentType": DocumentTypeDiscussion,
		"parentId":   discussionID,
		"owner":      owner,
		"content":    content,
		"edited":     false,
	})
	if err != nil {
		return nil, err
	}
	result, err := r.coll.UpdateOne(ctx, bson.M{
		"_id":      discussionID,
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
	}, bson.M{
		"$inc": bson.M{"nReply": 1},
		"$set": bson.M{"updateAt": now},
	})
	if err != nil {
		_, _ = r.coll.DeleteOne(ctx, bson.M{"_id": replyID})
		return nil, err
	}
	if result.MatchedCount == 0 {
		_, _ = r.coll.DeleteOne(ctx, bson.M{"_id": replyID})
		return nil, ErrNotFound
	}
	r.addHistory(ctx, item.ID, owner, content)
	return &item, nil
}

// AddTailReply appends a nested reply under an existing top-level reply.
func (r *Repository) AddTailReply(ctx context.Context, discussionID, replyID bson.ObjectID, owner int64, content string) (*Reply, *TailReply, error) {
	now := time.Now()
	tail := TailReply{
		ID:        bson.NewObjectID(),
		Owner:     owner,
		Content:   content,
		CreatedAt: now,
	}
	result := r.coll.FindOneAndUpdate(ctx, bson.M{
		"_id": replyID, "domainId": DomainSystem, "docType": DocumentTypeDiscussionReply,
		"parentType": DocumentTypeDiscussion, "parentId": discussionID,
	}, bson.M{"$push": bson.M{"reply": tail}}, options.FindOneAndUpdate().SetReturnDocument(options.After))
	var reply Reply
	if err := result.Decode(&reply); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	reply.CreatedAt = reply.ID.Timestamp()
	for i := range reply.Replies {
		reply.Replies[i].CreatedAt = reply.Replies[i].ID.Timestamp()
	}
	if _, err := r.coll.UpdateOne(ctx, bson.M{
		"_id":      discussionID,
		"domainId": DomainSystem,
		"docType":  DocumentTypeDiscussion,
	}, bson.M{"$set": bson.M{"updateAt": now}}); err != nil {
		return nil, nil, err
	}
	r.addHistory(ctx, tail.ID, owner, content)
	return &reply, &tail, nil
}
