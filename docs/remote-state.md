# Remote State

cactl stores all policy state inside Git itself -- no external databases, no cloud storage buckets, no lock files on disk. This page explains how the state model works, how it travels to your remote, and how to operate it day-to-day.

## How state is stored locally

cactl uses two Git primitives that live entirely outside your working tree and branch history:

### Custom refs (current state)

Every tracked policy is a **blob object** pointed to by a ref in a custom namespace:

```
refs/cactl/tenants/<tenant-id>/policies/<slug>   # policy JSON blob
refs/cactl/tenants/<tenant-id>/manifest          # manifest JSON blob
```

These refs are invisible to `git log`, `git status`, and your branch graph. They never create files in your working directory. You can inspect them directly:

```bash
# List all cactl refs
git for-each-ref refs/cactl/

# Read a policy blob
git cat-file blob refs/cactl/tenants/<tenant-id>/policies/require-mfa-all-users

# Read the manifest
git cat-file blob refs/cactl/tenants/<tenant-id>/manifest
```

### Annotated tags (version history)

Every successful deploy creates an **annotated tag** pointing to the deployed policy blob:

```
cactl/<tenant-id>/<slug>/<semver>
```

For example:

```
cactl/a793bcba-.../require-mfa-all-users/1.0.0
cactl/a793bcba-.../require-mfa-all-users/1.1.0
cactl/a793bcba-.../require-mfa-all-users/2.0.0
cactl/a793bcba-.../block-legacy-auth/1.0.0
```

Tags are immutable. Rollback creates a new forward version rather than rewriting or deleting tags, so the full audit trail is always preserved.

### The manifest

The manifest is a JSON blob stored at `refs/cactl/tenants/<tenant-id>/manifest`. It maps each policy slug to its live Entra Object ID and deployment metadata:

```json
{
  "schema_version": 1,
  "tenant": "a793bcba-...",
  "policies": {
    "require-mfa-all-users": {
      "slug": "require-mfa-all-users",
      "tenant": "a793bcba-...",
      "live_object_id": "d4e5f6a7-...",
      "version": "2.0.0",
      "last_deployed": "2026-03-14T16:42:00Z",
      "deployed_by": "cactl/az-cli",
      "auth_mode": "az-cli",
      "backend_sha": "a1b2c3d4e5f6..."
    }
  }
}
```

The manifest is the glue between your local policy files and the live Entra objects. Without it, cactl would not know which Entra Object ID corresponds to which slug.

## How state reaches the remote

cactl configures a custom **refspec** on your `origin` remote so that `git push` and `git fetch` automatically include cactl state. This happens the first time you run `cactl import`.

The refspec added to `.git/config`:

```ini
[remote "origin"]
    fetch = +refs/cactl/*:refs/cactl/*
    push  = +refs/cactl/*:refs/cactl/*
```

With this in place:

```bash
# Push policy state + tags to remote
git push --follow-tags

# Pull latest state from remote
git fetch
```

That's it. No special commands. `git push` sends your custom refs and annotated tags; `git fetch` pulls them back. The `+` prefix on the refspec means fast-forward updates are forced, which is safe because cactl refs always move forward (new blobs replace old ones, tags are never rewritten).

### What gets pushed

| What | Refspec | Carried by |
|---|---|---|
| Policy blobs | `refs/cactl/tenants/*/policies/*` | `git push` (via custom refspec) |
| Manifest | `refs/cactl/tenants/*/manifest` | `git push` (via custom refspec) |
| Version tags | `cactl/<tenant>/<slug>/<semver>` | `git push --follow-tags` |
| Desired-state files | `policies/<tenant-id>/*.json` | Normal commits on your branch |

### Verifying the refspec

```bash
# Check that cactl refspecs are configured
git config --get-all remote.origin.fetch
git config --get-all remote.origin.push

# You should see entries containing refs/cactl/*
```

If the refspec is missing (e.g. you cloned the repo fresh), run `cactl import` or add it manually:

```bash
git config --add remote.origin.fetch '+refs/cactl/*:refs/cactl/*'
git config --add remote.origin.push  '+refs/cactl/*:refs/cactl/*'
```

