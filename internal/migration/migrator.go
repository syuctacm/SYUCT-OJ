package migration

import (
	"context"
	"fmt"
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
}

type DomainReport struct {
	Domain   string `json:"domain"`
	Found    bool   `json:"found"`
	Archived bool   `json:"archived,omitempty"`
}

type Report struct {
	DryRun      bool               `json:"dry_run"`
	Target      string             `json:"target"`
	Sources     []string           `json:"sources"`
	StartedAt   time.Time          `json:"started_at"`
	EndedAt     time.Time          `json:"ended_at,omitempty"`
	Collections []CollectionReport `json:"collections"`
	Domains     []DomainReport     `json:"domains"`
	UsersMerged int64              `json:"users_merged"`
	Warnings    []string           `json:"warnings,omitempty"`
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
			"storage paths are not rewritten by this tool; filesystem/object-storage migration must be handled separately",
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

		domainFound := false
		var domainDoc bson.M
		err := db.Collection("domain").FindOne(ctx, bson.M{"$or": []bson.M{{"_id": source}, {"lower": source}}}).Decode(&domainDoc)
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
	report.EndedAt = time.Now().UTC()
	sort.Slice(report.Collections, func(i, j int) bool {
		if report.Collections[i].Domain == report.Collections[j].Domain {
			return report.Collections[i].Collection < report.Collections[j].Collection
		}
		return report.Collections[i].Domain < report.Collections[j].Domain
	})
	return report, nil
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
