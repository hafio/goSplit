package backup

import (
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
)

// nullString and nullInt build scan destinations already holding a value, as
// though a driver had just filled them in.
func nullString(s string, valid bool) *sql.NullString {
	return &sql.NullString{String: s, Valid: valid}
}

func nullInt(n int64, valid bool) *sql.NullInt64 {
	return &sql.NullInt64{Int64: n, Valid: valid}
}

func nullBool(b bool) *sql.NullBool {
	return &sql.NullBool{Bool: b, Valid: true}
}

// TestEncodeColumn covers the wire form of every kind, including the values
// that break naive encodings.
func TestEncodeColumn(t *testing.T) {
	tests := []struct {
		name    string
		kind    ColumnKind
		scanned any
		want    string
	}{
		// Quoted, not a bare number: encoding/json decodes numbers into
		// float64, which silently loses precision above 2^53.
		{"max int64", KindInt64, nullInt(math.MaxInt64, true), `"9223372036854775807"`},
		{"min int64", KindInt64, nullInt(math.MinInt64, true), `"-9223372036854775808"`},
		{"above float64 precision", KindInt64, nullInt(9007199254740993, true), `"9007199254740993"`},
		{"zero int64", KindInt64, nullInt(0, true), `"0"`},
		{"negative int64", KindInt64, nullInt(-250, true), `"-250"`},
		{"null nullable int", KindNullInt64, nullInt(0, false), `null`},
		{"set nullable int", KindNullInt64, nullInt(42, true), `"42"`},

		{"bool true", KindBool, nullBool(true), `true`},
		{"bool false", KindBool, nullBool(false), `false`},

		{"text", KindText, nullString("hello", true), `"hello"`},
		// The empty string must not become null.
		{"empty text", KindText, nullString("", true), `""`},
		{"null nullable text", KindNullText, nullString("", false), `null`},
		{"empty nullable text", KindNullText, nullString("", true), `""`},
		{"text needing escapes", KindText, nullString("a\"b\\c\nd", true), `"a\"b\\c\nd"`},

		// JSON is carried inside a string, verbatim: key order and interior
		// spacing survive, which re-serializing would destroy.
		{"json verbatim", KindJSONText, nullString(`{"b":2,"a":1}`, true), `"{\"b\":2,\"a\":1}"`},
		{"json with spacing", KindJSONText, nullString(`{"a": 1,  "b":2}`, true), `"{\"a\": 1,  \"b\":2}"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeColumn(tc.kind, tc.scanned)
			if err != nil {
				t.Fatalf("EncodeColumn: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("EncodeColumn = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestEncodeColumnRejectsUnexpectedNull asserts a NULL in a column the registry
// declares NOT NULL is reported rather than written as a zero value -- it means
// the registry and the live schema disagree.
func TestEncodeColumnRejectsUnexpectedNull(t *testing.T) {
	for _, kind := range []ColumnKind{KindText, KindJSONText, KindInt64} {
		var scanned any
		switch kind {
		case KindInt64:
			scanned = nullInt(0, false)
		default:
			scanned = nullString("", false)
		}
		if _, err := EncodeColumn(kind, scanned); err == nil {
			t.Errorf("kind %d: encoding a NULL in a NOT NULL column succeeded, want an error", kind)
		}
	}
}

// TestDecodeColumnRoundTrip asserts every kind survives encode then decode.
func TestDecodeColumnRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		kind    ColumnKind
		scanned any
		want    any
	}{
		{"max int64", KindInt64, nullInt(math.MaxInt64, true), int64(math.MaxInt64)},
		{"above float64 precision", KindInt64, nullInt(9007199254740993, true), int64(9007199254740993)},
		{"null int", KindNullInt64, nullInt(0, false), nil},
		{"text", KindText, nullString("hi", true), "hi"},
		{"empty text", KindText, nullString("", true), ""},
		{"null text", KindNullText, nullString("", false), nil},
		{"json", KindJSONText, nullString(`{"b":2,"a":1}`, true), `{"b":2,"a":1}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := EncodeColumn(tc.kind, tc.scanned)
			if err != nil {
				t.Fatalf("EncodeColumn: %v", err)
			}
			got, err := DecodeColumn(config.EngineSQLite, tc.kind, raw)
			if err != nil {
				t.Fatalf("DecodeColumn: %v", err)
			}
			if got != tc.want {
				t.Errorf("round trip = %#v (%T), want %#v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

// TestDecodeColumnRejectsBareNumber is the guard against a future
// "simplification" back to a JSON number for money. A bare number must be an
// error, not a silently truncated float64.
func TestDecodeColumnRejectsBareNumber(t *testing.T) {
	for _, kind := range []ColumnKind{KindInt64, KindNullInt64} {
		_, err := DecodeColumn(config.EngineSQLite, kind, json.RawMessage(`9007199254740993`))
		if err == nil {
			t.Errorf("kind %d: a bare JSON number was accepted for an integer column", kind)
			continue
		}
		if !strings.Contains(err.Error(), "quoted decimal string") {
			t.Errorf("kind %d: error should explain the required form, got: %v", kind, err)
		}
	}
}

// TestDecodeColumnRejectsNullForNotNull asserts an archive cannot smuggle a
// NULL into a NOT NULL column.
func TestDecodeColumnRejectsNullForNotNull(t *testing.T) {
	for _, kind := range []ColumnKind{KindText, KindJSONText, KindInt64, KindBool} {
		if _, err := DecodeColumn(config.EngineSQLite, kind, json.RawMessage(`null`)); err == nil {
			t.Errorf("kind %d: null was accepted for a NOT NULL column", kind)
		}
	}
}

// TestDecodeColumnRejectsWrongType asserts values of the wrong shape are
// refused rather than coerced.
func TestDecodeColumnRejectsWrongType(t *testing.T) {
	tests := []struct {
		name string
		kind ColumnKind
		raw  string
	}{
		{"object for text", KindText, `{"a":1}`},
		{"array for text", KindText, `[1,2]`},
		{"bool for text", KindText, `true`},
		{"string for bool", KindBool, `"true"`},
		{"number for bool", KindBool, `1`},
		{"non-numeric string for int", KindInt64, `"not-a-number"`},
		{"float string for int", KindInt64, `"1.5"`},
		{"hex string for int", KindInt64, `"0x10"`},
		{"overflowing int", KindInt64, `"9223372036854775808"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeColumn(config.EngineSQLite, tc.kind, json.RawMessage(tc.raw)); err == nil {
				t.Errorf("DecodeColumn accepted %s for kind %d", tc.raw, tc.kind)
			}
		})
	}
}

// TestDecodeBoolBindsPerEngine covers the one place binding depends on the
// engine: simplify_debts and notified are INTEGER on SQLite and BOOLEAN on
// Postgres. This satisfies R7's cross-engine coverage without a live Postgres.
func TestDecodeBoolBindsPerEngine(t *testing.T) {
	tests := []struct {
		engine config.Engine
		raw    string
		want   any
	}{
		{config.EngineSQLite, `true`, int64(1)},
		{config.EngineSQLite, `false`, int64(0)},
		{config.EnginePostgres, `true`, true},
		{config.EnginePostgres, `false`, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.engine)+"/"+tc.raw, func(t *testing.T) {
			got, err := DecodeColumn(tc.engine, KindBool, json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("DecodeColumn: %v", err)
			}
			if got != tc.want {
				t.Errorf("bound %#v (%T), want %#v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

// TestScanDestPerKind asserts each kind scans through the Null* type that can
// represent its full range, NULL included.
func TestScanDestPerKind(t *testing.T) {
	tests := []struct {
		kind ColumnKind
		want string
	}{
		{KindInt64, "*sql.NullInt64"},
		{KindNullInt64, "*sql.NullInt64"},
		{KindBool, "*sql.NullBool"},
		{KindText, "*sql.NullString"},
		{KindNullText, "*sql.NullString"},
		{KindJSONText, "*sql.NullString"},
	}
	for _, tc := range tests {
		got := ScanDest(tc.kind)
		if name := typeName(got); name != tc.want {
			t.Errorf("ScanDest(kind %d) = %s, want %s", tc.kind, name, tc.want)
		}
	}
}

func typeName(v any) string {
	switch v.(type) {
	case *sql.NullInt64:
		return "*sql.NullInt64"
	case *sql.NullBool:
		return "*sql.NullBool"
	case *sql.NullString:
		return "*sql.NullString"
	default:
		return "unknown"
	}
}

// TestIsJSONNull asserts the null check is a literal comparison, so a JSON
// string whose contents happen to be "null" is not mistaken for one.
func TestIsJSONNull(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{`null`, true},
		{` null `, true},
		{"\n\tnull\n", true},
		{`"null"`, false},
		{`nullx`, false},
		{`0`, false},
		{`""`, false},
		{``, false},
	}
	for _, tc := range tests {
		if got := isJSONNull(json.RawMessage(tc.raw)); got != tc.want {
			t.Errorf("isJSONNull(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

// TestTruncateForError bounds a hostile value before it reaches a log line.
func TestTruncateForError(t *testing.T) {
	long := json.RawMessage(strings.Repeat("x", 500))
	got := truncateForError(long)
	if len(got) > 100 {
		t.Errorf("truncateForError returned %d bytes, want it bounded", len(got))
	}
	if !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("truncated value should say so, got %q", got)
	}
	short := json.RawMessage(`"ok"`)
	if truncateForError(short) != `"ok"` {
		t.Error("a short value should pass through unchanged")
	}
}
