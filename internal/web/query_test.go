package web

import "testing"

func TestQueryWithout(t *testing.T) {
	f := map[string]string{"q": "food", "min": "10", "scope": "all"}

	// Dropping min leaves only q (scope "all" is treated as unset).
	if got := queryWithout(f, "/activity", "min"); got != "/activity?q=food" {
		t.Errorf("drop min = %q", got)
	}
	// Dropping q leaves only min.
	if got := queryWithout(f, "/activity", "q"); got != "/activity?min=10" {
		t.Errorf("drop q = %q", got)
	}
	// A group scope survives and is encoded.
	f2 := map[string]string{"scope": "group", "to": "2026-03-31"}
	if got := queryWithout(f2, "/g/1", "to"); got != "/g/1?scope=group" {
		t.Errorf("drop to = %q", got)
	}
	// Nothing active → bare action.
	if got := queryWithout(map[string]string{"scope": "all"}, "/activity", "q"); got != "/activity" {
		t.Errorf("empty = %q", got)
	}
}
