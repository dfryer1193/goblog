# goblog

A single-author blog server. You write Posts as Markdown in a separate git
repository, the Content Repository, and goblog publishes them with no manual
publishing step.

> **Status:** goblog is being rebuilt from scratch. Right now the server
> clones the Content Repository once at startup and serves its Posts as plain,
> unstyled HTML, and `goblog build` renders them the same way locally. The previous implementation is kept under
> the `legacy` tag for reference.

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
└── site.yaml    site settings: title, author, description, navigation, timezone
```

A Post's front matter needs a `title`. Its optional `date` (`YYYY-MM-DD`) is its
Publish Date. A Post without one, or with one still in the future, is a Draft
and isn't published. Only the default branch is published.

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
| `GOBLOG_SSH_KNOWN_HOSTS` | `known_hosts` lines for the SSH host, e.g. GitHub's published host keys             | `$SSH_KNOWN_HOSTS` or `~/.ssh/known_hosts` |
| `GOBLOG_WORK_DIR`        | Ephemeral working directory for the clone and the rendered site, e.g. an `emptyDir` | `goblog` under the temp directory          |
| `PORT`                   | The port to listen on                                                               | `8080`                                     |

The container image has no `known_hosts` file, so set `GOBLOG_SSH_KNOWN_HOSTS`
when you use a deploy key.

At startup goblog makes a shallow, single-branch clone of the Content
Repository into `GOBLOG_WORK_DIR` (with go-git: no `git` binary is needed),
renders the site into a fresh directory there and switches serving over to it.
Until that first render, `/readyz` reports not ready. Unknown paths get a 404.

goblog logs to stdout only. mjolnir's JSON log file (`logs/mjolnir.jsonl` by
default) is turned off, because the image's working directory may not be
writable.

The server listens on `$PORT` and answers these probes:

| Path        | Probe     |
| ----------- | --------- |
| `/livez`    | liveness  |
| `/readyz`   | readiness |
| `/startupz` | startup   |

To render the site once from a local checkout of the Content Repository, for
previewing, run:

```sh
goblog build -content path/to/content-repository -out public [-drafts]
```

The output directory must not exist yet, or be empty. `-drafts` also renders
Drafts, marked as such. Posts that can't be rendered, such as one with no
`title`, are reported and skipped. The rest of the site is still written, but
the command then exits with status 1.

The webhook and the poll that keep the site up to date are still to be built.

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
