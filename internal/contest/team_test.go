package contest_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
)

func TestTeamLifecycleIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_team_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	repo := contest.NewTeamRepository(db)
	team, err := repo.Create(ctx, 1, "  Alpha  ")
	if err != nil {
		t.Fatal(err)
	}
	if team.ID >= 0 || team.DisplayName != "Alpha" || len(team.Members) != 1 || team.Members[0] != 1 {
		t.Fatalf("created team = %+v", team)
	}

	renamed, err := repo.Rename(ctx, 1, team.ID, "Beta")
	if err != nil || renamed.DisplayName != "Beta" {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	if _, err := repo.Rename(ctx, 2, team.ID, "Nope"); err == nil {
		t.Fatal("non-member rename must fail")
	}
	if _, err := repo.Invite(ctx, 1, 2, team.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Invite(ctx, 1, 2, team.ID); err == nil {
		t.Fatal("duplicate invite must fail")
	}
	mine, invites, err := repo.ListForUser(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 0 || len(invites) != 1 || invites[0].ID != team.ID {
		t.Fatalf("invite lists = mine %+v invites %+v", mine, invites)
	}
	accepted, err := repo.Accept(ctx, 2, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted.Members) != 2 {
		t.Fatalf("accepted members = %+v", accepted.Members)
	}
	mine, invites, err = repo.ListForUser(ctx, 2)
	if err != nil || len(mine) != 1 || len(invites) != 0 {
		t.Fatalf("accepted lists = mine %+v invites %+v err=%v", mine, invites, err)
	}
	left, err := repo.Leave(ctx, 1, 2, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(left.Members) != 1 || left.Members[0] != 1 {
		t.Fatalf("members after leave = %+v", left.Members)
	}

	for uid := int64(3); uid <= 6; uid++ {
		if _, err := repo.Invite(ctx, 1, uid, team.ID); err != nil {
			t.Fatalf("invite %d: %v", uid, err)
		}
	}
	if _, err := repo.Invite(ctx, 1, 7, team.ID); err == nil {
		t.Fatal("team member limit must reject additional invite")
	}
	if err := repo.Reject(ctx, 3, team.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Invite(ctx, 1, 7, team.ID); err != nil {
		t.Fatalf("invite after reject: %v", err)
	}

	second, err := repo.Create(ctx, 7, "Second")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID >= team.ID {
		t.Fatalf("negative team ids not descending: first=%d second=%d", team.ID, second.ID)
	}
}
