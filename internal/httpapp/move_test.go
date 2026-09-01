package httpapp

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestMoveKeepsRecord confirms moving an expense edits it in place: same URL/id,
// relocated into the target group, with no soft-deleted "ghost" and no duplicate.
func TestMoveKeepsRecord(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123") // id 1
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}}) // group 1
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})

	// A direct expense between alice(1) and bob(2).
	resp := h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	path := resp.Request.URL.Path // /expenses/<id>
	_ = body(t, resp)
	if !strings.HasPrefix(path, "/expenses/") {
		t.Fatalf("no expense id in %q", path)
	}

	// Move it into group Trip with a re-split + acknowledgment.
	mv := h.post(path+"/move", url.Values{
		"target_group": {"1"}, "name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"}, "ack": {"1"}, "version": {"1"},
	})
	b := body(t, mv)
	if mv.StatusCode != http.StatusOK {
		t.Fatalf("move status %d", mv.StatusCode)
	}
	// Same record: redirect lands back on the original detail URL, showing the group.
	if mv.Request.URL.Path != path {
		t.Errorf("redirected to %q, want same record %q", mv.Request.URL.Path, path)
	}
	if !strings.Contains(b, "Trip") {
		t.Errorf("moved expense detail doesn't show the new group")
	}

	// One live expense, none soft-deleted, now in group 1.
	var total, deleted int
	_ = h.st.DB.QueryRow(`SELECT count(*) FROM expenses`).Scan(&total)
	_ = h.st.DB.QueryRow(`SELECT count(*) FROM expenses WHERE deleted_at IS NOT NULL`).Scan(&deleted)
	if total != 1 || deleted != 0 {
		t.Fatalf("expected 1 live expense after move, got total=%d deleted=%d", total, deleted)
	}
	var gid sql.NullInt64
	_ = h.st.DB.QueryRow(`SELECT group_id FROM expenses`).Scan(&gid)
	if !gid.Valid || gid.Int64 != 1 {
		t.Errorf("expense group not updated to 1: %+v", gid)
	}
}

// seedDirectExpense registers Alice (id 1) + friend Bob (id 2) and creates one
// direct expense with the given form fields; returns the harness and detail path.
func seedDirectExpense(t *testing.T, form url.Values) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	resp := h.post("/expenses", form)
	path := resp.Request.URL.Path
	_ = body(t, resp)
	if !strings.HasPrefix(path, "/expenses/") {
		t.Fatalf("no expense id in %q", path)
	}
	return h, path
}

// TestEditPagePrefillNoAck: a plain edit (no ?target) shows no move banner / ack
// and pre-checks the current participants.
func TestEditPagePrefillNoAck(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	b := body(t, h.get(path+"/move"))
	if strings.Contains(b, `name="ack"`) {
		t.Error("plain edit should not show the ack checkbox")
	}
	if strings.Contains(b, "different group") {
		t.Error("plain edit should not show the move warning banner")
	}
	if !strings.Contains(b, `name="include_1" value="1" checked`) ||
		!strings.Contains(b, `name="include_2" value="1" checked`) {
		t.Errorf("current participants not pre-checked:\n%s", b)
	}
	if !strings.Contains(b, "Edit expense") {
		t.Error("heading should be Edit expense")
	}
}

// TestEditPageRetargetResetsToEqual: switching the target to a different group
// shows the banner + ack and resets the split to EQUAL even for a stored EXACT.
func TestEditPageRetargetResetsToEqual(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EXACT"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"25.00"},
		"include_2": {"1"}, "value_2": {"15.00"},
	})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})

	b := body(t, h.get(path+"/move?target=1"))
	if !strings.Contains(b, `name="ack"`) {
		t.Error("retarget should show the ack checkbox")
	}
	if !strings.Contains(b, "different group") {
		t.Error("retarget should show the move warning banner")
	}
	if !strings.Contains(b, `name="method" value="EQUAL" checked`) {
		t.Errorf("retarget should preselect EQUAL:\n%s", b)
	}
	if strings.Contains(b, `name="method" value="EXACT" checked`) {
		t.Error("retarget must not keep the stored EXACT method")
	}
}

// TestEditSameGroupNoAckHTTP: editing a description without changing the group
// succeeds with no ack and lands back on the same record.
func TestEditSameGroupNoAckHTTP(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	ed := h.post(path+"/move", url.Values{
		"target_group": {"none"}, "name": {"Brunch"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"}, "version": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	b := body(t, ed)
	if ed.StatusCode != http.StatusOK {
		t.Fatalf("edit status %d", ed.StatusCode)
	}
	if ed.Request.URL.Path != path {
		t.Errorf("redirected to %q, want same record %q", ed.Request.URL.Path, path)
	}
	if !strings.Contains(b, "Brunch") {
		t.Errorf("edited description not shown:\n%s", b)
	}
}

// TestEditCrossGroupRequiresAck: relocating to a different group without the ack
// is rejected.
func TestEditCrossGroupRequiresAck(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	h.post("/groups/1/invite", url.Values{"email": {"bob@example.com"}})

	mv := h.post(path+"/move", url.Values{
		"target_group": {"1"}, "name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	_ = body(t, mv)
	if mv.StatusCode != http.StatusBadRequest {
		t.Fatalf("cross-group move without ack: status %d, want 400", mv.StatusCode)
	}
}

// TestEditPercentagePrefillSumsTo100 guards the basis-point rounding fix-up: the
// reconstructed percentages must sum to exactly 100%, even when integer
// truncation would otherwise leave them at 99.99%.
func TestEditPercentagePrefillSumsTo100(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Split"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"PERCENTAGE"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"33.33"},
		"include_2": {"1"}, "value_2": {"66.67"},
	})
	b := body(t, h.get(path+"/move"))
	if !strings.Contains(b, `name="method" value="PERCENTAGE" checked`) {
		t.Error("percentage method not preselected")
	}
	re := regexp.MustCompile(`name="value_\d+" value="([0-9.]+)"`)
	ms := re.FindAllStringSubmatch(b, -1)
	if len(ms) < 2 {
		t.Fatalf("expected 2 value inputs, got %d:\n%s", len(ms), b)
	}
	var sum float64
	for _, m := range ms {
		f, _ := strconv.ParseFloat(m[1], 64)
		sum += f
	}
	if sum < 99.995 || sum > 100.005 {
		t.Errorf("reconstructed percentages sum to %.2f, want 100.00", sum)
	}
}
