# goblog

A single-author blog server. You write Posts as Markdown in a separate git
repository, the Content Repository, and goblog publishes them with no manual
publishing step.

> **Status:** goblog is being rebuilt from scratch. The server clones the
> Content Repository, keeps it in sync through the webhook and polling, and
> serves the whole site (Posts, Pages, tags, the Atom feed and the sitemap) in
> its tmux-styled design, pre-compressed. The only JavaScript is a small, optional
> script that remembers which tree folders the reader folded.
> `goblog build` renders the same site locally. Deployment docs are still to
> come. The previous implementation is kept under the `legacy` tag for
> reference.

## How it works

goblog keeps a shallow clone of the Content Repository. Whenever that
repository changes, it renders the **whole site** into static, pre-compressed
files and switches over to the new output all at once. A change is signalled
by a signed GitHub webhook or found by a poll every minute. A scheduled Post's
Publish Date arriving also triggers a render. Between renders goblog only
serves files: no JavaScript, and no database. A failed render leaves the
previous site in place.

See [ADR-0001](docs/adr/0001-render-on-change-static-output.md) for why goblog
renders on change instead of running as a static site generator in CI or as a
database-backed server.

The [glossary](GLOSSARY.md) defines the terms used here and in the code:
Content Repository, Post, Page, Publish Date, Draft and Published Post.

## The Content Repository

```text
<content-repository>/
├── posts/       one Post per .md file, e.g. posts/picluster.md → /posts/picluster/
├── pages/       one Page per .md file, e.g. pages/about.md → /about/
├── images/      images referenced from Posts by relative path (subfolders allowed)
├── templates/   ignored by goblog, e.g. a new-post template
└── site.yaml    site settings: title, author, description, url, navigation, timezone
```

A Post's front matter needs a `title`. Its optional `date` (`YYYY-MM-DD`) is its
Publish Date. A Post without one, or with one still in the future, is a Draft
and isn't published. Only the default branch is published.

Relative links and images are rewritten to the site's URLs: `./picluster.md`
becomes `/posts/picluster/`, `../pages/about.md` becomes `/about/` and
`../images/rack.png` becomes `/images/rack.png`. A relative link that escapes
the Content Repository, or points at a missing file or at one the site never
publishes (such as `../README.md`), is an error, and the Post or Page is
skipped. A link to a Post that
isn't published (a Draft, or a Post or Page skipped for an error) is a
warning instead: it's shown as plain text until its target publishes.

`site.yaml` must set the site's `title` and its base `url`, e.g.
`url: https://blog.example.com`. A `site.yaml` without them is invalid, so the
build fails and a running goblog keeps serving the previous site. Everything
else in it is optional:

```yaml
title: Werewolf Notes          # required
url: https://blog.example.com  # required: absolute http(s), no query or fragment
author: D. Author
description: Homelab things
timezone: America/New_York     # an IANA name; decides when Publish Dates arrive (default UTC)
host: example.com              # the status bar's and info line's label (default: url's first label, e.g. blog)
nav:                           # the status bar's windows, in order
  - {name: posts, link: /}
  - {name: tags, link: /tags/}
  - {name: about, link: /about/}
```

The site publishes an Atom feed of the Published Posts at `/feed.xml` and a
sitemap at `/sitemap.xml`, both with absolute URLs under `url`. A Content
Repository with no `site.yaml` at all still builds, for previewing, with
default settings, but that's reported as a problem, and the site then has no
feed or sitemap, and nothing links to them.

## Running goblog

goblog ships as a single binary, `goblog`, and a container image of the same
name built by CI. It ships no deployment manifests: bring your own. Everything
specific to one blog lives in the Content Repository's `site.yaml` and in the
deployment's environment, so you can run your own blog from a fork without
code changes.

The server is configured from the environment:

