package httpapp

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var valueInputRe = regexp.MustCompile(`name="value_(\d+)" value="([^"]*)"`)

// versionInputRe captures the optimistic-concurrency token a rendered form
// carries, so a test can post what the server actually handed the browser
// instead of hardcoding a number.
var versionInputRe = regexp.MustCompile(`name="version" value="(\d+)"`)

// formValues maps the rendered per-participant value inputs by user id.
func formValues(b string) map[string]string {
	out := map[string]string{}
	for _, m := range valueInputRe.FindAllStringSubmatch(b, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// TestEditShareShowsTypedWeights is the headline fix: share weights come back as
// the weights that were typed, not as the minor-unit shares they produced.
func TestEditShareShowsTypedWeights(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"SHARE"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"2"},
		"include_2": {"1"}, "value_2": {"1"},
	})
	b := body(t, h.get(path+"/move"))
	if !strings.Contains(b, `name="method" value="SHARE" checked`) {
		t.Error("SHARE not preselected on the edit form")
	}
	vals := formValues(b)
	if vals["1"] != "2" || vals["2"] != "1" {
		t.Errorf("share weights = %v, want {1:2, 2:1} (minor units would be 2000/1000)", vals)
	}
}

// TestEditAdjustmentKeepsMethod: ADJUSTMENT used to be silently re-saved as
// EXACT, so an unchanged edit rewrote split_type in the database.
func TestEditAdjustmentKeepsMethod(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"ADJUSTMENT"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"5.00"},
		"include_2": {"1"}, "value_2": {"0.00"},
	})
	b := body(t, h.get(path+"/move"))
	if !strings.Contains(b, `name="method" value="ADJUSTMENT" checked`) {
		t.Errorf("ADJUSTMENT not preselected:\n%s", b)
	}
	if strings.Contains(b, `name="method" value="EXACT" checked`) {
		t.Error("ADJUSTMENT was downgraded to EXACT")
	}
	if vals := formValues(b); vals["1"] != "5.00" {
		t.Errorf("adjustment values = %v, want user 1 at 5.00", vals)
	}
	// Re-saving the form as rendered must leave the method alone.
	h.post(path+"/move", url.Values{
		"target_group": {"none"}, "name": {"Dinner"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"ADJUSTMENT"}, "paid_by": {"1"}, "version": {"1"},
		"include_1": {"1"}, "value_1": {"5.00"},
		"include_2": {"1"}, "value_2": {"0.00"},
	})
	var st string
	_ = h.st.DB.QueryRow(`SELECT split_type FROM expenses`).Scan(&st)
	if st != "ADJUSTMENT" {
		t.Errorf("split_type after an unchanged edit = %q, want ADJUSTMENT", st)
	}
}

// TestEditPercentageRestoresTypedDigits: the stored percentages come back as
// entered, rather than re-derived from the amounts and nudged to re-total 100.
func TestEditPercentageRestoresTypedDigits(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Split"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"PERCENTAGE"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"33.33"},
		"include_2": {"1"}, "value_2": {"66.67"},
	})
	vals := formValues(body(t, h.get(path+"/move")))
	if vals["1"] != "33.33" || vals["2"] != "66.67" {
		t.Errorf("percentages = %v, want the typed {1:33.33, 2:66.67}", vals)
	}
}

// TestEditEqualPayerNotSplitting: a payer who paid without taking a share comes
// back unchecked because they recorded no input -- previously a guess from a
// zero share.
func TestEditEqualPayerNotSplitting(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Treat"}, "amount": {"20.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_2": {"1"},
	})
	b := body(t, h.get(path+"/move"))
	if strings.Contains(b, `name="include_1" value="1" checked`) {
		t.Error("payer who owes nothing should come back unchecked")
	}
	if !strings.Contains(b, `name="include_2" value="1" checked`) {
		t.Error("the actual splitter should come back checked")
	}
}

// TestEditRoundTripsUnchanged is the user-facing promise: open an edit form,
// save it untouched, and nothing about the split moves.
func TestEditRoundTripsUnchanged(t *testing.T) {
	// 10.01 over equal weights leaves a remainder unit to distribute, so this
	// exercises the path where amounts are not a clean division of the total.
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"10.01"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"SHARE"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"1"},
		"include_2": {"1"}, "value_2": {"1"},
	})
	before := participantAmounts(t, h)
	vals := formValues(body(t, h.get(path+"/move")))
	form := url.Values{
		"target_group": {"none"}, "name": {"Dinner"}, "amount": {"10.01"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"SHARE"}, "paid_by": {"1"}, "version": {"1"},
	}
	for id, v := range vals {
		form.Set("include_"+id, "1")
		form.Set("value_"+id, v)
	}
	if r := h.post(path+"/move", form); r.StatusCode != http.StatusOK {
		t.Fatalf("re-save status %d", r.StatusCode)
	}
	after := participantAmounts(t, h)
	if len(before) != len(after) {
		t.Fatalf("participant count changed: %v -> %v", before, after)
	}
	for uid, amt := range before {
		if after[uid] != amt {
			t.Errorf("user %d moved from %d to %d on an unchanged re-save", uid, amt, after[uid])
		}
	}
}

