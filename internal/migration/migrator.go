package migration

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DefaultTargetDomain = "system"

var ScopedCollections = []string{
	"document",
	"document.status",
	"record",
	"record.stat",
	"record.history",
	"task",
	"schedule",
	"oplog",
	"contest.balloon",
	"vjudge",
	"vjudge.mount",
}

var maxFields = map[string]struct{}{
	"rp":        {},
	"rating":    {},
	"maxRating": {},
	"level":     {},
}

var storagePrefixes = []string{"problem", "contest", "training"}

type Options struct {
	Sources []string
	Target  string
	DryRun  bool
}

type CollectionReport struct {
	Collection string `json:"collection"`
	Domain     string `json:"domain"`
	Matched    int64  `json:"matched"`
	Modified   int64  `json:"modified,omitempty"`
	Remaining  *int64 `json:"remaining,omitempty"`
}

type DomainReport struct {
	Domain   string `json:"domain"`
	Found    bool   `json:"found"`
	Archived bool   `json:"archived,omitempty"`
}

type Residual struct {
	Collection string `json:"collection"`
	Domain     string `json:"domain"`
	Count      int64  `json:"count"`
}

type ValidationReport struct {
	OK        bool       `json:"ok"`
	Residuals []Residual `json:"residuals,omitempty"`
}

type Report struct {
	DryRun         bool               `json:"dry_run"`
	Target         string             `json:"target"`
	Sources        []string           `json:"sources"`
	StartedAt      time.Time          `json:"started_at"`
	EndedAt        time.Time          `json:"ended_at,omitempty"`
	Collections    []CollectionReport `json:"collections"`
	Domains        []DomainReport     `json:"domains"`
	UsersMerged    int64              `json:"users_merged"`
	UsersRemaining *int64             `json:"users_remaining,omitempty"`
	Validation     *ValidationReport  `json:"validation,omitempty"`
	Warnings       []string           `json:"warnings,omitempty"`
}

func Run(ctx context.Context, db *mongo.Database, opts Options) (*Report, error) {
	sources, target, err := normalize(opts)
	if err != nil {
		return nil, err
	}
	report := &Report{
		DryRun:    opts.DryRun,
		Target:    target,
		Sources:   sources,
		StartedAt: time.Now().UTC(),
		Warnings: []string{
			"physical object-storage migration is not performed; only logical storage.path prefixes are rewritten",
		},
	}
	if !opts.DryRun {
		report.DryRun = false
	}

	for _, source := range sources {
		for _, collection := range ScopedCollections {
			count, err := db.Collection(collection).CountDocuments(ctx, bson.M{"domainId": source})
			if err != nil {
				return nil, fmt.Errorf("count %s/%s: %w", collection, source, err)
			}
			report.Collections = append(report.Collections, CollectionReport{
				Collection: collection,
				Domain:     source,
				Matched:    count,
			})
		}
		storageCount, err := countStoragePaths(ctx, db, source)
		if err != nil {
			return nil, err
		}
		report.Collections = append(report.Collections, CollectionReport{
			Collection: "storage.path",
			Domain:     source,
			Matched:    storageCount,
		})

		domainFound := false
		var domainDoc bson.M
		err = db.Collection("domain").FindOne(ctx, bson.M{"$or": []bson.M{{"_id": source}, {"lower": source}}}).Decode(&domainDoc)
		if err == nil {
			domainFound = true
		} else if err != mongo.ErrNoDocuments {
			return nil, fmt.Errorf("read domain %s: %w", source, err)
		}
		report.Domains = append(report.Domains, DomainReport{Domain: source, Found: domainFound})

		if opts.DryRun {
			continue
		}

		for _, collection := range ScopedCollections {
			result, err := db.Collection(collection).UpdateMany(ctx, bson.M{"domainId": source}, bson.M{
				"$set":      bson.M{"domainId": target},
				"$addToSet": bson.M{"legacyDomains": source},
			})
			if err != nil {
				return nil, fmt.Errorf("update %s/%s: %w", collection, source, err)
			}
			updateReport(report.Collections, collection, source, result.ModifiedCount)
		}

		merged, err := mergeDomainUsers(ctx, db, source, target)
		if err != nil {
			return nil, err
		}
		report.UsersMerged += merged

		storageModified, storageRemaining, err := migrateStoragePaths(ctx, db, source, target)
		if err != nil {
			return nil, err
		}
		updateReport(report.Collections, "storage.path", source, storageModified)
		setReportRemaining(report.Collections, "storage.path", source, storageRemaining)

		if domainFound {
			if _, err := db.Collection("domain.legacy").ReplaceOne(ctx, bson.M{"_id": domainDoc["_id"]}, domainDoc, options.Replace().SetUpsert(true)); err != nil {
				return nil, fmt.Errorf("archive domain %s: %w", source, err)
			}
			if _, err := db.Collection("domain").DeleteOne(ctx, bson.M{"_id": domainDoc["_id"]}); err != nil {
				return nil, fmt.Errorf("delete domain %s: %w", source, err)
			}
			for i := range report.Domains {
				if report.Domains[i].Domain == source {
					report.Domains[i].Archived = true
				}
			}
		}
	}
	if !opts.DryRun {
		residuals, err := collectResiduals(ctx, db, sources)
		if err != nil {
			return nil, err
		}
		report.Validation = &ValidationReport{OK: len(residuals) == 0, Residuals: residuals}
		setAllReportRemaining(report.Collections, residuals)
		usersRemaining, err := countDomainUsers(ctx, db, sources)
		if err != nil {
			return nil, err
		}
		report.UsersRemaining = &usersRemaining
		if !report.Validation.OK {
			report.Warnings = append(report.Warnings, "migration finished with residual source-domain data; inspect validation.residuals")
		}
	}
	report.EndedAt = time.Now().UTC()
	sort.Slice(report.Collections, func(i, j int) bool {
		if report.Collections[i].Domain == report.Collections[j].Domain {
			return report.Collections[i].Collection < report.Collections[j].Collection
		}
		return report.Collections[i].Domain < report.Collections[j].Domain
	})
	return report, nil
}

