package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hafio/gosplit/internal/store"
)

// TestExportUserDataIncludesEverything: the export carries the profile image and
// every expense the user is part of -- a direct one and one in an archived group
// alike -- each with its participant rows, which sum to zero.
func TestExportUserDataIncludesEverything(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	a := covUser(t, svc, "A", "a@x.com")
	b := covUser(t, svc, "B", "b@x.com")
	covFriends(t, svc, a, b)
	if _, err := svc.Store.DB.ExecContext(ctx, svc.Store.Rebind(`UPDATE users SET image = ? WHERE id = ?`), "avatar.png", a.ID); err != nil {
		t.Fatal(err)
	}
	g, err := svc.Store.CreateGroup(ctx, &store.Group{Name: "Trip", CreatedBy: a.ID, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{a.ID, b.ID} {
		if err := svc.Store.AddGroupMember(ctx, g.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	mkExp(t, svc, "Dinner", 1000, "USD", "2026-01-10", a.ID, nil,
		[]store.ExpenseParticipant{{UserID: a.ID, Amount: 500}, {UserID: b.ID, Amount: -500}})
	mkExp(t, svc, "Hotel", 2000, "USD", "2026-01-11", b.ID, &g.ID,
		[]store.ExpenseParticipant{{UserID: b.ID, Amount: 1000}, {UserID: a.ID, Amount: -1000}})
	if err := svc.Store.SetGroupArchived(ctx, g.ID, true); err != nil {
		t.Fatal(err)
	}

	blob, err := svc.ExportUserData(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		User struct {
			Image *string `json:"image"`
		} `json:"user"`
		Expenses []struct {
			Expense struct {
				Name string
			} `json:"expense"`
			Participants []struct {
				Amount int64
			} `json:"participants"`
		} `json:"expenses"`
	}
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatal(err)
	}
	if out.User.Image == nil || *out.User.Image != "avatar.png" {
		t.Errorf("exported image = %v, want avatar.png", out.User.Image)
	}
	found := map[string]bool{}
	for _, e := range out.Expenses {
		var sum int64
		for _, p := range e.Participants {
			sum += p.Amount
		}
		if len(e.Participants) != 2 || sum != 0 {
			t.Errorf("%s exported with participants %v, want two summing to zero", e.Expense.Name, e.Participants)
		}
		found[e.Expense.Name] = true
	}
	if len(out.Expenses) != 2 || !found["Dinner"] || !found["Hotel"] {
		t.Errorf("exported expenses = %v, want Dinner and the archived group's Hotel", found)
	}
}