## Sharing state across machines

Because state lives in Git, sharing it between machines, CI runners, and team members works the same way as sharing code:

```
Developer A                    Remote (GitHub/ADO)                Developer B
─────────────                  ──────────────────                 ─────────────
cactl apply                         │                                   │
  ├─ writes blob refs              │                                   │
  └─ creates version tag            │                                   │
                                     │                                   │
git push --follow-tags ─────────────►│                                   │
                                     │◄────────────────── git fetch      │
                                     │                    cactl plan     │
                                     │                      └─ reads blob refs
                                     │                         from local Git
```

### CI/CD runners

CI runners clone the repo and automatically receive cactl state via the custom refspec. A typical pipeline:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0          # full history so tags are available
    fetch-tags: true        # explicitly fetch tags

- run: |
    # Ensure cactl refs are fetched (checkout may not fetch custom refspecs)
    git fetch origin '+refs/cactl/*:refs/cactl/*'
    git fetch --tags

- run: cactl plan --ci --tenant ${{ vars.AZURE_TENANT_ID }}
```

The key detail: `actions/checkout` fetches standard refs only. You need an explicit `git fetch origin '+refs/cactl/*:refs/cactl/*'` to pull cactl state into the runner.

### Cloning a repo with existing state

When you clone a repo that already has cactl state on the remote:

```bash
git clone https://github.com/your-org/ca-policies.git
cd ca-policies

# Configure the refspec (one-time setup)
git config --add remote.origin.fetch '+refs/cactl/*:refs/cactl/*'
git config --add remote.origin.push  '+refs/cactl/*:refs/cactl/*'

# Pull state
git fetch origin
git fetch --tags

# Verify
cactl status --tenant YOUR_TENANT_ID
```

## Multi-tenant state isolation

Each tenant's state is fully namespaced:

```
refs/cactl/tenants/TENANT_A/manifest
refs/cactl/tenants/TENANT_A/policies/require-mfa-all-users
refs/cactl/tenants/TENANT_B/manifest
refs/cactl/tenants/TENANT_B/policies/require-mfa-all-users
```

Two tenants can have a policy with the same slug -- they will never collide because the tenant ID is part of every ref path and tag name.

## Inspecting and debugging state

```bash
# List all tracked policies for a tenant
git for-each-ref --format='%(refname:strip=5)' refs/cactl/tenants/<tenant-id>/policies/

# Read the current manifest
git cat-file blob refs/cactl/tenants/<tenant-id>/manifest | python3 -m json.tool

# Compare a policy blob against its on-disk desired state
diff <(git cat-file blob refs/cactl/tenants/<tenant-id>/policies/require-mfa-all-users) \
     policies/<tenant-id>/require-mfa-all-users.json

# List all version tags for a policy
git tag -l 'cactl/<tenant-id>/require-mfa-all-users/*'

# Read the content of a specific version
git cat-file blob cactl/<tenant-id>/require-mfa-all-users/1.0.0^{}

# Show tag metadata (who deployed, when, message)
git tag -v cactl/<tenant-id>/require-mfa-all-users/1.0.0
```

## Why Git and not a remote backend?

| Concern | Git-native approach |
|---|---|
| **No new infrastructure** | If you have a Git remote, you have a state backend. No S3 buckets, no Cosmos DB, no Terraform Cloud. |
| **Atomic with code** | Policy files and their state travel in the same push. There is no drift between "what the code says" and "what the state says." |
| **Audit trail for free** | Annotated tags carry author, timestamp, and message. `git log --all` covers everything. |
| **Offline-capable** | State is local. You can plan and diff without network access. |
| **Access control reuse** | Whoever can push to the repo can update state. No separate IAM to manage. |
| **Encryption at rest** | Handled by your Git host (GitHub, Azure DevOps, GitLab). |

The trade-off is that concurrent applies from different machines are not locked. Use CI/CD concurrency controls (`concurrency` in GitHub Actions, exclusive locks in Azure DevOps) to ensure only one apply runs per tenant at a time.
