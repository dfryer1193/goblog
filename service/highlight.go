package service

import (
	"fmt"
	"io"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

// Palette colours from the author's alacritty.toml, as the code highlighting
// uses them.
const (
	paletteFG          = "#dddddd"
	paletteDim         = "#666666"
	paletteRed         = "#e84f4f"
	paletteGreen       = "#b7ce42"
	paletteYellow      = "#fea63c"
	paletteBlue        = "#66aabb"
	paletteMagenta     = "#b7416e"
	paletteCyan        = "#6d878d"
	paletteBrightGreen = "#bde077"
	codeBackground     = "#1b1b1b"
)

// codeStyle maps chroma's token types onto the palette. Code blocks are
// rendered with CSS classes, so this only decides the stylesheet's rules.
func codeStyle() (*chroma.Style, error) {
	// Token types not listed inherit their parent's style, e.g. CommentSingle
	// from Comment.
	//exhaustive:ignore
	style, err := chroma.NewStyle("goblog", chroma.StyleEntries{
		chroma.Background:        "bg:" + codeBackground + " " + paletteFG,
		chroma.Error:             paletteRed,
		chroma.Comment:           "italic " + paletteDim,
		chroma.CommentPreproc:    "noitalic " + paletteCyan,
		chroma.Keyword:           paletteMagenta,
		chroma.KeywordType:       paletteBlue,
		chroma.Name:              paletteFG,
		chroma.NameTag:           paletteBlue,
		chroma.NameAttribute:     paletteBlue,
		chroma.NameBuiltin:       paletteGreen,
		chroma.NameFunction:      paletteGreen,
		chroma.NameClass:         paletteGreen,
		chroma.NameVariable:      paletteYellow,
		chroma.NameConstant:      paletteYellow,
		chroma.Literal:           paletteBrightGreen,
		chroma.LiteralString:     paletteBrightGreen,
		chroma.LiteralNumber:     paletteYellow,
		chroma.Operator:          paletteCyan,
		chroma.Punctuation:       paletteCyan,
		chroma.GenericDeleted:    paletteRed,
		chroma.GenericInserted:   paletteGreen,
		chroma.GenericHeading:    "bold " + paletteYellow,
		chroma.GenericSubheading: paletteYellow,
		chroma.GenericEmph:       "italic",
		chroma.GenericStrong:     "bold",
		chroma.GenericPrompt:     paletteDim,
	})
	if err != nil {
		return nil, fmt.Errorf("building the code style: %w", err)
	}
	return style, nil
}

// codeFormatOptions are how code blocks are rendered: with CSS classes, not
// inline styles.
func codeFormatOptions() []chromahtml.Option {
	return []chromahtml.Option{chromahtml.WithClasses(true)}
}

// highlightCode is the goldmark extension that highlights fenced code blocks
// at render time.
func highlightCode() goldmark.Extender {
	return highlighting.NewHighlighting(
		highlighting.WithFormatOptions(codeFormatOptions()...),
	)
}

// writeCodeCSS writes the stylesheet rules for highlighted code.
func writeCodeCSS(w io.Writer) error {
	style, err := codeStyle()
	if err != nil {
		return err
	}
	if err = chromahtml.New(codeFormatOptions()...).WriteCSS(w, style); err != nil {
		return fmt.Errorf("writing the code style: %w", err)
	}
	return nil
}
