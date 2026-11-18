package service

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dfryer1193/goblog/models"
)

// tagsSection is the site section holding the tag pages, at /tags/. It's
// also the section's title.
const tagsSection = "tags"

// tagsURL is the site path of the page listing every tag.
const tagsURL = "/" + tagsSection + "/"

// renderTags renders /tags/, listing every tag of the listed Posts with its
// Post count, and /tags/<tag>/ for each tag, listing its Posts like the
// index. Only the listed Posts count, so Drafts don't unless the build
// includes them (see Build, which collects b.tags).
func renderTags(b *siteBuild) error {
	data := pageData{Title: tagsSection, Label: tagsSection, WindowTitle: "all", Tags: b.tags}
	if err := b.out.page(indexFileFor(tagsURL), "tags", data); err != nil {
		return err
	}
	for _, tag := range b.tags {
		name := indexFileFor(tag.URL())
		data = pageData{Title: "#" + tag.Name, Label: "#" + tag.Name, WindowTitle: tag.Name, Tag: tag, Posts: tag.Posts}
		if err := b.out.page(name, "tag", data); err != nil {
			return fmt.Errorf("rendering tag %s: %w", tag.Name, err)
		}
	}
	return nil
}

// collectTags groups posts, which are in index order, by tag. Tags with the
// same slug are the same tag, named as on the first (newest) Post that has
// it. The tags come back sorted by slug.
func collectTags(posts []*models.Post) []*models.Tag {
	bySlug := map[string]*models.Tag{}
	for _, p := range posts {
		for _, name := range p.Tags {
			slug := models.TagSlug(name)
			tag, ok := bySlug[slug]
			if !ok {
				tag = &models.Tag{Name: name}
				bySlug[slug] = tag
			}
			if !slices.Contains(tag.Posts, p) {
				tag.Posts = append(tag.Posts, p)
			}
		}
	}
	tags := make([]*models.Tag, 0, len(bySlug))
	for _, tag := range bySlug {
		tags = append(tags, tag)
	}
	slices.SortFunc(tags, func(a, b *models.Tag) int {
		return strings.Compare(models.TagSlug(a.Name), models.TagSlug(b.Name))
	})
	return tags
}
