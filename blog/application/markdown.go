package application

// MarkdownProcessingResult contains the results of processing a markdown file
type MarkdownProcessingResult struct {
	Title    string
	Snippet  string
	HTMLPath string
}

// TODO: Use yuin/goldmark for markdown rendering

// MarkdownRenderer defines the interface for converting markdown to HTML.
type MarkdownRenderer interface {
	Render(markdown []byte) (*MarkdownProcessingResult, error)
}

// DummyMarkdownRenderer is a dummy implementation of MarkdownRenderer for testing and MVP.
// It simply wraps the markdown content in <pre> tags.
type DummyMarkdownRenderer struct{}

func NewDummyMarkdownRenderer() *DummyMarkdownRenderer {
	return &DummyMarkdownRenderer{}
}

func (r *DummyMarkdownRenderer) Render(markdown []byte) (*MarkdownProcessingResult, error) {
	return &MarkdownProcessingResult{
		Title:    "Untitled",
		Snippet:  "",
		HTMLPath: "",
	}, nil
}
