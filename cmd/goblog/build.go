package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dfryer1193/goblog/service"
)

// errProblems means the site was written, but some Posts were skipped.
var errProblems = errors.New("some files were skipped")

// runBuild is `goblog build`: render the site once from a Content Repository
// checkout into an output directory. It writes the output even when some
// Posts are skipped, but then reports them and returns errProblems.
func runBuild(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("goblog build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: goblog build [-content dir] -out dir [-drafts]")
		flags.PrintDefaults()
	}
	content := flags.String("content", ".", "the Content Repository checkout to render")
	out := flags.String("out", "", "the directory to write the site into; it must not exist or be empty")
	drafts := flags.Bool("drafts", false, "include Drafts, for previewing")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err //nolint:wrapcheck // flag has already printed the problem and usage
	}
	if *out == "" || flags.NArg() > 0 {
		flags.Usage()
		return errors.New("-out is required, and there are no other arguments")
	}

	site, err := service.Build(os.DirFS(*content), time.Now(), service.BuildOptions{IncludeDrafts: *drafts})
	if err != nil {
		return fmt.Errorf("building %s: %w", *content, err)
	}
	if err = service.WriteSite(*out, site); err != nil {
		return fmt.Errorf("writing the site: %w", err)
	}
	for _, p := range site.Problems {
		_, _ = fmt.Fprintf(stderr, "skipped %s\n", p)
	}
	_, _ = fmt.Fprintf(stderr, "wrote %d files to %s\n", len(site.Files), *out)
	if len(site.Problems) > 0 {
		return fmt.Errorf("%d skipped: %w", len(site.Problems), errProblems)
	}
	return nil
}
