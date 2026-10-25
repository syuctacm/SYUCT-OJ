package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/filemeta"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID              int64           `bson:"_id" json:"id"`
	Mail            string          `bson:"mail" json:"email"`
	MailLower       string          `bson:"mailLower" json:"-"`
	Uname           string          `bson:"uname" json:"username"`
	UnameLower      string          `bson:"unameLower" json:"-"`
	Salt            string          `bson:"salt" json:"-"`
	Hash            string          `bson:"hash" json:"-"`
	HashType        string          `bson:"hashType" json:"-"`
	Priv            int64           `bson:"priv" json:"priv"`
	RegAt           time.Time       `bson:"regat" json:"regAt"`
	LoginAt         time.Time       `bson:"loginat" json:"loginAt"`
	DisplayName     string          `bson:"displayName" json:"displayName,omitempty"`
	StudentID       string          `bson:"studentId" json:"studentId,omitempty"`
	Avatar          string          `bson:"avatar" json:"avatar,omitempty"`
	Bio             string          `bson:"bio" json:"bio,omitempty"`
	QQ              string          `bson:"qq,omitempty" json:"qq,omitempty"`
	Wechat          string          `bson:"wechat,omitempty" json:"wechat,omitempty"`
	Gender          *int64          `bson:"gender,omitempty" json:"gender,omitempty"`
	School          string          `bson:"school,omitempty" json:"school,omitempty"`
	BackgroundImage string          `bson:"backgroundImage,omitempty" json:"backgroundImage,omitempty"`
	TFA             string          `bson:"tfa,omitempty" json:"-"`
	CheckinTime     string          `bson:"checkin_time,omitempty" json:"-"`
	CheckinLuck     int64           `bson:"checkin_luck,omitempty" json:"-"`
	CheckinCountNow int64           `bson:"checkin_cnt_now,omitempty" json:"-"`
	CheckinCountAll int64           `bson:"checkin_cnt_all,omitempty" json:"-"`
	Files           []filemeta.File `bson:"_files,omitempty" json:"-"`
}

type Group struct {
	ID       bson.ObjectID `bson:"_id" json:"id"`
	DomainID string        `bson:"domainId" json:"-"`
	Name     string        `bson:"name" json:"name"`
	UIDs     []int64       `bson:"uids" json:"uids"`
}

