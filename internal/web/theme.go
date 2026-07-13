package web

// DefaultTheme is the accent applied to logged-out pages and to users who have
// not chosen one. GoSplit ships a gentle burgundy by default.
const DefaultTheme = "burgundy"

// Theme is a named accent with a designed light and dark variant, so no user
// choice can break contrast. LightSoft/DarkSoft are tinted backgrounds used for
// pills and soft buttons.
type Theme struct {
	Slug       string
	Label      string
	LightAccent, LightInk, LightSoft string
	DarkAccent, DarkInk, DarkSoft    string
}

// themes is the ordered, closed set of selectable accents. The order here is the
// order shown in the profile swatch row.
var themes = []Theme{
	{"burgundy", "Burgundy", "#8d3f4c", "#ffffff", "#f3e6e9", "#d98d98", "#2e1016", "#382028"},
	{"teal", "Teal", "#0f766e", "#ffffff", "#e4f0ee", "#2dd4bf", "#04211d", "#10332f"},
	{"indigo", "Indigo", "#4f46e5", "#ffffff", "#e9e8fb", "#a5b4fc", "#1e1b4b", "#252a4a"},
	{"forest", "Forest", "#2d6a4f", "#ffffff", "#e3efe9", "#74c69d", "#08251a", "#1a3329"},
	{"amber", "Amber", "#b45309", "#ffffff", "#f7ecdf", "#fbbf24", "#451a03", "#3a2b10"},
	{"slate", "Slate", "#475569", "#ffffff", "#e8ecf1", "#94a3b8", "#0f172a", "#26303d"},
}

// Themes returns the selectable accent themes in display order.
func Themes() []Theme { return themes }

// ValidTheme reports whether slug is a known theme. It is the central allowlist
// gate: handlers must reject any value this rejects.
func ValidTheme(slug string) bool {
	for _, t := range themes {
		if t.Slug == slug {
			return true
		}
	}
	return false
}

// themeHex returns the accent hex for a theme slug in the requested mode,
// falling back to the default theme for unknown slugs. Used for the PWA
// theme-color meta tags.
func themeHex(slug string, dark bool) string {
	for _, t := range themes {
		if t.Slug == slug {
			if dark {
				return t.DarkAccent
			}
			return t.LightAccent
		}
	}
	if slug != DefaultTheme {
		return themeHex(DefaultTheme, dark)
	}
	return "#8d3f4c"
}
