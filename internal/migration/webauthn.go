package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// WebAuthnOptions controls the one-way import of Hydro's user.authenticators
// records into the native WebAuthn storage used by syuct-oj.
type WebAuthnOptions struct {
	Sources []string
	DryRun  bool
}

type WebAuthnConflict struct {
	UID          int64  `json:"uid"`
	Username     string `json:"username,omitempty"`
	CredentialID string `json:"credentialId,omitempty"`
	Reason       string `json:"reason"`
}

type WebAuthnDomainReport struct {
	Domain      string `json:"domain"`
	Users       int64  `json:"users"`
	Credentials int64  `json:"credentials"`
}

type WebAuthnReport struct {
	DryRun             bool                   `json:"dry_run"`
	Sources            []string               `json:"sources,omitempty"`
	StartedAt          time.Time              `json:"started_at"`
	EndedAt            time.Time              `json:"ended_at,omitempty"`
	OK                 bool                   `json:"ok"`
	Users              int64                  `json:"users"`
	Credentials        int64                  `json:"credentials"`
	PlannedImports     int64                  `json:"planned_imports"`
	Imported           int64                  `json:"imported,omitempty"`
	Existing           int64                  `json:"existing,omitempty"`
	SkippedFormerly    int64                  `json:"skipped_formerly_imported,omitempty"`
	PlannedUserUpdates int64                  `json:"planned_user_updates"`
	UsersUpdated       int64                  `json:"users_updated,omitempty"`
	UnassignedUsers    int64                  `json:"unassigned_users,omitempty"`
	Domains            []WebAuthnDomainReport `json:"domains,omitempty"`
	Conflicts          []WebAuthnConflict     `json:"conflicts,omitempty"`
	Warnings           []string               `json:"warnings,omitempty"`
}

type legacyUser struct {
	ID             int64                 `bson:"_id"`
	Username       string                `bson:"uname"`
	WebAuthnID     []byte                `bson:"webauthnId"`
	ImportedIDs    []string              `bson:"legacyWebAuthnImported"`
	Authenticators []legacyAuthenticator `bson:"authenticators"`
}

type legacyUserPlan struct {
	User             legacyUser
	Handle           []byte
	HandleNeedsWrite bool
	Active           bool
	Credentials      []plannedWebAuthnCredential
}

type storedWebAuthnCredential struct {
	ID  string `bson:"_id"`
	UID int64  `bson:"uid"`
}