| Variable                 | Meaning                                                                             | Default                                    |
| ------------------------ | ----------------------------------------------------------------------------------- | ------------------------------------------ |
| `GOBLOG_CONTENT_URL`     | The Content Repository's URL: SSH (`git@github.com:you/blog-content.git`) or HTTPS  | required                                   |
| `GOBLOG_CONTENT_BRANCH`  | The branch to publish                                                               | the remote's default branch                |
| `GOBLOG_DEPLOY_KEY`      | A PEM-encoded SSH private key, e.g. a read-only deploy key, for an SSH URL          | none                                       |
| `GOBLOG_SSH_KNOWN_HOSTS` | `known_hosts` lines for the SSH host, e.g. GitHub's published host keys. **Required with `GOBLOG_DEPLOY_KEY`** in the container image | `$SSH_KNOWN_HOSTS` or `~/.ssh/known_hosts` |
| `GOBLOG_WORK_DIR`        | Ephemeral working directory for the clone and the rendered site, e.g. an `emptyDir` | `goblog` under the temp directory          |
| `PORT`                   | The port to listen on                                                               | `8080`                                     |
| `GOBLOG_METRICS_PORT`    | The port serving the [Site Statistics](#site-statistics) metrics at `/metrics`      | `9090`                                     |
| `GOBLOG_STATS_SALT_FILE` | The path of the daily salt for Visitor IDs: see [the salt file](#the-salt-file)     | none: Page Views have no Visitor ID        |
| `GOBLOG_WEBHOOK_SECRET`  | The GitHub webhook's secret, for checking its signature                             | none: the webhook endpoint is disabled     |
| `GOBLOG_POLL_INTERVAL`   | How often to poll, as a Go duration such as `1m` or `30s`                           | `1m`                                       |

`GOBLOG_SSH_KNOWN_HOSTS` is required whenever you use a deploy key: the
container image has no `known_hosts` file, so without it goblog can't check
the SSH host's key and every clone and fetch fails. (Outside the image, goblog
falls back to `$SSH_KNOWN_HOSTS` or `~/.ssh/known_hosts`.) Point
`GOBLOG_WORK_DIR` at ephemeral storage, such as an `emptyDir`: goblog keeps
the clone and the rendered site there and rebuilds both at startup.

### Setting up secrets

[`scripts/setup-secrets.sh`](scripts/setup-secrets.sh) is an interactive
wizard that sets up every secret goblog needs, so you don't have to work the
steps out again each time. Run it on a machine with a browser, `gh` (signed in)
and the 1Password CLI `op` (signed in):

```sh
GH_REPO=<owner>/goblog ./scripts/setup-secrets.sh
```

It walks through:

1. **CI image pushes:** a Harbor robot account with push access, stored as the
   `HARBOR_USERNAME` and `HARBOR_PASSWORD` repository secrets. The wizard
   checks the credentials can push before storing them.
2. **Cloning:** a new read-only deploy key on the Content Repository, plus
   `known_hosts` built from GitHub's published host keys.
3. **The webhook:** a `push` webhook on the Content Repository with a
   generated secret.
4. **The cluster:** 1Password items holding the deploy key, `known_hosts` and
   the webhook secret, which your deployment maps to `GOBLOG_DEPLOY_KEY`,
   `GOBLOG_SSH_KNOWN_HOSTS` and `GOBLOG_WEBHOOK_SECRET`.

Secrets never appear on a command line or in a file outside a temporary
directory that the wizard deletes. Re-running it skips what's already set up,
or asks before replacing it. The defaults (the registry, the Content Repository
and the webhook URL) are variables at the top of the stages section; override
them with environment variables of the same names for your own blog.

At startup goblog makes a shallow, single-branch clone of the Content
Repository into `GOBLOG_WORK_DIR` (with go-git: no `git` binary is needed),
renders the site into a fresh directory there and switches serving over to it.
Until that first render succeeds, `/readyz` reports not ready, and every
other request (except `/webhook`) gets status 503 with a built-in "starting"
page, `Cache-Control: no-store` and `Retry-After: 10`. Unknown paths get the
site's own 404 page, with status 404, or a built-in 404 page in the same style
if the site's can't be read.

### Keeping the site in sync

Two triggers keep the site up to date. They never render at the same time:
one waits for the other.

- **The webhook.** Add a GitHub webhook to the Content Repository with the
  payload URL `https://<your-site>/webhook`, content type
  `application/json`, the secret from `GOBLOG_WEBHOOK_SECRET` and just the
  `push` event. goblog checks each delivery's `X-Hub-Signature-256` and
  answers 401 to one without a valid signature. A `push` gets 202 straight
  away, and goblog fetches and renders in the background. Pushes that arrive
  during a sync cause one more sync after it. A `ping` gets 204. Without
  `GOBLOG_WEBHOOK_SECRET`, `/webhook` answers like an unknown path (404, to
  every method) and goblog logs at startup
  that only polling will sync.
- **The poll.** Every `GOBLOG_POLL_INTERVAL`, goblog asks the remote for the
  branch's latest commit (like `git ls-remote`, with no fetch) and fetches and
  renders only if it has changed. That catches pushes whose webhook was lost,
  or that reached another replica. If the date in the site's timezone
  (`timezone` in `site.yaml`, UTC by default) has changed since the last
  render, the poll renders the same commit again without fetching, so Posts
  scheduled for the new day publish on time, even if the remote is
  unreachable. Until the first render succeeds, each poll retries the full
  sync.

A sync or render that fails is logged, and the previous site keeps serving. A
commit that can't be rendered isn't retried until the next commit or the
next day.

goblog logs to stdout only. mjolnir's JSON log file (`logs/mjolnir.jsonl` by
default) is turned off, because the image's working directory may not be
writable.

The server listens on `$PORT` and answers these probes:

