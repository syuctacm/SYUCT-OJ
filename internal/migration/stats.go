package migration

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	statsTargetDomain = "system"
	problemDocType    = 10
	solutionDocType   = 11
	acceptedStatus    = 1
	canceledStatus    = 9
	statsBatchSize    = 200
)

var ignoredContestIDs = func() []bson.ObjectID {
	pretest, err := bson.ObjectIDFromHex("000000000000000000000000")
	if err != nil {
		panic(err)
	}
	generate, err := bson.ObjectIDFromHex("000000000000000000000001")
	if err != nil {
		panic(err)
	}
	return []bson.ObjectID{pretest, generate}
}()

type StatsOptions struct {
	Domain string
	DryRun bool
}

type StatsReport struct {
	DryRun             bool      `json:"dry_run"`
	Domain             string    `json:"domain"`
	StartedAt          time.Time `json:"started_at"`
	EndedAt            time.Time `json:"ended_at,omitempty"`
	Records            int64     `json:"records"`
	SolutionDocs       int64     `json:"solution_docs"`
	UserStatsMatched   int64     `json:"user_stats_matched"`
	LikeStatsMatched   int64     `json:"like_stats_matched"`
	ProblemsMatched    int64     `json:"problems_matched"`
	ProblemsMissing    int64     `json:"problems_missing,omitempty"`
	UsersMissing       int64     `json:"users_missing,omitempty"`
	UserDocsUpdated    int64     `json:"user_docs_updated,omitempty"`
	ProblemDocsUpdated int64     `json:"problem_docs_updated,omitempty"`
	Warnings           []string  `json:"warnings,omitempty"`
}

type userStats struct {
	Domain  string
	UID     int64
	NSubmit int64
	NAccept int64
	NLiked  int64
}

type problemStats struct {
	Domain  string
	PID     int64
	NSubmit int64
	NAccept int64
	Status  map[string]int64
	Scores  map[int]int64
}

func RunStatsRebuild(ctx context.Context, db *mongo.Database, opts StatsOptions) (*StatsReport, error) {
	domain := opts.Domain
	if domain == "" {
		domain = statsTargetDomain
	}
	if domain != statsTargetDomain {
		return nil, fmt.Errorf("stats rebuild currently supports only domain %q", statsTargetDomain)
	}
	report := &StatsReport{DryRun: opts.DryRun, Domain: domain, StartedAt: time.Now().UTC()}
	filter := recordStatsFilter(domain)

	var err error
	report.Records, err = db.Collection("record").CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("count records for stats rebuild: %w", err)
	}
	report.SolutionDocs, err = db.Collection("document").CountDocuments(ctx, solutionStatsFilter(domain))
	if err != nil {
		return nil, fmt.Errorf("count solution votes for stats rebuild: %w", err)
	}

	users, err := aggregateUserStats(ctx, db, domain, userRecordStatsFilter(domain))
	if err != nil {
		return nil, err
	}
	likes, err := aggregateLikeStats(ctx, db, domain)
	if err != nil {
		return nil, err
	}
	for key, stat := range likes {
		entry := users[key]
		if entry == nil {
			entry = &userStats{Domain: domain, UID: stat.UID}
			users[key] = entry
		}
		entry.NLiked = stat.NLiked
	}
	missingUsers, err := findMissingDomainUsers(ctx, db, domain, users)
	if err != nil {
		return nil, err
	}
	if len(missingUsers) > 0 {
		report.UsersMissing = int64(len(missingUsers))
		for _, uid := range missingUsers {
			delete(users, userStatsKey(domain, uid))
		}
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("%d user stats target(s) have no domain.user document and were skipped", len(missingUsers)))
	}
	report.UserStatsMatched = int64(len(users))
	report.LikeStatsMatched = countLikeUsersAfterFilter(likes, users)

	problems, err := aggregateProblemStats(ctx, db, domain, filter)
	if err != nil {
		return nil, err
	}
	missing, err := findMissingProblems(ctx, db, domain, problems)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		report.ProblemsMissing = int64(len(missing))
		for _, pid := range missing {
			delete(problems, pid)
		}
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("%d record problem id(s) have no matching document and were skipped", len(missing)))
	}
	report.ProblemsMatched = int64(len(problems))

	if !opts.DryRun {
		report.UserDocsUpdated, err = updateUserStats(ctx, db, domain, users)
		if err != nil {
			return nil, err
		}
		report.ProblemDocsUpdated, err = updateProblemStats(ctx, db, domain, problems)
		if err != nil {
			return nil, err
		}
	}
	report.EndedAt = time.Now().UTC()
	return report, nil
}

