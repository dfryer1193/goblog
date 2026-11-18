package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/dfryer1193/goblog/models"
)

// settingsFile is the Content Repository file holding the site settings.
const settingsFile = "site.yaml"

// defaultSettings are the site settings when the Content Repository has no
// site.yaml.
func defaultSettings() models.SiteSettings {
	return models.SiteSettings{
		Title: "goblog",
		Nav:   []models.NavLink{{Name: "posts", Link: "/"}, {Name: "tags", Link: "/tags/"}},
	}
}

// settingsYAML is site.yaml as the author wrote it.
type settingsYAML struct {
	Title       string `yaml:"title"`
	Author      string `yaml:"author"`
	Description string `yaml:"description"`
	URL         string `yaml:"url"`
	Nav         []struct {
		Name string `yaml:"name"`
		Link string `yaml:"link"`
	} `yaml:"nav"`
	Timezone string `yaml:"timezone"`
}

// readSettings reads site.yaml from content. A missing site.yaml gives the
// default settings and a Problem, so that the site still builds. An invalid
// one is an error: guessing at the settings could publish Posts early.
func readSettings(content fs.FS) (models.SiteSettings, *models.Problem, error) {
	src, err := fs.ReadFile(content, settingsFile)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultSettings(), &models.Problem{
			Path: settingsFile,
			Err:  errors.New("missing, so the site uses the default settings"),
		}, nil
	}
	if err != nil {
		return models.SiteSettings{}, nil, fmt.Errorf("reading %s: %w", settingsFile, err)
	}
	settings, err := parseSettings(src)
	if err != nil {
		return models.SiteSettings{}, nil, fmt.Errorf("%s: %w", settingsFile, err)
	}
	return settings, nil, nil
}

// parseSettings parses and validates site.yaml's contents.
func parseSettings(src []byte) (models.SiteSettings, error) {
	var raw settingsYAML
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		return models.SiteSettings{}, fmt.Errorf("invalid YAML: %w", err)
	}

	settings := models.SiteSettings{
		Title:       strings.TrimSpace(raw.Title),
		Author:      strings.TrimSpace(raw.Author),
		Description: strings.TrimSpace(raw.Description),
	}
	if settings.Title == "" {
		return models.SiteSettings{}, errors.New("title is required")
	}
	var err error
	if settings.URL, err = parseBaseURL(raw.URL); err != nil {
		return models.SiteSettings{}, err
	}
	for i, n := range raw.Nav {
		link := models.NavLink{Name: strings.TrimSpace(n.Name), Link: strings.TrimSpace(n.Link)}
		if link.Name == "" || link.Link == "" {
			return models.SiteSettings{}, fmt.Errorf("nav entry %d needs both a name and a link", i+1)
		}
		settings.Nav = append(settings.Nav, link)
	}
	if tz := strings.TrimSpace(raw.Timezone); tz != "" {
		if settings.Timezone, err = time.LoadLocation(tz); err != nil {
			return models.SiteSettings{}, fmt.Errorf("timezone %q is not an IANA timezone name", tz)
		}
	}
	return settings, nil
}

// parseBaseURL checks that s, if set, is an absolute http or https URL, and
// drops any trailing slash.
func parseBaseURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("url %q is not an absolute http or https URL", s)
	}
	return strings.TrimRight(s, "/"), nil
}