type Repository struct {
	collection  *mongo.Collection
	records     *mongo.Collection
	groups      *mongo.Collection
	documents   *mongo.Collection
	domainUsers *mongo.Collection
	tokens      *mongo.Collection
	ipLogins    *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		collection:  db.Collection("user"),
		records:     db.Collection("record"),
		groups:      db.Collection("user.group"),
		documents:   db.Collection("document"),
		domainUsers: db.Collection("domain.user"),
		tokens:      db.Collection("token"),
		ipLogins:    db.Collection("iplogin"),
	}
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]User, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []User{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	escaped := regexp.QuoteMeta(query)
	or := []bson.M{
		{"unameLower": bson.M{"$regex": "^" + escaped, "$options": "i"}},
		{"displayName": bson.M{"$regex": escaped, "$options": "i"}},
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		or = append(or, bson.M{"_id": id})
	}
	cursor, err := r.collection.Find(ctx, bson.M{"$or": or}, options.Find().
		SetProjection(bson.M{"_id": 1, "uname": 1, "displayName": 1, "avatar": 1}).
		SetSort(bson.D{{Key: "unameLower", Value: 1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]User, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FindByIDs(ctx context.Context, ids []int64) ([]User, error) {
	if len(ids) == 0 {
		return []User{}, nil
	}
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return []User{}, nil
	}
	cursor, err := r.collection.Find(ctx, bson.M{"_id": bson.M{"$in": unique}}, options.Find().
		SetProjection(bson.M{"_id": 1, "uname": 1, "displayName": 1, "avatar": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]User, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	byID := make(map[int64]User, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ordered := make([]User, 0, len(items))
	for _, id := range unique {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
}

func (r *Repository) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []Group{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	cursor, err := r.groups.Find(ctx, bson.M{
		"domainId": "system",
		"name":     bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"},
	}, options.Find().
		SetProjection(bson.M{"name": 1, "uids": 1}).
		SetSort(bson.D{{Key: "name", Value: 1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Group, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FindGroupsByNames(ctx context.Context, names []string) ([]Group, error) {
	if len(names) == 0 {
		return []Group{}, nil
	}
	unique := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		unique = append(unique, name)
	}
	if len(unique) == 0 {
		return []Group{}, nil
	}
	cursor, err := r.groups.Find(ctx, bson.M{
		"domainId": "system",
		"name":     bson.M{"$in": unique},
	}, options.Find().SetProjection(bson.M{"name": 1, "uids": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]Group, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	order := make(map[string]int, len(unique))
	for index, name := range unique {
		order[name] = index
	}
	sort.SliceStable(items, func(i, j int) bool { return order[items[i].Name] < order[items[j].Name] })
	return items, nil
}

func (r *Repository) FindByLogin(ctx context.Context, identifier string) (*User, error) {
	lower := strings.ToLower(strings.TrimSpace(identifier))
	if lower == "" {
		return nil, ErrNotFound
	}
	var u User
	err := r.collection.FindOne(ctx, bson.M{
		"$or": []bson.M{
			{"unameLower": lower},
			{"mailLower": lower},
			{"uname": identifier},
			{"mail": identifier},
		},
	}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*User, error) {
	var u User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Files lists the files a user uploaded to their own storage area.
func (r *Repository) Files(ctx context.Context, uid int64) ([]filemeta.File, error) {
	var doc struct {
		Files []filemeta.File `bson:"_files"`
	}
	if err := r.collection.FindOne(ctx, bson.M{"_id": uid}).Decode(&doc); err != nil {
		return nil, err
	}
	files := append([]filemeta.File(nil), doc.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

// UpsertFile stores or replaces one entry of the user's `_files` list.
func (r *Repository) UpsertFile(ctx context.Context, uid int64, file filemeta.File) error {
	if err := filemeta.Upsert(ctx, r.collection, bson.M{"_id": uid}, "_files", file); err != nil {
		if err == filemeta.ErrNotFound {
			return fmt.Errorf("user not found")
		}
		return err
	}
	return nil
}

// DeleteFiles removes entries from the user's `_files` list.
func (r *Repository) DeleteFiles(ctx context.Context, uid int64, names []string) error {
	if err := filemeta.Delete(ctx, r.collection, bson.M{"_id": uid}, "_files", names); err != nil {
		if err == filemeta.ErrNotFound {
			return fmt.Errorf("user not found")
		}
		return err
	}
	return nil
}

// UpdateSettings applies a validated $set document to the user profile.
// UpdatePassword replaces the stored hydro password hash.
func (r *Repository) UpdatePassword(ctx context.Context, uid int64, salt, hash string) error {
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid},
		bson.M{"$set": bson.M{"salt": salt, "hash": hash, "hashType": "hydro"}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// RecordLogin updates the last-login timestamp and IP shown on the profile.
func (r *Repository) RecordLogin(ctx context.Context, uid int64, ip string) error {
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid},
		bson.M{"$set": bson.M{"loginat": time.Now().UTC(), "loginip": ip}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateSettings(ctx context.Context, uid int64, set bson.M) error {
	if uid <= 0 {
		return fmt.Errorf("invalid user id")
	}
	if len(set) == 0 {
		return nil
	}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *Repository) Statistics(ctx context.Context, uid int64) (int64, int64, error) {
	submit, err := r.records.CountDocuments(ctx, bson.M{"uid": uid, "domainId": "system"})
	if err != nil {
		return 0, 0, err
	}
	result := r.records.Distinct(ctx, "pid", bson.M{"uid": uid, "domainId": "system", "status": 1})
	if err := result.Err(); err != nil {
		return 0, 0, err
	}
	var pids []any
	if err := result.Decode(&pids); err != nil {
		return 0, 0, err
	}
	return submit, int64(len(pids)), nil
}

// ProblemBrief is a compact problem reference used by the profile page.
type ProblemBrief struct {
	DocID int64  `json:"docId"`
	PID   string `json:"pid"`
	Title string `json:"title"`
}

// AcceptedProblems returns the visible problems the user has solved, ordered
// by docId, capped at limit (default 100, max 200).
func (r *Repository) AcceptedProblems(ctx context.Context, uid int64, limit int64) ([]ProblemBrief, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	result := r.records.Distinct(ctx, "pid", bson.M{"uid": uid, "domainId": "system", "status": 1})
	if err := result.Err(); err != nil {
		return nil, err
	}
	var pids []any
	if err := result.Decode(&pids); err != nil {
		return nil, err
	}
	if len(pids) == 0 {
		return []ProblemBrief{}, nil
	}
	cursor, err := r.documents.Find(ctx, bson.M{
		"domainId": "system",
		"docType":  10,
		"hidden":   bson.M{"$ne": true},
		"docId":    bson.M{"$in": pids},
	}, options.Find().
		SetProjection(bson.M{"docId": 1, "pid": 1, "title": 1}).
		SetSort(bson.D{{Key: "docId", Value: 1}}).
		SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	items := make([]ProblemBrief, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// RankingEntry is one row of the site ranking.
type RankingEntry struct {
	UID   int64   `json:"uid"`
	RP    float64 `json:"rp"`
	Level int     `json:"level"`
}

func toInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	}
	return 0, false
}

func toFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case int:
		return float64(typed)
	}
	return 0
}

// DomainStats returns the per-domain RP and level of a user (zero when absent).
func (r *Repository) DomainStats(ctx context.Context, uid int64) (float64, int, error) {
	var doc bson.M
	err := r.domainUsers.FindOne(ctx, bson.M{"domainId": "system", "uid": uid}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	level, _ := toInt64(doc["level"])
	return toFloat(doc["rp"]), int(level), nil
}

// Rank returns the 1-based ranking position of the user (0 when the user has no RP).
func (r *Repository) Rank(ctx context.Context, uid int64) (int, error) {
	rp, _, err := r.DomainStats(ctx, uid)
	if err != nil || rp <= 0 {
		return 0, err
	}
	higher, err := r.domainUsers.CountDocuments(ctx, bson.M{"domainId": "system", "rp": bson.M{"$gt": rp}})
	if err != nil {
		return 0, err
	}
	return int(higher) + 1, nil
}

// Ranking lists users with RP ordered by RP descending.
func (r *Repository) Ranking(ctx context.Context, page, limit int) ([]RankingEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	filter := bson.M{"domainId": "system", "rp": bson.M{"$gt": 0}}
	total, err := r.domainUsers.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.domainUsers.Find(ctx, filter, options.Find().
		SetProjection(bson.M{"uid": 1, "rp": 1, "level": 1}).
		SetSort(bson.D{{Key: "rp", Value: -1}, {Key: "uid", Value: 1}}).
		SetSkip(int64((page-1)*limit)).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	items := make([]RankingEntry, 0, len(docs))
	for _, doc := range docs {
		uid, ok := toInt64(doc["uid"])
		if !ok {
			continue
		}
		level, _ := toInt64(doc["level"])
		items = append(items, RankingEntry{UID: uid, RP: toFloat(doc["rp"]), Level: int(level)})
	}
	return items, total, nil
}

type ActivityDay struct {
	Submissions int `json:"submissions"`
	AC          int `json:"ac"`
}

type Activity struct {
	TotalSubmissions int                    `json:"totalSubmissions"`
	ActiveDays       int                    `json:"activeDays"`
	MaxStreak        int                    `json:"maxStreak"`
	TotalSolved      int                    `json:"totalSolved"`
	DailyStats       map[string]ActivityDay `json:"dailyStats"`
}

func (r *Repository) Activity(ctx context.Context, uid int64) (*Activity, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := today.AddDate(0, 0, -364)
	cursor, err := r.records.Find(ctx, bson.M{
		"uid": uid, "domainId": "system",
		"_id": bson.M{"$gte": bson.NewObjectIDFromTimestamp(start.UTC())},
	}, options.Find().SetProjection(bson.M{"pid": 1, "status": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var records []struct {
		ID     bson.ObjectID `bson:"_id"`
		PID    int64         `bson:"pid"`
		Status int           `bson:"status"`
	}
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}
	daily := make(map[string]ActivityDay)
	acceptedByDay := make(map[string]map[int64]bool)
	acceptedAll := make(map[int64]bool)
	for _, record := range records {
		date := record.ID.Timestamp().In(location).Format("2006-01-02")
		day := daily[date]
		day.Submissions++
		daily[date] = day
		if record.Status == 1 {
			if acceptedByDay[date] == nil {
				acceptedByDay[date] = make(map[int64]bool)
			}
			acceptedByDay[date][record.PID] = true
			acceptedAll[record.PID] = true
		}
	}
	dates := make([]string, 0, len(daily))
	activeDays := 0
	for date, day := range daily {
		day.AC = len(acceptedByDay[date])
		daily[date] = day
		if day.Submissions > 0 {
			activeDays++
		}
		dates = append(dates, date)
	}
	sort.Strings(dates)
	maxStreak := 0
	currentStreak := 0
	var previous time.Time
	for _, date := range dates {
		parsed, err := time.ParseInLocation("2006-01-02", date, location)
		if err != nil {
			continue
		}
		if !previous.IsZero() && parsed.Sub(previous) == 24*time.Hour {
			currentStreak++
		} else {
			currentStreak = 1
		}
		if currentStreak > maxStreak {
			maxStreak = currentStreak
		}
		previous = parsed
	}
	return &Activity{
		TotalSubmissions: len(records),
		ActiveDays:       activeDays,
		MaxStreak:        maxStreak,
		TotalSolved:      len(acceptedAll),
		DailyStats:       daily,
	}, nil
}
