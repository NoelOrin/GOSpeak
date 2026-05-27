---
name: git-commit-workflow
description: Use when creating branches, committing, merging, or any git workflow operation. Triggers on "git branch", "commit", "提交", "merge", "rebase", "同步master", "切分支", "提测", "上线", "hotfix".
---

# Git Commit Workflow

## Overview

Enforce the company's four-branch git workflow: `master`, `feature/*`, `release/*`, `hotfix/*`. Guide branch creation, naming, commit messages, master sync, and merge discipline.

## When to Use

- User says "create branch", "切分支", "新建分支"
- User says "commit", "提交", "git commit"
- User says "merge", "rebase", "同步master", "同步"
- User says "提测", "上线", "发版", "release"
- User says "hotfix", "紧急修复", "回滚", "rollback"
- Preparing a PR/MR or reviewing branch readiness

**Do NOT use when:** Debugging git merge conflicts, explaining basic git commands, or writing documentation unrelated to workflow.

---

## 1. Branch Creation Rules

### 1.1 Naming Format

| Branch Type | Format | Example |
|-------------|--------|---------|
| Feature | `feature/<module>-<brief>` or `feature/<JIRA>` | `feature/user-login`, `feature/PROJ-1234` |
| Personal sub-branch | `feature/<parent>-<role>` | `feature/user-login-frontend` |
| Release | `release/v<semver>` | `release/v1.2.0` |
| Hotfix | `hotfix/<brief>` | `hotfix/pay-500-error` |

Rules:
- All lowercase.
- Use hyphens `-` only. No underscores, spaces, or Chinese.
- Keep to 3–5 words.

### 1.2 Origin Rules

| Branch Type | Must cut from |
|-------------|---------------|
| `feature/*` | Latest `master` |
| `release/*` | Corresponding `feature/*` (after test pass) |
| `hotfix/*` | Latest `master` tag |

**Before creating any branch:**
```bash
git checkout master
git pull origin master
```

---

## 2. Mandatory Master Sync Points

Sync latest `master` into your branch **before** these actions:

| Trigger | Reason |
|---------|--------|
| Before QA testing | Ensure test env matches production |
| Before cutting `release/*` | Ensure release contains all shipped code |
| After another `feature` merges to `master` | Avoid stale baseline |
| After any `hotfix` merges to `master` | Prevent overwriting fixes |

Sync command (merge, recommended for feature/release):
```bash
git checkout feature/xxx
git fetch origin
git merge origin/master
```

If conflicts exist, resolve them, then:
```bash
git add .
git commit -m "merge: sync master"
```

---

## 3. Commit Message Rules

Follow **Conventional Commits** (matching `commitlint.config.js`):

```
type(scope): subject
```

### 3.1 Allowed Types

| Type | Meaning |
|------|---------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Code formatting, no logic change |
| `refactor` | Restructuring, neither feat nor fix |
| `perf` | Performance improvement |
| `test` | Test-related |
| `chore` | Build/tooling/deps |
| `revert` | Revert previous commit |
| `build` | Build system / Docker / Vite |
| `ci` | CI pipeline changes |
| `types` | Type definitions |
| `wip` | Work in progress |

### 3.2 Subject Rules

- ≤ 72 characters
- Imperative present tense (`add`, not `added`)
- No trailing period
- Lowercase after the colon
- One commit = one logical unit. Do NOT mix unrelated changes.

### 3.3 Scope Guidelines

| Changed Area | Scope |
|--------------|-------|
| `src/api/services/*` | `api` |
| `src/pages/*` | `pages` or page name |
| `src/components/*` | `components` |
| `src/store/*` | `store` |
| `src/router/*` | `router` |
| Cross-cutting / trivial | omit scope |

---

## 4. Branch Lifecycle Rules

### 4.1 Feature Branch

- Lifetime: days to ~1 week. **Never exceed 2 weeks** — split if needed.
- Must sync master at least once per week.
- Delete after release is complete.

### 4.2 Release Branch

- Cut from `feature/*` only after QA passes.
- **Only bug fixes allowed.** No new features.
- Delete after merging to `master` and tagging.

### 4.3 Hotfix Branch

- Cut from latest `master` tag.
- **Minimum fix only.** No feature work, no refactoring, no optimization.
- Merge to `master` immediately, tag as patch (`v1.2.0` → `v1.2.1`).
- **Must sync to all active `feature/*` and `release/*` branches.**
- Delete immediately after merge.

---

## 5. Merge & PR Rules

### 5.1 Master Protection (Red Lines)

1. **NEVER push directly to `master`.**
2. All merges to `master` must go through PR/MR with at least 1 approver.
3. PR must pass CI before merge.
4. Commit messages must pass `commitlint`.

### 5.2 Pre-Merge Checklist

Before approving or merging any `release/*` or `hotfix/*`:

- [ ] Branch is based on latest `master` (not behind)
- [ ] CI pipeline passes
- [ ] QA test / hotfix verification confirmed
- [ ] No unresolved conflicts
- [ ] Commit messages valid
- [ ] For release: no new-feature commits mixed in
- [ ] For hotfix: minimal scope confirmed

### 5.3 After Merge

1. Tag `master` immediately:
   ```bash
   git tag -a v1.2.0 -m "Release v1.2.0: user login"
   git push origin v1.2.0
   ```
2. Notify all active feature owners to sync master.
3. Delete the merged `feature/*`, `release/*`, or `hotfix/*` branch.

---

## 6. Special Scenarios

### 6.1 Release Validation Finds a Bug

- If the bug is from an **already shipped** feature → create `hotfix/*` from `master`, fix, merge, tag, sync.
- Do **not** fix inside the `release/*` branch if the feature is already on master.

### 6.2 Two Features Need to Ship Together

- **Queue them.** Merge A to master first. B syncs master, then merges.
- Never merge two `release/*` branches into master simultaneously.

### 6.3 Need to Rollback After Release

```bash
git revert <merge-commit-hash>
git push origin master
```

Then redeploy the previous stable tag. Follow up with `hotfix` or next `feature` cycle.

### 6.4 Feature Exceeds 2 Weeks

Split into independently shippable sub-features. Each gets its own `feature/*` branch.

---

## 7. Workflow Quick Reference

### 7.1 Normal Iteration

```bash
# 1. Create feature from latest master
git checkout master && git pull origin master
git checkout -b feature/user-login

# 2. Develop, commit regularly
git add .
git commit -m "feat(user): add login form"

# 3. Before QA / cutting release — sync master
git fetch origin
git merge origin/master

# 4. After QA pass, cut release
git checkout -b release/v1.2.0

# 5. Release validation fixes only
git commit -m "fix(user): correct login redirect"

# 6. Merge to master via PR, tag, notify, delete branches
```

### 7.2 Hotfix

```bash
# 1. Cut from latest master tag
git checkout master
git pull origin master
git checkout -b hotfix/pay-500-error

# 2. Minimal fix
git commit -m "fix(pay): resolve 500 on checkout"

# 3. Merge to master via PR, tag patch
git tag -a v1.2.1 -m "Hotfix v1.2.1: pay 500 error"

# 4. Sync hotfix to all active feature/release branches
# 5. Delete hotfix branch
```

---

## Verification

Before guiding the user through any workflow step, verify:
- [ ] Current branch follows naming convention
- [ ] Branch origin matches the table in §1.2
- [ ] Commit type is in allowed list
- [ ] Subject ≤ 72 chars, imperative, no period
- [ ] Master sync is suggested at mandatory points
- [ ] No direct push to master is ever recommended