// TestEditLegacyFallsBackToDerivation: expenses saved before inputs were stored
// keep the old reconstruction, including presenting ADJUSTMENT as EXACT.
func TestEditLegacyFallsBackToDerivation(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"ADJUSTMENT"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"5.00"},
		"include_2": {"1"}, "value_2": {"0.00"},
	})
	if _, err := h.st.DB.Exec(`DELETE FROM expense_split_inputs`); err != nil {
		t.Fatalf("clear inputs: %v", err)
	}
	b := body(t, h.get(path+"/move"))
	if !strings.Contains(b, `name="method" value="EXACT" checked`) {
		t.Errorf("legacy ADJUSTMENT should fall back to EXACT:\n%s", b)
	}
}

// TestEditLegacyPercentageStillSumsTo100: the derived path's basis-point
// fix-up stays exercised now that the happy path no longer reaches it.
func TestEditLegacyPercentageStillSumsTo100(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Split"}, "amount": {"10.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"PERCENTAGE"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"33.33"},
		"include_2": {"1"}, "value_2": {"66.67"},
	})
	if _, err := h.st.DB.Exec(`DELETE FROM expense_split_inputs`); err != nil {
		t.Fatalf("clear inputs: %v", err)
	}
	vals := formValues(body(t, h.get(path+"/move")))
	if len(vals) < 2 {
		t.Fatalf("expected 2 value inputs, got %v", vals)
	}
	var sum float64
	for _, v := range vals {
		f, _ := strconv.ParseFloat(v, 64)
		sum += f
	}
	if sum < 99.995 || sum > 100.005 {
		t.Errorf("reconstructed percentages sum to %.2f, want 100.00", sum)
	}
}

// participantAmounts reads every live participant row as user id -> amount.
func participantAmounts(t *testing.T, h *harness) map[int64]int64 {
	t.Helper()
	rows, err := h.st.DB.Query(`SELECT user_id, amount FROM expense_participants ORDER BY user_id`)
	if err != nil {
		t.Fatalf("read participants: %v", err)
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var uid, amt int64
		if err := rows.Scan(&uid, &amt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[uid] = amt
	}
	return out
}

// TestAddExpenseErrorKeepsTypedValues: a rejected add comes back as submitted --
// nothing persisted, but nothing retyped either -- with the message on the field
// that caused it.
func TestAddExpenseErrorKeepsTypedValues(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})

	// Exact amounts that do not sum to the total.
	resp := h.post("/expenses", url.Values{
		"name": {"Lunch"}, "amount": {"30.00"}, "currency": {"USD"},
		"date": {"2026-03-09"}, "method": {"EXACT"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {"5.00"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if vals := formValues(b); vals["1"] != "5.00" {
		t.Errorf("typed values wiped: %v", vals)
	}
	if strings.Contains(b, `name="include_2" value="1" checked`) {
		t.Error("the participant the user left unchecked came back checked")
	}
	if !strings.Contains(b, "field-err") {
		t.Errorf("no field-level error rendered:\n%s", b)
	}
	if !strings.Contains(b, `value="Lunch"`) {
		t.Error("description not preserved on the re-render")
	}
	var n int
	_ = h.st.DB.QueryRow(`SELECT count(*) FROM expenses`).Scan(&n)
	if n != 0 {
		t.Errorf("%d expenses persisted despite the validation failure", n)
	}
}

// TestEditErrorRerendersForm: a rejected edit returns the working form, not the
// dead-end message page it used to.
func TestEditErrorRerendersForm(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	resp := h.post(path+"/move", url.Values{
		"target_group": {"none"}, "name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EXACT"}, "paid_by": {"1"}, "version": {"1"},
		"include_1": {"1"}, "value_1": {"1.00"},
		"include_2": {"1"}, "value_2": {"2.00"},
	})
	b := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(b, "expense-form") {
		t.Errorf("rejected edit did not re-render the form:\n%s", b)
	}
	if vals := formValues(b); vals["1"] != "1.00" || vals["2"] != "2.00" {
		t.Errorf("submitted values lost on the re-render: %v", vals)
	}
}

// TestEditConflictRejectsSecondWriter: two people editing the same expense --
// the second save is refused with 409 instead of silently erasing the first.
func TestEditConflictRejectsSecondWriter(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	// Both forms were rendered from the row's initial version.
	form := func(name string) url.Values {
		return url.Values{
			"target_group": {"none"}, "name": {name}, "amount": {"40.00"}, "currency": {"USD"},
			"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"}, "version": {"1"},
			"include_1": {"1"}, "include_2": {"1"},
		}
	}
	if r := h.post(path+"/move", form("Winner")); r.StatusCode != http.StatusOK {
		t.Fatalf("first save status %d", r.StatusCode)
	}
	resp := h.post(path+"/move", form("Loser"))
	b := body(t, resp)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second save status %d, want 409", resp.StatusCode)
	}
	if !strings.Contains(b, "expense-form") {
		t.Error("conflict should re-render the form with the user's edit intact")
	}
	if !strings.Contains(b, `value="Loser"`) {
		t.Error("the second editor lost their typing on the conflict page")
	}
	var name string
	_ = h.st.DB.QueryRow(`SELECT name FROM expenses`).Scan(&name)
	if name != "Winner" {
		t.Errorf("stored name = %q, want the first writer's %q", name, "Winner")
	}
}

