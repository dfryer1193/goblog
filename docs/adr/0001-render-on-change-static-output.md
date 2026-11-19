# Render the whole site on change, serve static files, no database

goblog runs in a memory-constrained Talos Kubernetes cluster. So instead of rendering pages per request, or storing posts in a database, it keeps a local clone of the Content Repository. When the Content Repository changes (signalled by a webhook or found by polling), goblog re-renders the entire site into a fresh output directory and then switches over to it all at once. Every request is served as a static file. The Content Repository is the only source of truth, and the rendered output is disposable, so v1 has no database.

## Considered Options

- **A pure static site generator, built in the Content Repository's CI and served by Caddy/nginx.** This puts the least load on the cluster. It was rejected because it throws away the webhook/pull model and the Go server that Ivaldi scaffolds. It would also make later dynamic features, such as comments, a separate system.
- **A live server that renders per request or keeps posts in a database (the old goblog design).** Rejected because it does more work and uses more memory per request, all to keep a store that only duplicates the Content Repository.

## Consequences

Comments, which are planned but not in v1, will be the first thing that needs a database and dynamic endpoints. They will sit alongside the static output rather than replacing it.
