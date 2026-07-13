// Package money handles integer minor-unit amounts. Money is NEVER a float:
// all amounts are int64 minor units (e.g. cents) and are formatted only at
// render time, per currency. Negative amounts are allowed (refunds/corrections).
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// decimals returns the number of minor-unit digits for a currency code.
// Most currencies use 2; a handful are zero- or three-decimal. Unknown codes
// default to 2.
func decimals(currency string) int {
	switch strings.ToUpper(currency) {
	case "JPY", "KRW", "VND", "CLP", "ISK", "HUF", "TWD", "UGX", "XAF", "XOF":
		return 0
	case "BHD", "KWD", "OMR", "TND", "IQD", "JOD", "LYD":
		return 3
	default:
		return 2
	}
}

// Format renders minor units as a decimal string for the given currency,
// without a currency symbol (e.g. 1234 USD -> "12.34", -50 JPY -> "-50").
func Format(minor int64, currency string) string {
	d := decimals(currency)
	if d == 0 {
		return strconv.FormatInt(minor, 10)
	}
	neg := minor < 0
	if neg {
		minor = -minor
	}
	scale := pow10(d)
	whole := minor / scale
	frac := minor % scale
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	b.WriteString(strconv.FormatInt(whole, 10))
	b.WriteByte('.')
	fracStr := strconv.FormatInt(frac, 10)
	for i := len(fracStr); i < d; i++ {
		b.WriteByte('0')
	}
	b.WriteString(fracStr)
	return b.String()
}

// FormatWithCode renders minor units with the currency code appended,
// e.g. "12.34 USD".
func FormatWithCode(minor int64, currency string) string {
	return Format(minor, currency) + " " + strings.ToUpper(currency)
}

// Parse converts a user-entered decimal string into int64 minor units for the
// given currency. It accepts an optional leading sign and up to `decimals`
// fractional digits. Extra precision is rejected rather than silently rounded.
func Parse(s, currency string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("money: empty amount")
	}
	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	s = strings.ReplaceAll(s, ",", "")
	d := decimals(currency)

	intPart, fracPart, hasFrac := strings.Cut(s, ".")
	if intPart == "" {
		intPart = "0"
	}
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: invalid amount %q", s)
	}
	var frac int64
	if hasFrac {
		if len(fracPart) > d {
			return 0, fmt.Errorf("money: too many decimal places for %s (max %d)", currency, d)
		}
		for len(fracPart) < d {
			fracPart += "0"
		}
		if fracPart != "" {
			frac, err = strconv.ParseInt(fracPart, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("money: invalid fraction %q", fracPart)
			}
		}
	}
	minor := whole*pow10(d) + frac
	if neg {
		minor = -minor
	}
	return minor, nil
}

func pow10(n int) int64 {
	r := int64(1)
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}