func RunLegacyWebAuthn(ctx context.Context, db *mongo.Database, opts WebAuthnOptions) (*WebAuthnReport, error) {
	sources := normalizeWebAuthnSources(opts.Sources)
	report := &WebAuthnReport{
		DryRun:    opts.DryRun,
		Sources:   sources,
		StartedAt: time.Now().UTC(),
	}
	users, err := loadLegacyWebAuthnUsers(ctx, db)
	if err != nil {
		return nil, err
	}
	report.Users = int64(len(users))
	if len(users) == 0 {
		report.OK = true
		report.EndedAt = time.Now().UTC()
		return report, nil
	}

	plans := make([]*legacyUserPlan, 0, len(users))
	credentialIDs := make([]string, 0)
	duplicates := map[string]WebAuthnConflict{}
	for _, user := range users {
		plan := &legacyUserPlan{
			User:   user,
			Handle: []byte(strconv.FormatInt(user.ID, 10)),
		}
		plan.HandleNeedsWrite = !bytes.Equal(user.WebAuthnID, plan.Handle)
		importedIDs := make(map[string]struct{}, len(user.ImportedIDs))
		for _, id := range user.ImportedIDs {
			importedIDs[id] = struct{}{}
		}
		seen := map[string]struct{}{}
		for _, raw := range user.Authenticators {
			report.Credentials++
			id, credential, createdAt, err := convertLegacyAuthenticator(raw)
			if err != nil {
				report.Conflicts = append(report.Conflicts, WebAuthnConflict{
					UID: user.ID, Username: user.Username, Reason: err.Error(),
				})
				continue
			}
			if _, ok := seen[id]; ok {
				report.Conflicts = append(report.Conflicts, WebAuthnConflict{
					UID: user.ID, Username: user.Username, CredentialID: id,
					Reason: "duplicate credential id in legacy authenticator list",
				})
				continue
			}
			if previous, ok := duplicates[id]; ok {
				report.Conflicts = append(report.Conflicts, WebAuthnConflict{
					UID: user.ID, Username: user.Username, CredentialID: id,
					Reason: fmt.Sprintf("credential id also belongs to uid %d", previous.UID),
				})
				continue
			}
			seen[id] = struct{}{}
			duplicates[id] = WebAuthnConflict{UID: user.ID, Username: user.Username, CredentialID: id}
			credentialIDs = append(credentialIDs, id)
			if createdAt.IsZero() {
				createdAt = time.Now().UTC()
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"legacy credential %s for uid %d has no registration time; using migration time", id, user.ID))
			}
			_, previouslyImported := importedIDs[id]
			plan.Credentials = append(plan.Credentials, plannedWebAuthnCredential{
				ID: id, Name: credentialName(raw.Name), CreatedAt: createdAt, Credential: credential,
				PreviouslyImported: previouslyImported,
			})
		}
		if len(plan.Credentials) > 0 {
			plans = append(plans, plan)
		}
	}

	existing, err := loadExistingWebAuthnCredentials(ctx, db, credentialIDs)
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		for i := range plan.Credentials {
			item := &plan.Credentials[i]
			doc, ok := existing[item.ID]
			if !ok {
				if item.PreviouslyImported {
					report.SkippedFormerly++
					continue
				}
				report.PlannedImports++
				plan.Active = true
				continue
			}
			if doc.UID != plan.User.ID {
				report.Conflicts = append(report.Conflicts, WebAuthnConflict{
					UID: plan.User.ID, Username: plan.User.Username, CredentialID: item.ID,
					Reason: fmt.Sprintf("credential id already belongs to uid %d", doc.UID),
				})
				continue
			}
			item.Existing = true
			plan.Active = true
			report.Existing++
		}
	}
	report.PlannedUserUpdates = countPlannedUserUpdates(plans)
	if err := detectWebAuthnHandleConflicts(ctx, db, plans, report); err != nil {
		return nil, err
	}

	memberships, err := loadSourceMemberships(ctx, db, sources)
	if err != nil {
		return nil, err
	}
	report.Domains, report.UnassignedUsers = summarizeWebAuthnDomains(users, memberships, sources)
	if len(sources) > 0 && len(report.Domains) > 0 {
		assigned := int64(0)
		for _, item := range report.Domains {
			assigned += item.Users
		}
		if assigned == 0 {
			report.Warnings = append(report.Warnings,
				"no legacy users with authenticators belong to the selected source domains; source-domain counts are empty")
		}
	}
	if len(report.Conflicts) > 0 {
		report.OK = false
		report.Warnings = append(report.Warnings,
			"legacy WebAuthn migration is blocked until all credential conflicts are resolved")
		report.EndedAt = time.Now().UTC()
		if !opts.DryRun {
			return report, fmt.Errorf("legacy WebAuthn migration has %d conflicts", len(report.Conflicts))
		}
		return report, nil
	}

	report.OK = true
	if !opts.DryRun {
		if err := applyLegacyWebAuthnImport(ctx, db, plans); err != nil {
			report.OK = false
			return report, err
		}
		for _, plan := range plans {
			for _, item := range plan.Credentials {
				if !item.Existing && !item.PreviouslyImported {
					report.Imported++
				}
			}
			if plan.Active && plan.HandleNeedsWrite {
				report.UsersUpdated++
			}
		}
	}
	report.EndedAt = time.Now().UTC()
	return report, nil
}

func normalizeWebAuthnSources(sources []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(sources))
	for _, source := range sources {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		result = append(result, source)
	}
	return result
}

func loadLegacyWebAuthnUsers(ctx context.Context, db *mongo.Database) ([]legacyUser, error) {
	cursor, err := db.Collection("user").Find(ctx,
		bson.M{"authenticators.0": bson.M{"$exists": true}},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("read legacy authenticators: %w", err)
	}
	defer cursor.Close(ctx)
	var users []legacyUser
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("decode legacy authenticators: %w", err)
	}
	return users, nil
}

func loadExistingWebAuthnCredentials(ctx context.Context, db *mongo.Database, ids []string) (map[string]storedWebAuthnCredential, error) {
	result := make(map[string]storedWebAuthnCredential, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := db.Collection("webauthn_credentials").Find(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"uid": 1}))
	if err != nil {
		return nil, fmt.Errorf("read existing WebAuthn credentials: %w", err)
	}
	defer cursor.Close(ctx)
	var items []storedWebAuthnCredential
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode existing WebAuthn credentials: %w", err)
	}
	for _, item := range items {
		result[item.ID] = item
	}
	return result, nil
}

func applyLegacyWebAuthnImport(ctx context.Context, db *mongo.Database, plans []*legacyUserPlan) error {
	collection := db.Collection("webauthn_credentials")
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "uid", Value: 1}},
		Options: options.Index().SetName("webauthn_credentials_uid"),
	}); err != nil {
		return fmt.Errorf("ensure WebAuthn credential index: %w", err)
	}
	for _, plan := range plans {
		for _, item := range plan.Credentials {
			if item.Existing || item.PreviouslyImported {
				continue
			}
			data, err := json.Marshal(item.Credential)
			if err != nil {
				return fmt.Errorf("encode legacy credential %s: %w", item.ID, err)
			}
			_, err = collection.InsertOne(ctx, bson.M{
				"_id": item.ID, "uid": plan.User.ID, "name": item.Name,
				"data": string(data), "createdAt": item.CreatedAt,
			})
			if err != nil {
				return fmt.Errorf("insert legacy credential %s: %w", item.ID, err)
			}
		}
		update := bson.M{}
		if plan.Active && plan.HandleNeedsWrite {
			update["$set"] = bson.M{"webauthnId": plan.Handle}
		}
		ids := make([]string, 0, len(plan.Credentials))
		for _, item := range plan.Credentials {
			ids = append(ids, item.ID)
		}
		if len(ids) > 0 {
			update["$addToSet"] = bson.M{"legacyWebAuthnImported": bson.M{"$each": ids}}
		}
		if len(update) > 0 {
			if _, err := db.Collection("user").UpdateOne(ctx,
				bson.M{"_id": plan.User.ID}, update); err != nil {
				return fmt.Errorf("finalize legacy WebAuthn import for uid %d: %w", plan.User.ID, err)
			}
		}
	}
	return nil
}