| Path        | Probe     |
| ----------- | --------- |
| `/livez`    | liveness  |
| `/readyz`   | readiness |
| `/startupz` | startup   |

### Deploying

goblog ships no manifests, so here is everything a deployment needs:

- **Image:** CI builds `goblog` for `linux/amd64` and `linux/arm64` with ko, on
  `gcr.io/distroless/static-debian12:nonroot`, and pushes it to the
  registry on merges to `main` and on version tags (`v0.1.0`, …). Pin a
  version tag. The image has no shell, no `git` and no `known_hosts`, and
  runs as the distroless `nonroot` user (UID 65532).
- **Environment:** the variables in the table above. A typical deployment
  sets `GOBLOG_CONTENT_URL` (an SSH URL), `GOBLOG_DEPLOY_KEY`,
  `GOBLOG_SSH_KNOWN_HOSTS`, `GOBLOG_WEBHOOK_SECRET` and `GOBLOG_WORK_DIR`,
  taking the secrets from wherever your cluster keeps them.
  [`scripts/setup-secrets.sh`](scripts/setup-secrets.sh) creates them.
- **Volume:** one writable volume mounted at `GOBLOG_WORK_DIR`, such as an
  `emptyDir`. goblog keeps the clone and the rendered site there and rebuilds
  both at startup, so nothing needs to persist and a PVC buys nothing.
  goblog writes only under `GOBLOG_WORK_DIR` (the clone, the rendered site and
  the `known_hosts` file) and logs only to stdout, so the root filesystem can
  be read-only.
- **Port and probes:** HTTP on `PORT` (default `8080`). Point liveness at
  `/livez`, readiness at `/readyz` (not ready until the first render is
  served) and, if you use one, the startup probe at `/startupz`.
- **Metrics:** Prometheus metrics at `/metrics` on `GOBLOG_METRICS_PORT`
  (default `9090`), apart from the site. Keep that port off the public
  route, and point your scraper at it.
