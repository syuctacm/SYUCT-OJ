package contest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const TeamMemberLimit = 5

type Team struct {
	ID          int64     `bson:"_id" json:"id"`
	UName       string    `bson:"uname" json:"-"`
	DisplayName string    `bson:"displayName" json:"displayName"`
	Members     []int64   `bson:"members" json:"members"`
	Invite      []int64   `bson:"invite,omitempty" json:"invite,omitempty"`
	Avatar      string    `bson:"avatar,omitempty" json:"avatar,omitempty"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
}

type TeamRepository struct {
	collection *mongo.Collection
}

func NewTeamRepository(db *mongo.Database) *TeamRepository {
	return &TeamRepository{collection: db.Collection("vuser")}
}

func (r *TeamRepository) Create(ctx context.Context, uid int64, displayName string) (*Team, error) {
	displayName = strings.TrimSpace(displayName)
	if uid <= 0 {
		return nil, fmt.Errorf("invalid user")
	}
	if displayName == "" {
		return nil, fmt.Errorf("team name is required")
	}
	if len([]rune(displayName)) > 64 {
		return nil, fmt.Errorf("team name is too long")
	}
	var first struct {
		ID int64 `bson:"_id"`
	}
	teamID := int64(-1000)
	err := r.collection.FindOne(ctx, bson.M{}, options.FindOne().SetSort(bson.D{{Key: "_id", Value: 1}})).Decode(&first)
	if err == nil {
		teamID = first.ID - 1
	} else if err != mongo.ErrNoDocuments {
		return nil, err
	}
	for attempts := 0; attempts < 100; attempts++ {
		random := make([]byte, 4)
		if _, err := rand.Read(random); err != nil {
			return nil, err
		}
		team := Team{
			ID: teamID, UName: fmt.Sprintf("team:%d:%s", uid, hex.EncodeToString(random)),
			DisplayName: displayName, Members: []int64{uid}, Invite: []int64{}, CreatedAt: time.Now().UTC(),
		}
		if _, err := r.collection.InsertOne(ctx, team); err == nil {
			return &team, nil
		} else if !mongo.IsDuplicateKeyError(err) {
			return nil, err
		}
		teamID--
	}
	return nil, fmt.Errorf("unable to allocate team id")
}

func (r *TeamRepository) ListForUser(ctx context.Context, uid int64) ([]Team, []Team, error) {
	mine := make([]Team, 0)
	invites := make([]Team, 0)
	if uid <= 0 {
		return mine, invites, nil
	}
	cursor, err := r.collection.Find(ctx, bson.M{"$or": []bson.M{{"members": uid}, {"invite": uid}}},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, nil, err
	}
	defer cursor.Close(ctx)
	var teams []Team
	if err := cursor.All(ctx, &teams); err != nil {
		return nil, nil, err
	}
	for _, team := range teams {
		if containsUserID(team.Members, uid) {
			mine = append(mine, team)
		} else if containsUserID(team.Invite, uid) {
			invites = append(invites, team)
		}
	}
	return mine, invites, nil
}

func (r *TeamRepository) Rename(ctx context.Context, actor int64, teamID int64, displayName string) (*Team, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, fmt.Errorf("team name is required")
	}
	if len([]rune(displayName)) > 64 {
		return nil, fmt.Errorf("team name is too long")
	}
	result, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": teamID, "members": actor},
		bson.M{"$set": bson.M{"displayName": displayName}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("team not found or permission denied")
	}
	return r.get(ctx, teamID)
}

func (r *TeamRepository) Invite(ctx context.Context, actor, target, teamID int64) (*Team, error) {
	if target <= 0 || target == actor {
		return nil, fmt.Errorf("invalid invitee")
	}
	team, err := r.get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if !containsUserID(team.Members, actor) {
		return nil, fmt.Errorf("team not found or permission denied")
	}
	if containsUserID(team.Members, target) || containsUserID(team.Invite, target) {
		return nil, fmt.Errorf("user is already a member or invitee")
	}
	if len(team.Members)+len(team.Invite) >= TeamMemberLimit {
		return nil, fmt.Errorf("team member limit is %d", TeamMemberLimit)
	}
	result, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": teamID, "members": actor},
		bson.M{"$addToSet": bson.M{"invite": target}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("team not found or permission denied")
	}
	return r.get(ctx, teamID)
}

func (r *TeamRepository) Accept(ctx context.Context, uid, teamID int64) (*Team, error) {
	team, err := r.get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if !containsUserID(team.Invite, uid) {
		return nil, fmt.Errorf("invitation not found")
	}
	if len(team.Members) >= TeamMemberLimit {
		return nil, fmt.Errorf("team member limit is %d", TeamMemberLimit)
	}
	result, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": teamID, "invite": uid},
		bson.M{"$pull": bson.M{"invite": uid}, "$addToSet": bson.M{"members": uid}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("invitation not found")
	}
	return r.get(ctx, teamID)
}

func (r *TeamRepository) Reject(ctx context.Context, uid, teamID int64) error {
	result, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": teamID, "invite": uid},
		bson.M{"$pull": bson.M{"invite": uid}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("invitation not found")
	}
	return nil
}

func (r *TeamRepository) Leave(ctx context.Context, actor, target, teamID int64) (*Team, error) {
	team, err := r.get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if target == actor {
		if !containsUserID(team.Members, actor) && !containsUserID(team.Invite, actor) {
			return nil, fmt.Errorf("team not found or permission denied")
		}
	} else if !containsUserID(team.Members, actor) {
		return nil, fmt.Errorf("team not found or permission denied")
	}
	if target <= 0 {
		return nil, fmt.Errorf("invalid member")
	}
	result, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": teamID},
		bson.M{"$pull": bson.M{"members": target, "invite": target}})
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("team not found")
	}
	return r.get(ctx, teamID)
}

func (r *TeamRepository) get(ctx context.Context, teamID int64) (*Team, error) {
	var team Team
	if err := r.collection.FindOne(ctx, bson.M{"_id": teamID}).Decode(&team); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("team not found")
		}
		return nil, err
	}
	return &team, nil
}

func containsUserID(values []int64, uid int64) bool {
	for _, value := range values {
		if value == uid {
			return true
		}
	}
	return false
}
