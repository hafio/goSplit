package split

import (
	"encoding/json"
	"strconv"
)

// InputsVersion is the payload format version written to expense_split_inputs.
// Bump it when the shape changes; DecodeInputs treats any other version as
// absent, so an older app never misreads a newer payload.
const InputsVersion = 1

// Inputs is the reference-only record of what a user actually entered for a
// split. It is restored into the edit form and is NEVER read to compute
// balances -- the zero-sum participant rows remain the only source of truth for
// money.
//
// Values maps a user id (as a decimal string, since JSON object keys are
// strings) to the single method-relevant input: PERCENTAGE basis points,
// EXACT/ADJUSTMENT minor units, SHARE the typed weight, EQUAL 0. A user's
// presence as a key means they were one of the splitters, which is how the
// payer's forced creditor row (see finalize) stays distinguishable from a payer
// who genuinely took a share.
type Inputs struct {
	Version int              `json:"v"`
	Method  Method           `json:"method"`
	Values  map[string]int64 `json:"values"`
}

// Input returns the single raw value a Line carries under method m -- the one
// field computeShares will read. EQUAL takes no per-participant input, so it is
// 0. Inverse of LineFromInput.
func (l Line) Input(m Method) int64 {
	switch m {
	case PERCENTAGE:
		return l.BasisPoints
	case EXACT:
		return l.Exact
	case SHARE:
		return l.ShareUnits
	case ADJUSTMENT:
		return l.Adjustment
	default:
		// EQUAL and the system methods carry no per-participant input.
		return 0
	}
}

// LineFromInput rebuilds a Line from a raw input value, whether freshly parsed
// from the form or restored from storage. Inverse of Line.Input.
func LineFromInput(m Method, userID, v int64) Line {
	l := Line{UserID: userID}
	switch m {
	case PERCENTAGE:
		l.BasisPoints = v
	case EXACT:
		l.Exact = v
	case SHARE:
		l.ShareUnits = v
	case ADJUSTMENT:
		l.Adjustment = v
	}
	return l
}

// EncodeInputs renders the splitters' raw inputs as the JSON payload stored
// alongside an expense. Only user-selectable methods are recorded; the system
// methods never run through Compute and so have nothing to restore.
func EncodeInputs(m Method, lines []Line) (string, error) {
	if !UserSelectable(m) || len(lines) == 0 {
		return "", nil
	}
	vals := make(map[string]int64, len(lines))
	for _, l := range lines {
		vals[strconv.FormatInt(l.UserID, 10)] = l.Input(m)
	}
	b, err := json.Marshal(Inputs{Version: InputsVersion, Method: m, Values: vals})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeInputs reads a stored payload back into user id -> raw input, reporting
// whether it is usable. It returns false -- meaning "no inputs recorded, fall
// back to reconstructing them from the amounts" -- for an empty, malformed or
// future-versioned payload, and for one whose method disagrees with the
// expense's split_type. That last check is what makes the sidecar self-healing:
// expenses.split_type stays authoritative, and a payload left behind by an
// earlier method is ignored rather than restored over the current one.
func DecodeInputs(payload string, m Method) (map[int64]int64, bool) {
	if payload == "" {
		return nil, false
	}
	var in Inputs
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		return nil, false
	}
	if in.Version != InputsVersion || in.Method != m || len(in.Values) == 0 {
		return nil, false
	}
	out := make(map[int64]int64, len(in.Values))
	for k, v := range in.Values {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, false
		}
		out[id] = v
	}
	return out, true
}
