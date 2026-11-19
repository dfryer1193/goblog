# Research: using `worldtree-industries/ivaldi` for dfryer1193/goblog

Researched 2026-11-18. All claims come from primary sources at pinned commits:

- **Ivaldi** at [`5dc8210`][iv] (`main`, "Merge pull request #19"). Citations look like `iv:README.md#L23`, which links to [`blob/5dc8210…/README.md#L23`][iv].
- **worldtree-industries/workflows** (the shared reusable workflow Ivaldi's CI calls) at [`e27aff8`][wf] = tag `v1.0.0`, also `main` HEAD today. Citations look like `wf:<path>#L<n>`.
- The state of goblog itself comes from `gh api` / `git` run against this repo.

Both repos are **public** (`gh repo view … --json visibility`).

[iv]: https://github.com/worldtree-industries/ivaldi/tree/5dc821065b1f7a26daf3098a1e28463c450b4c5b
[wf]: https://github.com/worldtree-industries/workflows/tree/e27aff84ed75bc3995068bce81b197c7dd6eafff

To turn a citation into a URL, use `https://github.com/worldtree-industries/ivaldi/blob/5dc821065b1f7a26daf3098a1e28463c450b4c5b/<path>#L<n>` for `iv:`, and `https://github.com/worldtree-industries/workflows/blob/e27aff84ed75bc3995068bce81b197c7dd6eafff/<path>#L<n>` for `wf:`.

---

## 1. Installation and invocation

- **Install:** `go install github.com/worldtree-industries/ivaldi/cmd/ivaldi@latest`, with `$(go env GOPATH)/bin` on `PATH` (iv:README.md#L9-L13). Ivaldi's own `go.mod` says `go 1.27` (iv:go.mod#L3), so building it needs a Go 1.27 toolchain.
- **There are no flags, no arguments and no config file.** The README says so (iv:README.md#L5, #L17), and the code matches: `cli.Run` accepts exactly one argument, `init` or `add`, and prints usage otherwise (iv:internal/cli/cli.go#L51-L75). Every input is an interactive prompt on stdin, answered one line at a time in order (iv:docs/extending-ivaldi.md#L15).
- **It works on the current directory** (iv:cmd/ivaldi/main.go#L29-L40). It **never commits** anything (iv:internal/cli/cli.go#L119). It never creates GitHub repos or secrets (iv:README.md#L139).

### `ivaldi init`: runs against existing repos ("Adoption")

`init` works in an empty directory *or* in a repo that already has code (iv:README.md#L21, #L32-L40). It runs these steps in order (iv:README.md#L23-L30; iv:internal/cli/cli.go#L171-L242):

1. It refuses to run if `.forge.yaml` already exists.
2. **Module path.** If a `go.mod` exists, Ivaldi uses its module path and doesn't ask (iv:internal/cli/modulepath.go#L18-L27). Otherwise it prompts, with a default taken from `origin`: `git@github.com:o/r.git` becomes `github.com/o/r` (iv:internal/cli/modulepath.go#L32-L55).
3. **Existing binaries.** Every `cmd/<name>/` that holds `package main` is a binary. For each one, Ivaldi asks its Kind and that Kind's questions, and records it in the Manifest. It never rewrites that binary's `main.go`. A `cmd/<name>/Dockerfile` records `build: dockerfile`. A *root* `Dockerfile` only produces a warning saying to move it into `cmd/<name>/` (iv:README.md#L36-L37; iv:internal/cli/adopt.go#L77, #L104-L108, #L143-L144).
4. **New binaries.** Prompts for a comma-separated list. A blank answer means none (iv:README.md#L25, #L46-L50).
5. **Overwrite prompts.** Ivaldi asks about each existing file it would write, defaulting to *no*. `go.mod`, `README.md` and the Layers' `doc.go` are never overwritten. An existing `.gitignore` only gets `/bin/` and `/logs/` appended if they're missing (iv:README.md#L38; iv:internal/scaffold/scaffold.go#L161-L213).
6. **Go line.** If `go.mod` says less than `go 1.27`, Ivaldi asks to raise it, defaulting to *yes*, then runs `go mod edit -go=1.27`. The minimum comes from mjolnir (iv:internal/cli/modulepath.go#L74-L115).
7. Runs `git init`, unless the directory is already the top level of a git repo (iv:internal/cli/cli.go#L214-L218, #L267-L273).
8. Writes the files. The Manifest is written last (iv:internal/cli/cli.go#L223-L225).
9. Runs `git config core.hooksPath .githooks`, unless you kept your own hook (iv:internal/cli/cli.go#L233-L247).
10. Fetches mjolnir and runs `go mod tidy` if any http or job binary was added. A failure here is only a warning (iv:README.md#L50; iv:docs/extending-ivaldi.md#L9).
11. **Applies GitHub repository settings** through `gh api`, using `origin` (see §2.7). If that fails, it warns and prints the `gh` commands to run by hand (iv:internal/cli/github.go#L52-L81).

### `ivaldi add`

`add` adds binaries to a Project, and refuses to run if there's no `.forge.yaml`. It validates names (lowercase, `[a-z][a-z0-9-]*`, 63 characters at most, unique), asks each binary's Kind and that Kind's questions, writes `cmd/<name>/main.go`, appends to the Manifest, and fetches modules (iv:README.md#L42-L52). It never touches GitHub settings (iv:README.md#L139). To remove or rename a binary, you edit things by hand (iv:README.md#L52).

### Kinds (built in, not extensible without forking)

| Kind | Deployable? | Prompts | Template deps |
|---|---|---|---|
| `http` | yes | port (default: next free port from 8080, range 1024–65535, unique). For the first http binary, also "Create a client layer?" (default no) | mjolnir `health`, `router`, `utils/log` |
| `job` | yes | schedule: `hourly`/`daily`/`weekly`/`monthly`/`none` (default) or a 5-field cron expression | mjolnir `utils/log` |
| `tool` | no | nothing | stdlib `flag` |

Sources: iv:README.md#L54-L66; iv:internal/kind/http.go#L12-L59; iv:internal/kind/job.go#L13-L60; iv:internal/kind/kind.go#L20-L22; iv:internal/templates/kinds/http.go.tmpl#L11-L13; iv:internal/templates/kinds/job.go.tmpl#L11.

The generated `http` server listens on `$PORT`, falling back to the port from the Manifest. It serves probes, and its readiness check **reports not-ready until you implement it** (iv:README.md#L64). For goblog, that means the server is never ready until someone writes that check.

---

## 2. Everything it generates

From `scaffold.Skeleton` (iv:internal/scaffold/scaffold.go#L159-L205) plus each Kind's files. Ivaldi splits what it writes into "Managed" files (identical in every Project, don't edit) and "Unmanaged" files (written once, then yours) (iv:README.md#L102-L119):

| Path | Managed? | Content / source |
|---|---|---|
| `go.mod` | Unmanaged, always kept if present | `module <path>\n\ngo 1.27` (scaffold.go#L163) |
| `Makefile` | Unmanaged | only `include ivaldi.mk` (iv:internal/templates/Makefile) |
| `ivaldi.mk` | **Managed** | targets `lint`, `test`, `build` (default goal), `run-<name>`, `install`, `setup-hooks`. Binaries are discovered from `cmd/*/main.go` (iv:internal/templates/ivaldi.mk#L12-L41) |
| `.githooks/pre-commit` (0755) | **Managed** | `set -eu`. If `golangci-lint` isn't on `PATH`, it exits 1 with install instructions. Otherwise it runs `make lint test` (iv:internal/templates/pre-commit#L7-L16) |
| `.github/workflows/ci.yml` | **Managed** | caller of the shared workflow, pinned by SHA (§2.1) |
| `.golangci.yml` | Unmanaged | maratori's golden config (golangci-lint v2) plus `goimports.local-prefixes: [<module path>]` plus depguard rules per Layer (iv:internal/templates/golangci.yml.tmpl#L1-L13, #L38-L39, #L176-L234) |
| `.gitignore` | Unmanaged, appended to if present | `/bin/`, `/logs/` (iv:internal/templates/gitignore) |
| `README.md` | Unmanaged, kept if present | skeleton titled `path.Base(modulePath)` (iv:internal/templates/README.md.tmpl) |
| `api/doc.go`, `service/doc.go`, `data/doc.go`, `models/doc.go` | Unmanaged, kept if present | Layer package docs (scaffold.go#L89, #L197-L213) |
| `client/doc.go` | Unmanaged, optional | only if you answer yes for the first http binary (iv:internal/kind/http.go#L47-L57) |
| `cmd/<name>/main.go` | Unmanaged | from the Kind's template |
| `.forge.yaml` | Unmanaged, Ivaldi only appends | the Manifest: `version: 1`, then `binaries: {<name>: {kind, port \| schedule, [base_image], [build: dockerfile]}}` (iv:README.md#L82-L100) |

**What it doesn't generate:** no Dockerfile, ever (iv:README.md#L100). No Kubernetes or other deploy manifests: deploys are left to "Brokkr", which isn't built yet (iv:GLOSSARY.md#L59-L61; iv:docs/brokkr-notes.md#L3). No `internal/` directory (iv:README.md#L70).

### 2.1 Layer rules (enforced by depguard in lint)

| Layer | May import |
|---|---|
| `api` | `service`, `models` |
| `service` | `data`, `models` |
| `data` | `models` |
| `models` | nothing |
| `client` | `models` |

Sources: iv:internal/scaffold/scaffold.go#L102-L136; iv:README.md#L68-L80.

Packages outside the Layers and `cmd/` aren't constrained (iv:README.md#L40). That covers goblog's existing `blog/`, `shared/` and `webhook/`.

### 2.2 CI: the generated `ci.yml`

On `pull_request` to `main`, `push` to `main`, and tags `v*.*.*`, the generated workflow has `permissions: contents: read` and one job (iv:internal/templates/ci.yml#L5-L24):

```yaml
uses: worldtree-industries/workflows/.github/workflows/go.yml@e27aff84ed75bc3995068bce81b197c7dd6eafff # v1.0.0
secrets:
  HARBOR_USERNAME: ${{ secrets.HARBOR_USERNAME }}
  HARBOR_PASSWORD: ${{ secrets.HARBOR_PASSWORD }}
```

The SHA and version are compiled into Ivaldi as constants (iv:internal/scaffold/scaffold.go#L138-L147, #L227-L232).

### 2.3 The shared workflow (`wf:.github/workflows/go.yml`)

- **Secrets:** `HARBOR_USERNAME` and `HARBOR_PASSWORD`, both `required: true`. Its only input is `fixture`, for internal use (wf:.github/workflows/go.yml#L8-L23).
- **Hard-coded environment:** `REGISTRY: registry.werewolves.fyi`, `GOLANGCI_LINT_VERSION: v2.14.0`, `KO_VERSION: v0.19.1` (#L28-L31). The image name is hard-coded too: `IMAGE: registry.werewolves.fyi/library/${{ matrix.name }}` (#L139).
- **Jobs:**
  - `lint`: golangci-lint-action.
  - `test`: `go test -race ./...`, with Go taken from `go.mod`.
  - `affected`: builds `cmd/plan` and `cmd/affected` from the workflow repo at `job.workflow_sha` to decide which Deployables to build, the tag, and whether to push.
  - `images`: one matrix entry per affected Deployable, `linux/amd64` only. It logs into Harbor only when pushing, then builds with ko on `base_image`, or with `docker/build-push-action` if `build: dockerfile` is set. It uses a GHA cache.

  Sources: #L33-L202.
- **Tags and pushes:** a PR builds and never logs in. A push to `main` pushes `sha-<short>`. A `vX.Y.Z` tag pushes Deployables affected since the previous tag (wf:README.md#L75-L83).
- **The default base image** is `gcr.io/distroless/static-debian12:nonroot` (wf:README.md#L151-L153; iv:README.md#L100).
- The workflows README says to pass the secrets by name and "never `secrets: inherit`" (wf:README.md#L56-L57).

### 2.4 GitHub settings `init` applies

These run via `gh api --method PUT` (iv:internal/cli/github.go#L10-L41; iv:README.md#L131-L139):

- `actions/permissions`: `enabled=true`, `allowed_actions=selected`.
- `actions/permissions/selected-actions`: `github_owned_allowed=true`, `verified_allowed=false`, and `patterns_allowed` set to `worldtree-industries/*`, `golangci/golangci-lint-action@*`, `docker/login-action@*`, `docker/setup-buildx-action@*` and `docker/build-push-action@*`.
- `actions/permissions/workflow`: `default_workflow_permissions=read`, `can_approve_pull_request_reviews=false`.

These settings need `origin` on GitHub and `gh` with admin rights. Ivaldi checks admin rights with `gh api repos/<repo> --jq .permissions.admin` (github.go#L58-L66).

---

## 3. Every worldtree-industries-specific assumption

| Assumption | Where | Why it matters for goblog |
|---|---|---|
| CI calls `worldtree-industries/workflows/.github/workflows/go.yml@e27aff8…` | iv:internal/templates/ci.yml#L21; scaffold.go#L144-L147 | That repo is public, so a public repo outside the org can call it. But you depend on another org's workflow and its release cadence. |
| Image registry `registry.werewolves.fyi` (Harbor), project `library` | wf:.github/workflows/go.yml#L29, #L139; wf:README.md#L53, #L68-L69 | **Hard-coded, with no input to override it.** Pushes from goblog go there or nowhere. |
| Image name is `library/<binary>`, with **no repo or project prefix** | wf:.github/workflows/go.yml#L139; iv:docs/brokkr-notes.md#L36 | A binary named `server` would push `registry.werewolves.fyi/library/server` and collide with any other Project's `server`. Give the binary a distinctive name, such as `goblog`. |
| Secrets `HARBOR_USERNAME` / `HARBOR_PASSWORD`, described as **org-level** secrets | iv:internal/templates/ci.yml#L23-L24; wf:README.md#L51-L54; iv:docs/adr/0001-projects-are-public-by-default.md#L3 | Org secrets aren't visible to a user-owned repo. goblog has **no secrets** today (`gh secret list -R dfryer1193/goblog` is empty). |
| Allowed-actions pattern `worldtree-industries/*` | iv:internal/cli/github.go#L12 | Required for the shared workflow to run. Fine as is. A fork under `dfryer1193/*` would need this pattern changed. |
| Projects are public by default (GitHub Free limits) | iv:docs/adr/0001-projects-are-public-by-default.md | goblog is public, so `patterns_allowed` takes effect and runner minutes are free. |
| mjolnir as the framework: module `github.com/dfryer1193/mjolnir@latest` | iv:internal/kind/kind.go#L22 | That GitHub repo now **redirects to `worldtree-industries/mjolnir`** (`gh repo view dfryer1193/mjolnir` returns the worldtree URL), but its `go.mod` still declares `module github.com/dfryer1193/mjolnir`, latest tag `v1.8.2`. It's public, so it isn't a private dependency, but it relies on GitHub's rename redirect. Ivaldi's own docs disagree on the location (iv:README.md#L64 vs iv:docs/extending-ivaldi.md#L10). |
| Go 1.27 minimum (from mjolnir) | iv:internal/cli/modulepath.go#L74-L75 | goblog's `go.mod` is `go 1.25.1`, so `init` will offer to raise it. |
| Deploys by "Brokkr" GitOps (not built yet), reading `.forge.yaml` and Harbor images | iv:GLOSSARY.md#L59-L61; iv:docs/brokkr-notes.md | No deploy path exists, for the org or for you. |
| `ivaldi-hooks` skill and "Managed files kept current by a separate system" | iv:README.md#L104; iv:skills/ivaldi-hooks/SKILL.md#L8-L11 | It fetches templates from `worldtree-industries/ivaldi@main`. That works for anyone, since the repo is public. |
| Issue tracker and agent docs point to the worldtree repo | iv:AGENTS.md#L7 | Ivaldi's own repo only. These files aren't generated into Projects. |

**Not found anywhere:** `vars.*` references, private registries or private Go modules, Dockerfiles, deploy manifests, or domains other than `registry.werewolves.fyi` (searched with `grep -rn 'secrets\.\|vars\.'` over both repos at the pinned SHAs). The only secrets are the two Harbor secrets.

---

## 4. Configurable at init time vs. manual post-init edits

**Configurable at init (prompts only):** module path (and only when there's no `go.mod`), binary names, Kinds, ports, schedules, whether to create the `client` Layer, whether to overwrite each existing file, and whether to raise the go line (iv:docs/extending-ivaldi.md#L15).

**Configurable afterwards by editing `.forge.yaml`, with nothing to regenerate:** `base_image`, `build: dockerfile`, port and schedule (iv:README.md#L84, #L95-L100).

**Not configurable at all. Changing these means editing generated files or forking:**

- The shared workflow repo, its SHA, and the secret names in `ci.yml`. These are compiled-in constants, and the file is "Managed: don't edit". In practice nothing keeps Managed files current yet: the README only says "a separate system's job" (iv:README.md#L104), so editing it is safe today, but it will diverge from upstream.
- Registry, image path, platform (amd64 only), and lint and ko versions. These live in the shared workflow, so changing them means forking `worldtree-industries/workflows`.
- The allowed-actions list. It's compiled in, so you'd change it afterwards with `gh api`.
- Kinds, Layer names and rules (the rules are editable in the Unmanaged `.golangci.yml`), and the mjolnir dependency.

---

## 5. Caveats for use outside the org

1. **The reusable workflow is callable.** `worldtree-industries/workflows` is public, and the workflow checks out its own commands via `job.workflow_repository` / `job.workflow_sha` (wf:.github/workflows/go.yml#L94-L107), which also works from outside the org. Lint, test and PR image builds should therefore run without any secrets, because PRs never log in (wf:README.md#L79).
2. **Pushes to `main` and tag pushes need Harbor credentials.** Org-level secrets don't reach `dfryer1193/goblog`. The ADR's documented escape route is **repo-level** `HARBOR_*` secrets (iv:docs/adr/0001-projects-are-public-by-default.md#L8). You'd need a Harbor robot account from whoever runs `registry.werewolves.fyi`, with push rights on `library`.
   - *Inference, not verified:* with the secrets unset, the `required: true` secrets reach the callee as empty strings, so the `docker/login-action` step would fail on push-to-main and tag runs.
3. **The registry isn't configurable.** If you want images somewhere else (for example `ghcr.io/dfryer1193/goblog`), you have two options:
   - (a) Fork `workflows` to your own account, change `REGISTRY`/`IMAGE` and the login, then point `ci.yml` at the fork and add `dfryer1193/*` to `patterns_allowed`. For GHCR, the callee would also need `packages: write`, and the caller's `permissions: contents: read` would block that, so `ci.yml` would need editing too.
   - (b) Replace `ci.yml` with your own workflow.
4. **Image names are global within `library`,** so name the Deployable something unique (§3).
5. **GitHub settings will change goblog's Actions policy.** Today it's `allowed_actions: all` (from `gh api repos/dfryer1193/goblog/actions/permissions`). After `init` it becomes a selected list, and any other action goblog later uses must be added to it. You have admin rights on the repo (`.permissions.admin == true`), so `init` *will* apply these settings automatically.
6. **Running in this git worktree:** `init` checks `git rev-parse --show-toplevel` against the directory (iv:internal/cli/cli.go#L267-L273). A worktree's top level is the worktree directory, so it won't `git init`. `core.hooksPath` is written to the repo config, which is shared by all worktrees.
7. **Deploys:** none are provided. Brokkr doesn't exist (iv:docs/brokkr-notes.md#L3), so you'd have to deploy goblog's image yourself.
8. **Lint strictness:** maratori's golden config is very strict, and the pre-commit hook blocks commits on any finding. Existing code will need cleanup, or `.golangci.yml` adjustments, before the first commit goes through the hook.

---

## Checklist

### Secrets this repo must provide

- [ ] `HARBOR_USERNAME`: repo-level secret. A Harbor robot account on `registry.werewolves.fyi` with push rights on the `library` project. Needed only for pushes to `main` and `vX.Y.Z` tags. PRs don't need it.
- [ ] `HARBOR_PASSWORD`: that robot account's password/token.
- (No `vars.*` are referenced anywhere. If you swap the registry, these two are replaced by whatever your own workflow needs, for example nothing beyond `GITHUB_TOKEN` with `packages: write` for GHCR.)

Command to set them: `gh secret set HARBOR_USERNAME -R dfryer1193/goblog` (and the same for `HARBOR_PASSWORD`), as described in the ADR's escape route.

### Post-init adjustments likely needed

- [ ] Decide on the registry. Either get Harbor robot credentials (above), or fork `worldtree-industries/workflows` / rewrite `.github/workflows/ci.yml` to push elsewhere. The shared workflow has no registry input.
- [ ] Name the http binary uniquely (for example `goblog`, not `server`), because the image path is `library/<binary>` with no project prefix.
- [ ] If you change the workflow source, update the allowed-actions `patterns_allowed` (add `dfryer1193/*` or similar) with `gh api --method PUT repos/dfryer1193/goblog/actions/permissions/selected-actions …`. Also add any other actions you use.
- [ ] Accept raising `go.mod` from `go 1.25.1` to `go 1.27`, and make sure the local toolchain and CI can use it (CI reads `go-version-file: go.mod`).
- [ ] Implement the generated readiness check in `cmd/<name>/main.go`. It reports not-ready until you do.
- [ ] Add a `cmd/<name>/Dockerfile` with `build: dockerfile` only if ko on distroless isn't enough. For example, a blog that reads markdown from a git repo at runtime may need `git` in the image; a ko build would need a `base_image` that has it. Ivaldi never writes Dockerfiles.
- [ ] Every developer clone: run `make setup-hooks` (`core.hooksPath` isn't cloned) and install golangci-lint v2.
- [ ] Expect lint cleanup or `.golangci.yml` tuning for the strict golden config before the pre-commit hook passes.
- [ ] Deployment is yours to build. There is no Brokkr, and there are no manifests.
- [ ] Optional: the generated `ci.yml` and `ivaldi.mk` are marked "Managed: don't edit". If you edit them, expect drift from upstream Ivaldi, and don't let a future sync tool overwrite your changes.
