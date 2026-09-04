package backup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hafio/gosplit/internal/config"
)

// This file is the portability boundary. Every value crossing into or out of an
// archive goes through it, so the wire form is decided by the column's declared
// ColumnKind and never by what a driver happens to return -- scanning into a
// bare `any` yields []byte on one engine and string on the other, which is
// precisely how a cross-engine dump corrupts itself.

// ScanDest returns a fresh scan destination for a column of the given kind.
// Every kind scans through a sql.Null* type, including the NOT NULL ones: that
// keeps one code path, and it lets EncodeColumn report an unexpected NULL as a
// registry-versus-schema disagreement instead of silently writing a zero value.
func ScanDest(kind ColumnKind) any {
	switch kind {
	case KindInt64, KindNullInt64:
		return new(sql.NullInt64)
	case KindBool:
		// NullBool reads both physical forms: database/sql converts SQLite's
		// INTEGER 0/1 through driver.Bool, and pgx returns a native bool.
		return new(sql.NullBool)
	default:
		return new(sql.NullString)
	}
}

// EncodeColumn converts a scanned value into its archive form.
//
//   - KindInt64/KindNullInt64 become a QUOTED decimal string. encoding/json
//     decodes numbers into float64 by default, which loses precision above
//     2^53 -- and every money column is int64 minor units, so a numeric
//     encoding would silently corrupt large amounts and ids.
//   - KindBool becomes JSON true/false on both engines.
//   - KindJSONText carries the stored text verbatim inside a JSON string, so
//     key order and whitespace survive byte-for-byte.
//   - SQL NULL becomes JSON null, which stays distinct from "".
func EncodeColumn(kind ColumnKind, scanned any) (json.RawMessage, error) {
	switch kind {
	case KindInt64, KindNullInt64:
		v, ok := scanned.(*sql.NullInt64)
		if !ok {
			return nil, fmt.Errorf("expected *sql.NullInt64 scan destination, got %T", scanned)
		}
		if !v.Valid {
			if kind == KindInt64 {
				return nil, errUnexpectedNull
			}
			return jsonNull, nil
		}
		return json.Marshal(strconv.FormatInt(v.Int64, 10))

	case KindBool:
		v, ok := scanned.(*sql.NullBool)
		if !ok {
			return nil, fmt.Errorf("expected *sql.NullBool scan destination, got %T", scanned)
		}
		if !v.Valid {
			return nil, errUnexpectedNull
		}
		return json.Marshal(v.Bool)

	case KindText, KindNullText, KindJSONText:
		v, ok := scanned.(*sql.NullString)
		if !ok {
			return nil, fmt.Errorf("expected *sql.NullString scan destination, got %T", scanned)
		}
		if !v.Valid {
			if kind != KindNullText {
				return nil, errUnexpectedNull
			}
			return jsonNull, nil
		}
		return json.Marshal(v.String)

	default:
		return nil, fmt.Errorf("unknown column kind %d", kind)
	}
}

// DecodeColumn converts an archive value into a bind argument for the target
// engine. It is strict on purpose: the input is attacker-controlled, so a value
// that does not match the column's declared kind is an error rather than
// something coerced into range.
func DecodeColumn(engine config.Engine, kind ColumnKind, raw json.RawMessage) (any, error) {
	isNull := isJSONNull(raw)

	switch kind {
	case KindInt64, KindNullInt64:
		if isNull {
			if kind == KindInt64 {
				return nil, errNullNotAllowed
			}
			return nil, nil
		}
		// Reject a bare JSON number explicitly. Unmarshalling it into an int64
		// would appear to work, but it means the archive was written by
		// something that did not use EncodeColumn, and a float64 round trip in
		// that writer may already have mangled the value.
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("integer column must be a quoted decimal string, got %s: %w", truncateForError(raw), err)
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("integer column value %q is not a base-10 int64: %w", s, err)
		}
		return n, nil

	case KindBool:
		if isNull {
			return nil, errNullNotAllowed
		}
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("boolean column must be JSON true or false, got %s: %w", truncateForError(raw), err)
		}
		// The one place binding branches on engine: the column is INTEGER on
		// SQLite and BOOLEAN on Postgres. Binding the physical form explicitly
		// keeps this independent of how a driver chooses to coerce a Go bool.
		if engine == config.EnginePostgres {
			return b, nil
		}
		if b {
			return int64(1), nil
		}
		return int64(0), nil

	case KindText, KindNullText, KindJSONText:
		if isNull {
			if kind != KindNullText {
				return nil, errNullNotAllowed
			}
			return nil, nil
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("text column must be a JSON string, got %s: %w", truncateForError(raw), err)
		}
		return s, nil

	default:
		return nil, fmt.Errorf("unknown column kind %d", kind)
	}
}

// jsonNull is the literal written for a SQL NULL.
var jsonNull = json.RawMessage("null")

var (
	errUnexpectedNull = fmt.Errorf("column is NULL but the registry declares it NOT NULL (the backup registry disagrees with the live schema)")
	errNullNotAllowed = fmt.Errorf("archive holds null for a NOT NULL column")
)

// isJSONNull reports whether raw is the JSON null literal, ignoring
// surrounding whitespace. Compared literally rather than by unmarshalling, so
// that a JSON string containing the text "null" is not mistaken for one.
func isJSONNull(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case 'n':
			return string(trimJSONSpace(raw)) == "null"
		default:
			return false
		}
	}
	return false
}

func trimJSONSpace(raw json.RawMessage) json.RawMessage {
	start, end := 0, len(raw)
	for start < end && isJSONSpace(raw[start]) {
		start++
	}
	for end > start && isJSONSpace(raw[end-1]) {
		end--
	}
	return raw[start:end]
}

func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// truncateForError bounds a hostile value before it reaches an error message,
// so a multi-megabyte archive field cannot flood the log.
func truncateForError(raw json.RawMessage) string {
	const max = 64
	if len(raw) <= max {
		return string(raw)
	}
	return string(raw[:max]) + "... (truncated)"
}