func recordStatsFilter(domain string) bson.M {
	return bson.M{
		"domainId": domain,
		"contest":  bson.M{"$nin": ignoredContestIDs},
		"status":   bson.M{"$ne": canceledStatus},
	}
}

func userRecordStatsFilter(domain string) bson.M {
	filter := recordStatsFilter(domain)
	filter["uid"] = bson.M{"$gte": 0}
	return filter
}

func solutionStatsFilter(domain string) bson.M {
	return bson.M{"domainId": domain, "docType": solutionDocType, "vote": bson.M{"$gt": 0}}
}

func aggregateUserStats(ctx context.Context, db *mongo.Database, domain string, filter bson.M) (map[string]*userStats, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":     bson.M{"domainId": "$domainId", "pid": "$pid", "uid": "$uid"},
			"nSubmit": bson.M{"$sum": 1},
			"nAccept": bson.M{"$sum": bson.M{"$cond": []any{
				bson.M{"$eq": []any{"$status", acceptedStatus}}, 1, 0,
			}}},
		}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":     bson.M{"domainId": "$_id.domainId", "uid": "$_id.uid"},
			"nSubmit": bson.M{"$sum": bson.M{"$min": []any{"$nSubmit", 1}}},
			"nAccept": bson.M{"$sum": bson.M{"$min": []any{"$nAccept", 1}}},
		}}},
	}
	cursor, err := db.Collection("record").Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, fmt.Errorf("aggregate user stats: %w", err)
	}
	defer cursor.Close(ctx)
	result := map[string]*userStats{}
	for cursor.Next(ctx) {
		var item struct {
			ID struct {
				DomainID string `bson:"domainId"`
				UID      int64  `bson:"uid"`
			} `bson:"_id"`
			NSubmit int64 `bson:"nSubmit"`
			NAccept int64 `bson:"nAccept"`
		}
		if err := cursor.Decode(&item); err != nil {
			return nil, fmt.Errorf("decode user stats: %w", err)
		}
		if item.ID.UID < 0 {
			continue
		}
		key := userStatsKey(item.ID.DomainID, item.ID.UID)
		result[key] = &userStats{
			Domain: item.ID.DomainID, UID: item.ID.UID,
			NSubmit: item.NSubmit, NAccept: item.NAccept,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("aggregate user stats: %w", err)
	}
	return result, nil
}

func aggregateLikeStats(ctx context.Context, db *mongo.Database, domain string) (map[string]*userStats, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: solutionStatsFilter(domain)}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":    bson.M{"domainId": "$domainId", "uid": "$owner"},
			"nLiked": bson.M{"$sum": "$vote"},
		}}},
	}
	cursor, err := db.Collection("document").Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, fmt.Errorf("aggregate solution likes: %w", err)
	}
	defer cursor.Close(ctx)
	result := map[string]*userStats{}
	for cursor.Next(ctx) {
		var item struct {
			ID struct {
				DomainID string `bson:"domainId"`
				UID      int64  `bson:"uid"`
			} `bson:"_id"`
			NLiked int64 `bson:"nLiked"`
		}
		if err := cursor.Decode(&item); err != nil {
			return nil, fmt.Errorf("decode solution likes: %w", err)
		}
		if item.ID.UID < 0 {
			continue
		}
		key := userStatsKey(item.ID.DomainID, item.ID.UID)
		result[key] = &userStats{
			Domain: item.ID.DomainID, UID: item.ID.UID, NLiked: item.NLiked,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("aggregate solution likes: %w", err)
	}
	return result, nil
}

