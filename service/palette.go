package service

import "strings"

// The palette: the author's alacritty colours, and their tmux status bar's.
// It's the one source of the site's colours: the stylesheet and api's
// built-in pages get them from PaletteCSS, and the code highlighting from
// these constants.
const (
	colourBG           = "#222222"
	colourFG           = "#dddddd"
	colourDim          = "#666666"
	colourRed          = "#e84f4f"
	colourGreen        = "#b7ce42"
	colourYellow       = "#fea63c"
	colourBlue         = "#66aabb"
	colourMagenta      = "#b7416e"
	colourCyan         = "#6d878d"
	colourBrightGreen  = "#bde077"
	colourBrightYellow = "#ffe863"
	colourCodeBG       = "#1b1b1b"
	colourRule         = "#333333"
	colourPaneBorder   = "#444444"
	// tmux's status-bg, colour008 and colour245.
	colourBarBG     = "#3a3a3a"
	colourColour008 = "#666666"
	colourColour245 = "#8a8a8a"
)

// paletteVars are the CSS custom properties PaletteCSS defines, in order.
var paletteVars = []struct{ name, value string }{ //nolint:gochecknoglobals // fixed table
	{"bg", colourBG},
	{"fg", colourFG},
	{"dim", colourDim},
	{"red", colourRed},
	{"green", colourGreen},
	{"yellow", colourYellow},
	{"blue", colourBlue},
	{"magenta", colourMagenta},
	{"cyan", colourCyan},
	{"bright-green", colourBrightGreen},
	{"bright-yellow", colourBrightYellow},
	{"code-bg", colourCodeBG},
	{"rule", colourRule},
	{"pane-border", colourPaneBorder},
	{"bar-bg", colourBarBG},
	{"colour008", colourColour008},
	{"colour245", colourColour245},
}

// PaletteCSS is a CSS :root rule defining the palette as custom properties,
// such as --red: #e84f4f, for the stylesheet and any page that must look
// like the site without loading it.
func PaletteCSS() string {
	var b strings.Builder
	b.WriteString("/* The palette: the author's alacritty colours and tmux status bar. */\n:root {\n")
	for _, v := range paletteVars {
		b.WriteString("  --" + v.name + ": " + v.value + ";\n")
	}
	b.WriteString("}\n")
	return b.String()
}
