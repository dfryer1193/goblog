# goblog

A single-author blog that publishes Posts written as Markdown in a separate git repository.

## Language

**Content Repository**:
The git repository holding the blog's Posts and their images. It is a separate repository from goblog itself.
_Avoid_: post repo, source repo, blog repo

**Post**:
One Markdown file in the Content Repository's `posts/` folder, plus the metadata in its front matter.
_Avoid_: article, entry

**Page**:
A standalone Markdown file in the Content Repository's `pages/` folder, such as About. Pages are linked from the navigation and never appear in the post index or the feed. They have no Publish Date.
_Avoid_: static page

**Publish Date**:
The date the author sets for a Post to go public. Posts are ordered by it and the site displays it. It isn't necessarily the date the Post was created or first committed.
_Avoid_: create date, commit date

**Draft**:
A Post with no Publish Date, or with a Publish Date that hasn't arrived yet. Being a Draft is a consequence of the Publish Date, not something the author sets separately.
_Avoid_: unpublished post

**Published Post**:
A Post on the Content Repository's default branch whose Publish Date has arrived. Only Published Posts appear on the site.
