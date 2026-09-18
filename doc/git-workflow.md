# Git Branching & Release Workflow

## Branch Structure

| Branch | Purpose |
|--------|---------|
| `main` | Production — always reflects the **latest stable release** |
| `dev` | Integration — all new features are developed here |
| `release/X.Y` | Maintenance — patch/hotfix fixes for version X.Y only |

## Tags

| Tag | Commit | Description |
|-----|--------|-------------|
| `v1.0.0` | `efa0e75` | Legacy v1 baseline (preserved) |
| `v3.0.0` | `353ed95` | Current stable release |

---

## Patching v3 (Bug Fix on Current Stable)

When a bug is found in the current stable release (v3.x.x):

### 1. Switch to the release branch

```bash
git checkout release/3.0
```

### 2. Create a fix branch (optional but recommended)

```bash
git checkout -b hotfix/describe-the-fix
```

### 3. Make your changes, commit, and push

```bash
git add .
git commit -m "fix: describe what was fixed"
git push origin hotfix/describe-the-fix
```

### 4. Merge the fix into release/3.0

```bash
git checkout release/3.0
git merge hotfix/describe-the-fix
git push origin release/3.0
```

### 5. Tag the patch version

```bash
git tag -a v3.0.1 -m "Patch: describe the fix"
git push origin v3.0.1
```

### 6. Merge release/3.0 into main (update production)

Create a PR on GitHub: `release/3.0` → `main`, then merge.

### 7. Merge the fix back into dev (so future versions get it too)

```bash
git checkout dev
git merge release/3.0
git push origin dev
```

### 8. Delete the hotfix branch

```bash
git branch -d hotfix/describe-the-fix
git push origin --delete hotfix/describe-the-fix
```

### Patch Flow Diagram

```
hotfix/fix-xyz ──▶ release/3.0 ──▶ main (production updated)
                        │
                        └──────▶ dev (fix lands in future versions)

Tag: v3.0.0 → v3.0.1 → v3.0.2 ...
```

---

## Releasing v4 (New Major/Minor Version)

When `dev` has accumulated enough features and is stable:

### 1. Ensure dev is stable and up to date

```bash
git checkout dev
git pull origin dev
```

### 2. Create the release branch from dev

```bash
git checkout -b release/4.0
git push origin release/4.0
```

### 3. Tag the release

```bash
git tag -a v4.0.0 -m "Release v4.0.0 — new features"
git push origin v4.0.0
```

### 4. Merge release/4.0 into main (update production)

Create a PR on GitHub: `release/4.0` → `main`, then merge.

### 5. Merge back into dev (sync any last-minute release changes)

```bash
git checkout dev
git merge release/4.0
git push origin dev
```

### 6. Continue developing on dev

`dev` is now the base for v4.x features. The `release/4.0` branch stays open for v4 patches.

### Release Flow Diagram

```
dev ─────────▶ release/4.0 ──▶ main (new production)
                 │
                 └────▶ dev (sync back)

Tag: v4.0.0

Old release/3.0 stays available for v3 patches if needed.
```

---

## Quick Reference

| Scenario | Work on | Merge to | Tag |
|----------|----------|----------|-----|
| New feature | `dev` | — | — |
| Bug fix (current v3) | `release/3.0` | `main` + `dev` | `v3.0.x` |
| Bug fix (next v4) | `release/4.0` | `main` + `dev` | `v4.0.x` |
| New stable release | `dev` → `release/X.0` | `main` + `dev` | `vX.0.0` |

---

## Version Numbering

```
v<MAJOR>.<MINOR>.<PATCH>

MAJOR  — Breaking changes, large feature sets (3 → 4)
MINOR  — New features, backward-compatible (4.0 → 4.1)
PATCH  — Bug fixes only (4.1.0 → 4.1.1)