func countPlannedUserUpdates(plans []*legacyUserPlan) int64 {
	var count int64
	for _, plan := range plans {
		if plan.Active && plan.HandleNeedsWrite {
			count++
		}
	}
	return count
}

func detectWebAuthnHandleConflicts(ctx context.Context, db *mongo.Database, plans []*legacyUserPlan, report *WebAuthnReport) error {
	uids := make([]int64, 0)
	legacyIDs := map[int64]map[string]struct{}{}
	for _, plan := range plans {
		if plan.Active && plan.HandleNeedsWrite {
			uids = append(uids, plan.User.ID)
			ids := make(map[string]struct{}, len(plan.Credentials))
			for _, item := range plan.Credentials {
				ids[item.ID] = struct{}{}
			}
			legacyIDs[plan.User.ID] = ids
		}
	}
	if len(uids) == 0 {
		return nil
	}
	cursor, err := db.Collection("webauthn_credentials").Find(ctx,
		bson.M{"uid": bson.M{"$in": uids}},
		options.Find().SetProjection(bson.M{"uid": 1}))
	if err != nil {
		return fmt.Errorf("check WebAuthn handle conflicts: %w", err)
	}
	defer cursor.Close(ctx)
	counts := map[int64]int64{}
	for cursor.Next(ctx) {
		var item struct {
			ID  string `bson:"_id"`
			UID int64  `bson:"uid"`
		}
		if err := cursor.Decode(&item); err != nil {
			return fmt.Errorf("decode WebAuthn handle conflict: %w", err)
		}
		if _, ok := legacyIDs[item.UID][item.ID]; !ok {
			counts[item.UID]++
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("check WebAuthn handle conflicts: %w", err)
	}
	for _, plan := range plans {
		if !plan.Active || !plan.HandleNeedsWrite || counts[plan.User.ID] == 0 {
			continue
		}
		report.Conflicts = append(report.Conflicts, WebAuthnConflict{
			UID: plan.User.ID, Username: plan.User.Username,
			Reason: fmt.Sprintf("user already has %d native Passkey(s) with a different user handle; changing the handle would invalidate them", counts[plan.User.ID]),
		})
	}
	return nil
}

func loadSourceMemberships(ctx context.Context, db *mongo.Database, sources []string) (map[int64][]string, error) {
	result := map[int64][]string{}
	if len(sources) == 0 {
		return result, nil
	}
	cursor, err := db.Collection("domain.user").Find(ctx,
		bson.M{"domainId": bson.M{"$in": sources}},
		options.Find().SetProjection(bson.M{"uid": 1, "domainId": 1}))
	if err != nil {
		return nil, fmt.Errorf("read domain memberships for WebAuthn report: %w", err)
	}
	defer cursor.Close(ctx)
	var items []struct {
		UID      int64  `bson:"uid"`
		DomainID string `bson:"domainId"`
	}
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode domain memberships for WebAuthn report: %w", err)
	}
	seen := map[int64]map[string]struct{}{}
	for _, item := range items {
		if seen[item.UID] == nil {
			seen[item.UID] = map[string]struct{}{}
		}
		if _, ok := seen[item.UID][item.DomainID]; ok {
			continue
		}
		seen[item.UID][item.DomainID] = struct{}{}
		result[item.UID] = append(result[item.UID], item.DomainID)
	}
	for uid := range result {
		sort.Strings(result[uid])
	}
	return result, nil
}

func summarizeWebAuthnDomains(users []legacyUser, memberships map[int64][]string, sources []string) ([]WebAuthnDomainReport, int64) {
	byDomain := make(map[string]*WebAuthnDomainReport, len(sources))
	for _, source := range sources {
		byDomain[source] = &WebAuthnDomainReport{Domain: source}
	}
	var unassigned int64
	for _, user := range users {
		domains := memberships[user.ID]
		matched := false
		for _, domain := range domains {
			if item, ok := byDomain[domain]; ok {
				item.Users++
				item.Credentials += int64(len(user.Authenticators))
				matched = true
			}
		}
		if !matched {
			unassigned++
		}
	}
	result := make([]WebAuthnDomainReport, 0, len(sources))
	for _, source := range sources {
		result = append(result, *byDomain[source])
	}
	return result, unassigned
}
