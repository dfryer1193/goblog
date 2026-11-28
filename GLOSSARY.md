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

### Site Statistics

**Site Statistics**:
What the author learns about how the site is read: who comes, from where, and what they read. Only the author sees them.
_Avoid_: analytics, tracking, metrics

**Visitor**:
A person reading the site, counted once per UTC day, whatever the site's timezone. The same person on two different days is two Visitors, because nothing ties a reader's visits together across days.
_Avoid_: user, unique, session

**Page View**:
One request by a Visitor for a page of the site.
_Avoid_: hit, view

**Visitor 404**:
A Visitor's request for a path the site doesn't have, answered with the 404 page. It's never a Page View, and a Bot's request for a missing path is never one.
_Avoid_: broken link, 404 hit

**Source**:
Where a Visitor came from: a referring site (grouped, so every Hacker News URL is one Source), a search engine, or a Campaign. A Visitor who arrives with neither a referrer nor a Campaign has no Source; they came direct.
_Avoid_: referrer, referer, origin

**Campaign**:
A Source the author names by tagging a link, such as `?ref=newsletter` or `?utm_source=mastodon`. A Campaign takes precedence over the referring site.
_Avoid_: UTM

**Feed Subscriber**:
Someone following the site's Atom feed in a feed reader, as counted by what feed readers report. Feed Subscribers aren't Visitors, and the feed reader requesting the feed isn't a Bot.
_Avoid_: follower, RSS subscriber

**Bot**:
A program, not a person, requesting the site's pages, such as a search crawler. Bots are never Visitors and their requests are never Page Views, but they're counted on their own.
_Avoid_: crawler, spider