// TestEditWithoutVersionFailsClosed: a post carrying no version token used to
// match every never-edited row, because rows started at version 0 and so does a
// missing token -- the exact fail-open the column exists to prevent. Rows start
// at 1 now, so such a post is refused, while the form the server rendered still
// round-trips.
func TestEditWithoutVersionFailsClosed(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	form := func(name, version string) url.Values {
		f := url.Values{
			"target_group": {"none"}, "name": {name}, "amount": {"40.00"}, "currency": {"USD"},
			"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
			"include_1": {"1"}, "include_2": {"1"},
		}
		if version != "" {
			f.Set("version", version)
		}
		return f
	}

	resp := h.post(path+"/move", form("Hand-built", ""))
	_ = body(t, resp)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("version-less edit: status %d, want 409", resp.StatusCode)
	}
	var name string
	_ = h.st.DB.QueryRow(`SELECT name FROM expenses`).Scan(&name)
	if name != "Dinner" {
		t.Errorf("a version-less post overwrote the row: %q", name)
	}

	// The token the edit form actually renders is accepted, and saves.
	rendered := body(t, h.get(path+"/move"))
	m := versionInputRe.FindStringSubmatch(rendered)
	if m == nil {
		t.Fatal("edit form carries no version token")
	}
	if r := h.post(path+"/move", form("Brunch", m[1])); r.StatusCode != http.StatusOK {
		t.Fatalf("edit at the rendered version: status %d", r.StatusCode)
	}
	_ = h.st.DB.QueryRow(`SELECT name FROM expenses`).Scan(&name)
	if name != "Brunch" {
		t.Errorf("stored name = %q, want Brunch", name)
	}
}

// TestDeleteRejectsStaleVersion: the delete button carries the version its page
// was rendered from, so a delete decided before someone else's edit landed is
// refused (409) instead of discarding that edit unseen. Reloading the page hands
// back the current version and the delete goes through.
func TestDeleteRejectsStaleVersion(t *testing.T) {
	h, path := seedDirectExpense(t, url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "include_2": {"1"},
	})
	detail := body(t, h.get(path))
	m := versionInputRe.FindStringSubmatch(detail)
	if m == nil {
		t.Fatalf("delete form carries no version token:\n%s", detail)
	}
	stale := m[1]

	// Someone else saves an edit, which bumps the version this page holds.
	edit := url.Values{
		"target_group": {"none"}, "name": {"Brunch"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"}, "version": {stale},
		"include_1": {"1"}, "include_2": {"1"},
	}
	if r := h.post(path+"/move", edit); r.StatusCode != http.StatusOK {
		t.Fatalf("competing edit: status %d", r.StatusCode)
	}

	resp := h.post(path+"/delete", url.Values{"version": {stale}})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stale delete: status %d, want 409", resp.StatusCode)
	}
	if n := deletedCount(t, h); n != 0 {
		t.Fatalf("%d expenses soft-deleted by a stale delete", n)
	}

	m = versionInputRe.FindStringSubmatch(body(t, h.get(path)))
	if m == nil {
		t.Fatal("reloaded detail page carries no version token")
	}
	if r := h.post(path+"/delete", url.Values{"version": {m[1]}}); r.StatusCode != http.StatusOK {
		t.Fatalf("delete at the current version: status %d", r.StatusCode)
	}
	if n := deletedCount(t, h); n != 1 {
		t.Errorf("soft-deleted expenses = %d, want 1", n)
	}
}

// deletedCount reports how many expenses are soft-deleted.
func deletedCount(t *testing.T, h *harness) int {
	t.Helper()
	var n int
	if err := h.st.DB.QueryRow(`SELECT count(*) FROM expenses WHERE deleted_at IS NOT NULL`).Scan(&n); err != nil {
		t.Fatalf("count deleted: %v", err)
	}
	return n
}
