package service

import (
	"fmt"
	"io"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

// codeStyle maps chroma's token types onto the palette. Code blocks are
// rendered with CSS classes, so this only decides the stylesheet's rules.
func codeStyle() (*chroma.Style, error) {
	// Token types not listed inherit their parent's style, e.g. CommentSingle
	// from Comment.
	//exhaustive:ignore
	style, err := chroma.NewStyle("goblog", chroma.StyleEntries{
		chroma.Background:        "bg:" + colourCodeBG + " " + colourFG,
		chroma.Error:             colourRed,
		chroma.Comment:           "italic " + colourDim,
		chroma.CommentPreproc:    "noitalic " + colourCyan,
		chroma.Keyword:           colourMagenta,
		chroma.KeywordType:       colourBlue,
		chroma.Name:              colourFG,
		chroma.NameTag:           colourBlue,
		chroma.NameAttribute:     colourBlue,
		chroma.NameBuiltin:       colourGreen,
		chroma.NameFunction:      colourGreen,
		chroma.NameClass:         colourGreen,
		chroma.NameVariable:      colourYellow,
		chroma.NameConstant:      colourYellow,
		chroma.Literal:           colourBrightGreen,
		chroma.LiteralString:     colourBrightGreen,
		chroma.LiteralNumber:     colourYellow,
		chroma.Operator:          colourCyan,
		chroma.Punctuation:       colourCyan,
		chroma.GenericDeleted:    colourRed,
		chroma.GenericInserted:   colourGreen,
		chroma.GenericHeading:    "bold " + colourYellow,
		chroma.GenericSubheading: colourYellow,
		chroma.GenericEmph:       "italic",
		chroma.GenericStrong:     "bold",
		chroma.GenericPrompt:     colourDim,
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