func aggregateProblemStats(ctx context.Context, db *mongo.Database, domain string, filter bson.M) (map[int64]*problemStats, error) {
	firstGroup := bson.M{
		"_id":     bson.M{"domainId": "$domainId", "pid": "$pid", "uid": "$uid"},
		"nSubmit": bson.M{"$sum": 1},
		"nAccept": bson.M{"$sum": bson.M{"$cond": []any{
			bson.M{"$eq": []any{"$status", acceptedStatus}}, 1, 0,
		}}},
		"AC":  statusSum(acceptedStatus),
		"WA":  statusSum(2),
		"TLE": statusSum(3),
		"MLE": statusSum(4),
		"RE":  statusSum(6),
		"CE":  statusSum(7),
		"SE":  statusSum(8),
		"IGN": statusSum(9),
	}
	secondGroup := bson.M{
		"_id":     bson.M{"domainId": "$_id.domainId", "pid": "$_id.pid"},
		"nSubmit": bson.M{"$sum": "$nSubmit"},
		"nAccept": bson.M{"$sum": bson.M{"$min": []any{"$nAccept", 1}}},
		"AC":      bson.M{"$sum": "$AC"},
		"WA":      bson.M{"$sum": "$WA"},
		"TLE":     bson.M{"$sum": "$TLE"},
		"MLE":     bson.M{"$sum": "$MLE"},
		"RE":      bson.M{"$sum": "$RE"},
		"CE":      bson.M{"$sum": "$CE"},
		"SE":      bson.M{"$sum": "$SE"},
		"IGN":     bson.M{"$sum": "$IGN"},
	}
	for i := 0; i <= 100; i++ {
		firstGroup[fmt.Sprintf("s%d", i)] = bson.M{"$sum": bson.M{"$cond": []any{
			bson.M{"$and": []any{
				bson.M{"$gte": []any{"$score", i}},
				bson.M{"$lt": []any{"$score", i + 1}},
			}}, 1, 0,
		}}}
		secondGroup[fmt.Sprintf("s%d", i)] = bson.M{"$sum": fmt.Sprintf("$s%d", i)}
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$group", Value: firstGroup}},
		bson.D{{Key: "$group", Value: secondGroup}},
	}
	cursor, err := db.Collection("record").Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, fmt.Errorf("aggregate problem stats: %w", err)
	}
	defer cursor.Close(ctx)
	result := map[int64]*problemStats{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode problem stats: %w", err)
		}
		id := asBSONMap(doc["_id"])
		pid, ok := toInt64(id["pid"])
		if !ok {
			continue
		}
		entry := &problemStats{
			Domain: domain, PID: pid,
			Status: map[string]int64{},
			Scores: map[int]int64{},
		}
		entry.NSubmit, _ = toInt64(doc["nSubmit"])
		entry.NAccept, _ = toInt64(doc["nAccept"])
		for _, key := range []string{"AC", "WA", "TLE", "MLE", "RE", "CE", "SE", "IGN"} {
			value, _ := toInt64(doc[key])
			entry.Status[key] = value
		}
		for i := 0; i <= 100; i++ {
			value, _ := toInt64(doc[fmt.Sprintf("s%d", i)])
			if value != 0 {
				entry.Scores[i] = value
			}
		}
		result[pid] = entry
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("aggregate problem stats: %w", err)
	}
	return result, nil
}

func statusSum(status int) bson.M {
	return bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []any{"$status", status}}, 1, 0}}}
}

