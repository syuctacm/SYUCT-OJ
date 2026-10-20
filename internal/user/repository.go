package user

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID          int64     `bson:"_id" json:"id"`
	Mail        string    `bson:"mail" json:"email"`
	MailLower   string    `bson:"mailLower" json:"-"`
	Uname       string    `bson:"uname" json:"username"`
	UnameLower  string    `bson:"unameLower" json:"-"`
	Salt        string    `bson:"salt" json:"-"`
	Hash        string    `bson:"hash" json:"-"`
	HashType    string    `bson:"hashType" json:"-"`
	Priv        int64     `bson:"priv" json:"priv"`
	RegAt       time.Time `bson:"regat" json:"regAt"`
	LoginAt     time.Time `bson:"loginat" json:"loginAt"`
	DisplayName string    `bson:"displayName" json:"displayName,omitempty"`
	StudentID   string    `bson:"studentId" json:"studentId,omitempty"`
	Avatar      string    `bson:"avatar" json:"avatar,omitempty"`
	Bio         string    `bson:"bio" json:"bio,omitempty"`
}

type Group struct {
	ID       bson.ObjectID `bson:"_id" json:"id"`
	DomainID string        `bson:"domainId" json:"-"`
	Name     string        `bson:"name" json:"name"`
	UIDs     []int64       `bson:"uids" json:"uids"`
}

type Repository struct {
	collection *mongo.Collection
	records    *mongo.Collection
	groups     *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		collection: db.Collection("user"),
		records:    db.Collection("record"),
		groups:     db.Collection("user.group"),
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
