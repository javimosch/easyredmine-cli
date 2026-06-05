---
name: easyredmine-cli
description: Use easyredmine-cli for Redmine API operations on EasyRedmine (Simpliciti). Read issues, post comments, edit descriptions, change status, assign users, search project members, and smart-search across all open issues. JSON output by default, EASYREDMINE_API_KEY env var for auth, semantic exit codes.
---

# easyredmine-cli

Redmine API client for EasyRedmine (Simpliciti). Agent-first: JSON output by default, env-var auth, no interactive prompts, semantic exit codes.

## Installation

```bash
# Build from source (requires Go 1.22+)
git clone https://github.com/javimosch/easyredmine-cli
cd easyredmine-cli
go build -ldflags="-s -w" -o easyredmine-cli main.go
sudo mv easyredmine-cli /usr/local/bin/
```

Verify:
```bash
easyredmine-cli version
# → easyredmine-cli v1.0.3
```

## Authentication

Three methods, highest precedence first:

### 1. Env var (preferred — no config file, works immediately)

```bash
export EASYREDMINE_API_KEY=your-key-here
easyredmine-cli issue show 61809
```

### 2. Config file

```bash
easyredmine-cli config set --api-key your-key-here
```

Stored at `~/.config/easyredmine-cli/config.json`.

### 3. Environment base URL override

```bash
export EASYREDMINE_BASE_URL=https://easyredmine.simpliciti.fr
```

## Commands

### `issue show <id>`

Read an issue's full details — subject, description, status, priority, all journals (comments + field changes).

```bash
# JSON output (default)
easyredmine-cli issue show 61809
# → {"issue":{"id":61809,"subject":"[Message] Correction...","description":"...","journals":[...]}}

# Human-readable
easyredmine-cli issue show 61809 --human
```

### `issue comment <id> --text "<text>"`

Add a comment to an issue.

```bash
# JSON (default)
easyredmine-cli issue comment 61809 --text "Looks good to me"
# → {"ok":true,"issue_id":"61809","action":"comment"}

# Human
easyredmine-cli issue comment 61809 --text "Done" --human
# → Comment added to issue #61809
```

### `issue edit <id> --description "<text>"`

Replace an issue's description.

```bash
easyredmine-cli issue edit 61809 --description "<p>Updated description</p>"
```

### `issue status <id> --status-id <status_id>`

Change issue status.

```bash
easyredmine-cli issue status 61809 --status-id 51
# → {"ok":true,"issue_id":"61809","action":"status_change","status_id":51}
```

### `issue assign <id> --assigned-to-id <user_id>`

Assign issue to a user or group.

```bash
easyredmine-cli issue assign 61809 --assigned-to-id 199
# → {"ok":true,"issue_id":"61809","action":"assign","assigned_to_id":199}
```

### `user search "<query>" --project-id <id>`

Search users, groups, and roles within a project for assignment.

```bash
easyredmine-cli user search "QA" --project-id 1111
# → {"results":[{"id":46,"fullname":"Equipe QA Env (Group)"}],"total":1,"returned":1,"query":"QA"}
```

**Agent workflow**: When asked to "Assign to QA", search first to get the ID, then assign:
```bash
# 1. Find QA team ID
easyredmine-cli user search "QA" --project-id 1111
# → {"results":[{"id":46,"fullname":"Equipe QA Env (Group)"},...]}

# 2. Assign using the ID
easyredmine-cli issue assign 62507 --assigned-to-id 46
```

### `update [--check-only]`

Check for updates from GitHub releases and optionally auto-update.

```bash
# Check for updates only
easyredmine-cli update --check-only

# Check and install if update available
easyredmine-cli update
```

**Note**: If no GitHub releases exist yet, the command provides manual update instructions.

### `issue search "<phrase>"` — smart search

Breaks a phrase into individual words (filtering stop words), fetches all matching open issues, and ranks results by the number of word matches per issue.

```bash
easyredmine-cli issue search "correction statut message"
# → {"results":[{"id":61809,"match_count":3,...}],"total":23,...}
```

#### Algorithm

1. **Tokenize** → split on whitespace, lowercase, strip punctuation, remove stop words (French + English)
2. **Count** matching issues via API (`status_id=open&limit=1`)
3. **Fetch** matching issues concurrently (20 goroutines, 100/page)
4. **Match** each word against each issue's **subject** (case-insensitive)
5. **Rank** by `match_count` descending, tie-break by `updated_on` descending
6. **Paginate** by `--limit` / `--offset`

#### Date filters (API-side — faster)

These add `updated_on>=date` at the API level, reducing pages:

```bash
# ~3 seconds (1 page instead of 37)
easyredmine-cli issue search "message" --current-month

# ~15 seconds (~8 pages)
easyredmine-cli issue search "correction" --current-year

# Custom range
easyredmine-cli issue search "statut api" --after 2026-05-01
```

#### Search flags

| Flag | Default | Purpose |
|---|---|---|
| `--limit <n>` | 20 | Max results |
| `--offset <n>` | 0 | Pagination offset |
| `--status <s>` | `open` | Status filter |
| `--current-month` | false | Only this month (API-side) |
| `--current-year` | false | Only this year (API-side) |
| `--after YYYY-MM-DD` | — | After date (API-side) |
| `--min-matches <n>` | 1 | Minimum word matches |

#### Output format

```json
{
  "results": [
    {
      "id": 61809,
      "subject": "[Message] Correction de l'incohérence du statut",
      "project": {"id": 1101, "name": "Evolutions Diverses"},
      "status": {"id": 51, "name": "Dev In Progress"},
      "match_count": 2,
      "matched_words": ["message", "statut"],
      "updated_on": "2026-06-02T10:33:05Z"
    }
  ],
  "total": 15,
  "returned": 5,
  "query": "message statut",
  "words": ["message", "statut"]
}
```

Progress events on stderr.

## Output conventions

- **stdout**: Primary data (JSON by default)
- **stderr**: Progress, warnings, errors
- **Exit code 0**: Success

## Semantic exit codes

| Code | Meaning | Agent action |
|---|---|---|
| 0 | Success | Proceed |
| 85 | Invalid argument / config | Fix input, don't retry |
| 92 | Resource not found | Try different ID |
| 105 | Integration / API error | Retry with backoff |
| 110 | Internal error | Report bug |

Errors on stderr as structured JSON.

## Global flags

| Flag | Effect |
|---|---|
| `--human` / `-H` | Human-readable output |
| `--version` / `-v` | Show version |

## Shell composition

```bash
# Pipe search to jq
easyredmine-cli issue search "correction" --current-month | jq '.results[].id'

# Chain commands
easyredmine-cli issue search "statut api" --limit 3 | \
  jq -r '.results[].id' | \
  xargs -I {} easyredmine-cli issue show {}
```