- **Salt:** a Secret holding the daily salt, mounted read-only and named by
  `GOBLOG_STATS_SALT_FILE`, replaced every day at 00:00 UTC by a scheduled
  job. See [the salt file](#the-salt-file).
- **Webhook:** the Content Repository's push webhook goes to `/webhook` on
  the site's public hostname.
- **Replicas:** each replica clones, polls and renders on its own, with no
  shared state. A webhook reaches only one of them, and the others catch up
  within `GOBLOG_POLL_INTERVAL`. Spread replicas across nodes if you run
  more than one.
- **Resources:** request **32Mi** of memory and set a limit of **1Gi**, with
  a small CPU request (such as 25m). goblog uses about 30 MB at steady state.
  While it renders, it holds the whole site in memory, so the peak grows with
  the site:

  | Posts | Peak memory while rendering |
  | ----- | --------------------------- |
  | 30    | about 35 MB                 |
  | 100   | about 39 MB                 |
  | 200   | about 76 MB                 |

  These were measured with synthetic Posts of 40 paragraphs each. The limit
  is there to absorb that growth: it isn't reserved, and only the request is.
  Rendering 200 Posts takes under a second.

To render the site once from a local checkout of the Content Repository, for
previewing, run:

```sh
goblog build -content path/to/content-repository -out public [-drafts]
```

The output directory must not exist yet, or be empty. `-drafts` also renders
Drafts, marked as such. Posts that can't be rendered, such as one with no
`title`, are reported and skipped. The rest of the site is still written, but
the command then exits with status 1. Warnings, about things the build worked
around, are reported too, but on their own the command still succeeds.

## Site Statistics

goblog counts how its site is read without cookies, JavaScript or a
database (see [ADR-0002](docs/adr/0002-site-statistics-without-a-database.md)
and the glossary's Site Statistics terms). It classifies each request for
the site's files and emits what it learns two ways: a JSON line on stdout
for each Page View, and Prometheus metrics. The probes and the webhook
aren't counted. Emitting never changes or delays a response: if stdout
can't keep up, lines are dropped.

A **Page View** is a `GET` (not `HEAD`) that isn't a browser prefetch (no
`Sec-Purpose` or `Purpose` header saying `prefetch`), for an HTML page (a
Post, a Page, the index or a tag page), answered with 200 or 304. Images,
assets, the feed, the sitemap, redirects and 404s are never Page Views.

Each request has a **kind of page**, from its path: `post`, `page`, `index`,
`tag` (the tags page or a tag's page), `image`, `asset` (the stylesheet,
fonts and licences), `feed`, `sitemap`, `missing` (anything answered with
404) or `other` (such as a redirect).

### The JSON lines

Each Page View writes one JSON object on its own line to stdout, among
goblog's ordinary logs. These field names are a contract: later versions
only add fields.

| Field    | Meaning                                                           |
| -------- | ----------------------------------------------------------------- |
| `stream` | Always `goblog_stats`. Select Site Statistics lines by this field |
| `time`   | When the request was answered: RFC 3339, in UTC                   |
| `class`  | The class of request: `page_view`                                 |
| `path`   | The request's path, without its query string                      |
| `status` | The response's status code                                        |
| `kind`   | The kind of page                                                  |
| `visitor_id` | The Visitor ID: 16 hex digits, the same for one Visitor all UTC day. Omitted without a salt |

For example:

```json
{"stream":"goblog_stats","time":"2026-03-10T15:00:00Z","class":"page_view","path":"/posts/hello/","status":200,"kind":"post","visitor_id":"06c24fb019db91f4"}
```

### Visitor IDs

A Visitor ID tells Visitors apart for one UTC day, without cookies and
across every replica. It's the first 8 bytes, in hex (16 digits), of
HMAC-SHA-256 keyed with the daily salt, over the client IP, a NUL byte and
the `User-Agent`. Once the salt is replaced and destroyed, nobody, the
author included, can recompute an old day's Visitor IDs or tie them to the
new day's.

The client IP is the rightmost `X-Forwarded-For` entry, which the gateway
in front of goblog appends, so goblog must sit behind exactly one proxy
that appends it. Without the header, or if its rightmost entry isn't an IP
address, goblog uses the TCP peer. The client IP is never written to a
line or a log.

#### The salt file

The salt is a file at the path in `GOBLOG_STATS_SALT_FILE`, such as a
mounted Kubernetes Secret. goblog only reads it: it never makes up, derives
or stores a salt of its own. Whatever writes it must follow this contract:

- **Contents:** the raw bytes of the salt, at least **32 bytes** of fresh
  randomness, such as `head -c 32 /dev/urandom`. The whole file is the
  salt, byte for byte: nothing is trimmed or decoded, so a trailing newline
  is part of it.
- **Rotation:** replaced with a new random salt every day at **00:00 UTC**,
  with the old one destroyed, not kept anywhere. Every replica reads the
  same file, so they agree on Visitor IDs.
- **Reloading:** goblog rereads the file at most every 5 seconds, and uses
  a new salt as soon as it sees one, with no restart. (A mounted Secret
  takes the kubelet up to a minute or so to update, so replicas may
  disagree briefly around 00:00 UTC.)

If the variable isn't set, or the file is missing, empty, shorter than 32
bytes or unreadable, Page View lines leave out `visitor_id` and everything
else is emitted as usual. goblog logs a warning when the salt becomes
unusable (not on every request), `goblog_stats_salt_loaded` reads 0, and
Visitor IDs come back once the file is usable again.

### The metrics

Served at `/metrics` on `GOBLOG_METRICS_PORT` (default `9090`), never on the
site's port:

| Metric                        | Type    | Labels | Counts                                                 |
| ----------------------------- | ------- | ------ | ------------------------------------------------------ |
| `goblog_page_views_total`     | counter | `kind` | Page Views                                             |
| `goblog_other_requests_total` | counter | `kind` | Every other request for the site's files, such as images, assets, the feed, redirects and 404s |
| `goblog_stats_salt_loaded`    | gauge   | none   | 1 while a usable salt is loaded, 0 while Page Views have no Visitor ID |

### Settings

Site Statistics read their settings, the `StatsConfig`, on every request, so
a change applies at once. For now goblog always uses the defaults:

| Setting   | Meaning                                                          | Default |
| --------- | ---------------------------------------------------------------- | ------- |
| `enabled` | Whether Site Statistics are on. Off, no lines are written and no metrics counted; the site is served as usual | `true` |

`goblog build` doesn't count anything.

## Development

The repository is scaffolded with
[Ivaldi](https://github.com/worldtree-industries/ivaldi). You need Go 1.27 and
[golangci-lint v2](https://golangci-lint.run/welcome/install/).

| Command            | What it does                                  |
| ------------------ | --------------------------------------------- |
| `make lint`        | Run golangci-lint                             |
| `make test`        | Run the tests                                 |
| `make build`       | Build everything, and each binary into `bin/` |
| `make run-goblog`  | Run the server                                |
| `make install`     | `go install` every binary                     |
| `make setup-hooks` | Turn on the pre-commit hook in a fresh clone  |

Run `make setup-hooks` once after cloning. The pre-commit hook runs
`make lint test`.

The code follows Ivaldi's layers, and lint enforces which may import which:

| Layer      | Holds                                                  | May import          |
| ---------- | ------------------------------------------------------ | ------------------- |
| `api`      | HTTP handlers: static file serving, the webhook        | `service`, `models` |
| `service`  | the site build, and syncing and switching over output  | `data`, `models`    |
| `data`     | Content Repository access: clone and fetch             | `models`            |
| `models`   | Post, Page and site settings types, the rendered site  | nothing             |

## Licence

See [LICENSE](LICENSE).
