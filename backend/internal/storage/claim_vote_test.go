package storage

import (
	"context"
	"oss-max/internal/domain"
	"testing"
)

func TestSelectedOwnerVoteAndRevoke(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	m := newMeeting(t, db, 10)
	flats, _ := demoFlats(t)
	if err := db.ImportRegistry(ctx, m.ID, flats); err != nil {
		t.Fatal(err)
	}
	if err := db.Publish(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	owners, err := db.FlatOwners(ctx, m.ID, "1")
	if err != nil || len(owners) != 2 {
		t.Fatalf("owners: %+v %v", owners, err)
	}
	if n, _ := db.PendingCount(ctx, m.ID); n != 0 {
		t.Fatal("reading owners created a claim")
	}
	for i, maxID := range []int64{20, 21} {
		fresh, err := db.SubmitVote(ctx, m.ID, User{MaxID: maxID}, domain.ChoiceFor, "1", owners[i].ID)
		if err != nil || !fresh {
			t.Fatalf("submit: %v %v", fresh, err)
		}
	}
	pending, err := db.PendingClaims(ctx, m.ID)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	for _, c := range pending {
		if c.Weight != 43.95 || c.RequestedOwnerID == nil || c.OwnerID != nil || c.Choice != "" {
			t.Fatalf("wrong selected share or vote leaked: %+v", c)
		}
	}
	if n, a, err := db.PendingVotes(ctx, m.ID); err != nil || n != 2 || a != 87.9 {
		t.Fatalf("pending area: %d %v %v", n, a, err)
	}
	if r, _ := db.Result(ctx, m.ID); r.Tally.Total != 0 {
		t.Fatal("unconfirmed votes counted")
	}
	fresh, err := db.SubmitVote(ctx, m.ID, User{MaxID: 20}, domain.ChoiceAgainst, "1", owners[0].ID)
	if err != nil || fresh {
		t.Fatalf("repeat submission: %v %v", fresh, err)
	}
	for _, c := range pending {
		if err := db.ConfirmClaim(ctx, m.ID, c.ID, *c.RequestedOwnerID); err != nil {
			t.Fatal(err)
		}
	}
	if r, _ := db.Result(ctx, m.ID); r.Tally.Total != 87.9 || r.Tally.Against != 43.95 || r.Tally.For != 43.95 {
		t.Fatalf("tally: %+v", r.Tally)
	}
	confirmed, err := db.ReviewClaims(ctx, m.ID, ClaimConfirmed)
	if err != nil || len(confirmed) != 2 {
		t.Fatalf("confirmed: %+v %v", confirmed, err)
	}
	if err := db.RevokeClaim(ctx, m.ID, pending[0].ID); err != nil {
		t.Fatal(err)
	}
	if r, _ := db.Result(ctx, m.ID); r.Tally.Total != 43.95 {
		t.Fatalf("revoke tally: %+v", r.Tally)
	}
	mine, _ := db.UserClaims(ctx, m.ID, 20)
	if len(mine) != 1 || mine[0].Status != ClaimPending || mine[0].Choice != domain.ChoiceAgainst || mine[0].OwnerID != nil {
		t.Fatalf("revoked: %+v", mine)
	}
	owners, _ = db.FlatOwners(ctx, m.ID, "1")
	if owners[0].Taken {
		t.Fatal("revoked owner still taken")
	}
	if err := db.Finish(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.ConfirmClaim(ctx, m.ID, pending[0].ID, owners[0].ID); err != nil {
		t.Fatal(err)
	}
	if r, _ := db.Result(ctx, m.ID); r.Tally.Total != 87.9 || r.Tally.Against != 43.95 {
		t.Fatalf("reconfirm tally: %+v", r.Tally)
	}
}

func TestSubmitVoteRollback(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	m := newMeeting(t, db, 10)
	flats, _ := demoFlats(t)
	if err := db.ImportRegistry(ctx, m.ID, flats); err != nil {
		t.Fatal(err)
	}
	if err := db.Publish(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	owners, _ := db.FlatOwners(ctx, m.ID, "1")
	if _, err := db.SubmitVote(ctx, m.ID, User{MaxID: 20}, domain.ChoiceFor, "2", owners[0].ID); err == nil {
		t.Fatal("accepted owner from another flat")
	}
	if has, _ := db.HasClaim(ctx, m.ID, 20); has {
		t.Fatal("failed vote left a claim")
	}
	if _, err := db.SubmitVote(ctx, m.ID, User{MaxID: 10}, domain.ChoiceFor, "1", owners[0].ID); err != nil {
		t.Fatal(err)
	}
	if r, _ := db.Result(ctx, m.ID); r.Tally.Total != 43.95 {
		t.Fatalf("initiator share: %+v", r.Tally)
	}
	if _, err := db.SubmitVote(ctx, m.ID, User{MaxID: 99}, domain.ChoiceFor, "1", owners[0].ID); err != ErrOwnerTaken {
		t.Fatalf("taken owner: %v", err)
	}
	if has, _ := db.HasClaim(ctx, m.ID, 99); has {
		t.Fatal("taken owner left claim")
	}
}