func storagePathFilter(source string) bson.M {
	pattern := fmt.Sprintf("^(%s)/%s/", strings.Join(storagePrefixes, "|"), regexp.QuoteMeta(source))
	return bson.M{"path": bson.M{"$regex": pattern}}
}

func countStoragePaths(ctx context.Context, db *mongo.Database, source string) (int64, error) {
	count, err := db.Collection("storage").CountDocuments(ctx, storagePathFilter(source))
	if err != nil {
		return 0, fmt.Errorf("count storage.path/%s: %w", source, err)
	}
	return count, nil
}

func migrateStoragePaths(ctx context.Context, db *mongo.Database, source, target string) (int64, int64, error) {
	coll := db.Collection("storage")
	cursor, err := coll.Find(ctx, storagePathFilter(source), options.Find().SetProjection(bson.M{"path": 1}))
	if err != nil {
		return 0, 0, fmt.Errorf("read storage.path/%s: %w", source, err)
	}
	defer cursor.Close(ctx)
	var docs []struct {
		ID   any    `bson:"_id"`
		Path string `bson:"path"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return 0, 0, fmt.Errorf("decode storage.path/%s: %w", source, err)
	}
	var modified int64
	for _, doc := range docs {
		parts := strings.SplitN(doc.Path, "/", 3)
		if len(parts) != 3 || parts[1] != source {
			continue
		}
		newPath := parts[0] + "/" + target + "/" + parts[2]
		result, err := coll.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{"$set": bson.M{"path": newPath}})
		if err != nil {
			return modified, 0, fmt.Errorf("update storage.path/%s: %w", source, err)
		}
		modified += result.ModifiedCount
	}
	remaining, err := countStoragePaths(ctx, db, source)
	if err != nil {
		return modified, 0, err
	}
	return modified, remaining, nil
}

func countDomainUsers(ctx context.Context, db *mongo.Database, sources []string) (int64, error) {
	if len(sources) == 0 {
		return 0, nil
	}
	count, err := db.Collection("domain.user").CountDocuments(ctx, bson.M{"domainId": bson.M{"$in": sources}})
	if err != nil {
		return 0, fmt.Errorf("count residual domain.user: %w", err)
	}
	return count, nil
}

func collectResiduals(ctx context.Context, db *mongo.Database, sources []string) ([]Residual, error) {
	residuals := make([]Residual, 0)
	for _, source := range sources {
		for _, collection := range ScopedCollections {
			count, err := db.Collection(collection).CountDocuments(ctx, bson.M{"domainId": source})
			if err != nil {
				return nil, fmt.Errorf("validate %s/%s: %w", collection, source, err)
			}
			if count > 0 {
				residuals = append(residuals, Residual{Collection: collection, Domain: source, Count: count})
			}
		}
		userCount, err := db.Collection("domain.user").CountDocuments(ctx, bson.M{"domainId": source})
		if err != nil {
			return nil, fmt.Errorf("validate domain.user/%s: %w", source, err)
		}
		if userCount > 0 {
			residuals = append(residuals, Residual{Collection: "domain.user", Domain: source, Count: userCount})
		}
		storageCount, err := countStoragePaths(ctx, db, source)
		if err != nil {
			return nil, err
		}
		if storageCount > 0 {
			residuals = append(residuals, Residual{Collection: "storage.path", Domain: source, Count: storageCount})
		}
		domainCount, err := db.Collection("domain").CountDocuments(ctx, bson.M{"$or": []bson.M{{"_id": source}, {"lower": source}}})
		if err != nil {
			return nil, fmt.Errorf("validate domain/%s: %w", source, err)
		}
		if domainCount > 0 {
			residuals = append(residuals, Residual{Collection: "domain", Domain: source, Count: domainCount})
		}
	}
	return residuals, nil
}

func setReportRemaining(collections []CollectionReport, collection, source string, remaining int64) {
	for i := range collections {
		if collections[i].Collection == collection && collections[i].Domain == source {
			value := remaining
			collections[i].Remaining = &value
			return
		}
	}
}

func setAllReportRemaining(collections []CollectionReport, residuals []Residual) {
	for i := range collections {
		value := int64(0)
		collections[i].Remaining = &value
	}
	for _, residual := range residuals {
		for i := range collections {
			if collections[i].Collection == residual.Collection && collections[i].Domain == residual.Domain {
				value := residual.Count
				collections[i].Remaining = &value
			}
		}
	}
}

func normalize(opts Options) ([]string, string, error) {
	target := opts.Target
	if target == "" {
		target = DefaultTargetDomain
	}
	if target != DefaultTargetDomain {
		return nil, "", fmt.Errorf("target domain must be %q in this rewrite", DefaultTargetDomain)
	}
	seen := map[string]struct{}{}
	sources := make([]string, 0, len(opts.Sources))
	for _, source := range opts.Sources {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" || source == target {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return nil, "", fmt.Errorf("at least one source domain is required")
	}
	return sources, target, nil
}

func updateReport(collections []CollectionReport, collection, source string, modified int64) {
	for i := range collections {
		if collections[i].Collection == collection && collections[i].Domain == source {
			collections[i].Modified = modified
			return
		}
	}
}

func mergeDomainUsers(ctx context.Context, db *mongo.Database, source, target string) (int64, error) {
	cursor, err := db.Collection("domain.user").Find(ctx, bson.M{"domainId": source})
	if err != nil {
		return 0, fmt.Errorf("read domain.user/%s: %w", source, err)
	}
	defer cursor.Close(ctx)
	var merged int64
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return merged, err
		}
		if _, err := mergeDomainUser(ctx, db, doc, source, target); err != nil {
			return merged, err
		}
		merged++
	}
	if err := cursor.Err(); err != nil {
		return merged, err
	}
	return merged, nil
}

func mergeDomainUser(ctx context.Context, db *mongo.Database, sourceDoc bson.M, source, target string) (int64, error) {
	uid, ok := toInt64(sourceDoc["uid"])
	if !ok {
		return 0, fmt.Errorf("domain.user document has no numeric uid: %v", sourceDoc)
	}
	onInsert := bson.M{"domainId": target, "uid": uid}
	inc := bson.M{}
	max := bson.M{}
	for key, value := range sourceDoc {
		if key == "_id" || key == "domainId" || key == "uid" || key == "legacyDomains" {
			continue
		}
		if number, ok := toNumber(value); ok {
			if _, isMax := maxFields[key]; isMax {
				max[key] = number
			} else {
				inc[key] = number
			}
			continue
		}
		onInsert[key] = value
	}
	update := bson.M{"$setOnInsert": onInsert, "$addToSet": bson.M{"legacyDomains": source}}
	if len(inc) > 0 {
		update["$inc"] = inc
	}
	if len(max) > 0 {
		update["$max"] = max
	}
	result, err := db.Collection("domain.user").UpdateOne(ctx, bson.M{"domainId": target, "uid": uid}, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return 0, fmt.Errorf("merge domain.user uid=%d: %w", uid, err)
	}
	if result.UpsertedCount == 0 && result.MatchedCount == 0 {
		return 0, fmt.Errorf("merge domain.user uid=%d matched no target", uid)
	}
	if _, err := db.Collection("domain.user").DeleteOne(ctx, bson.M{"_id": sourceDoc["_id"]}); err != nil {
		return 0, fmt.Errorf("delete source domain.user uid=%d: %w", uid, err)
	}
	return 1, nil
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
		if v == float64(int64(v)) {
			return int64(v), true
		}
	}
	return 0, false
}

func toNumber(value any) (any, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return v, true
	}
	return nil, false
}