func findMissingProblems(ctx context.Context, db *mongo.Database, domain string, problems map[int64]*problemStats) ([]int64, error) {
	if len(problems) == 0 {
		return nil, nil
	}
	pids := make([]int64, 0, len(problems))
	for pid := range problems {
		pids = append(pids, pid)
	}
	cursor, err := db.Collection("document").Find(ctx, bson.M{
		"domainId": domain, "docType": problemDocType, "docId": bson.M{"$in": pids},
	}, options.Find().SetProjection(bson.M{"docId": 1}))
	if err != nil {
		return nil, fmt.Errorf("find problem stats targets: %w", err)
	}
	defer cursor.Close(ctx)
	found := map[int64]struct{}{}
	for cursor.Next(ctx) {
		var item struct {
			DocID int64 `bson:"docId"`
		}
		if err := cursor.Decode(&item); err != nil {
			return nil, fmt.Errorf("decode problem stats target: %w", err)
		}
		found[item.DocID] = struct{}{}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("find problem stats targets: %w", err)
	}
	missing := make([]int64, 0)
	for _, pid := range pids {
		if _, ok := found[pid]; !ok {
			missing = append(missing, pid)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return missing, nil
}

func findMissingDomainUsers(ctx context.Context, db *mongo.Database, domain string, users map[string]*userStats) ([]int64, error) {
	if len(users) == 0 {
		return nil, nil
	}
	uids := make([]int64, 0, len(users))
	for _, user := range users {
		uids = append(uids, user.UID)
	}
	cursor, err := db.Collection("domain.user").Find(ctx, bson.M{
		"domainId": domain, "uid": bson.M{"$in": uids},
	}, options.Find().SetProjection(bson.M{"uid": 1}))
	if err != nil {
		return nil, fmt.Errorf("find user stats targets: %w", err)
	}
	defer cursor.Close(ctx)
	found := map[int64]struct{}{}
	for cursor.Next(ctx) {
		var item struct {
			UID int64 `bson:"uid"`
		}
		if err := cursor.Decode(&item); err != nil {
			return nil, fmt.Errorf("decode user stats target: %w", err)
		}
		found[item.UID] = struct{}{}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("find user stats targets: %w", err)
	}
	missing := make([]int64, 0)
	for _, uid := range uids {
		if _, ok := found[uid]; !ok {
			missing = append(missing, uid)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return missing, nil
}

func countLikeUsersAfterFilter(likes map[string]*userStats, users map[string]*userStats) int64 {
	var count int64
	for key := range likes {
		if _, ok := users[key]; ok {
			count++
		}
	}
	return count
}

func updateUserStats(ctx context.Context, db *mongo.Database, domain string, stats map[string]*userStats) (int64, error) {
	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	models := make([]mongo.WriteModel, 0, len(keys))
	for _, key := range keys {
		item := stats[key]
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"domainId": domain, "uid": item.UID}).
			SetUpdate(bson.M{"$set": bson.M{
				"nSubmit": item.NSubmit, "nAccept": item.NAccept, "nLiked": item.NLiked,
			}}))
	}
	return applyBulkUpdates(ctx, db.Collection("domain.user"), models)
}

func updateProblemStats(ctx context.Context, db *mongo.Database, domain string, stats map[int64]*problemStats) (int64, error) {
	pids := make([]int64, 0, len(stats))
	for pid := range stats {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	models := make([]mongo.WriteModel, 0, len(pids))
	for _, pid := range pids {
		item := stats[pid]
		statsDoc := make(bson.D, 0, 9+len(item.Scores))
		for _, key := range []string{"AC", "WA", "TLE", "MLE", "RE", "CE", "SE", "IGN"} {
			statsDoc = append(statsDoc, bson.E{Key: key, Value: item.Status[key]})
		}
		scoreKeys := make([]int, 0, len(item.Scores))
		for score := range item.Scores {
			scoreKeys = append(scoreKeys, score)
		}
		sort.Ints(scoreKeys)
		for _, score := range scoreKeys {
			statsDoc = append(statsDoc, bson.E{Key: fmt.Sprintf("s%d", score), Value: item.Scores[score]})
		}
		values := bson.M{
			"nSubmit": item.NSubmit,
			"nAccept": item.NAccept,
			"stats":   statsDoc,
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"domainId": domain, "docType": problemDocType, "docId": pid}).
			SetUpdate(bson.M{"$set": values}))
	}
	return applyBulkUpdates(ctx, db.Collection("document"), models)
}

func applyBulkUpdates(ctx context.Context, collection *mongo.Collection, models []mongo.WriteModel) (int64, error) {
	var modified int64
	for start := 0; start < len(models); start += statsBatchSize {
		end := start + statsBatchSize
		if end > len(models) {
			end = len(models)
		}
		result, err := collection.BulkWrite(ctx, models[start:end], options.BulkWrite().SetOrdered(false))
		if err != nil {
			return modified, fmt.Errorf("apply stats bulk update: %w", err)
		}
		modified += result.ModifiedCount
	}
	return modified, nil
}

func userStatsKey(domain string, uid int64) string {
	return fmt.Sprintf("%s/%d", domain, uid)
}

func asBSONMap(value any) bson.M {
	switch item := value.(type) {
	case bson.M:
		return item
	case bson.D:
		result := make(bson.M, len(item))
		for _, element := range item {
			result[element.Key] = element.Value
		}
		return result
	default:
		return nil
	}
}
