# easyredmine-cli
easyredmine-cli — Redmine API client for EasyRedmine. Read issues, post comments, edit descriptions.

Agent-friendly: JSON output by default, `--human` for humans, `EASYREDMINE_API_KEY` env var auth, semantic exit codes.

Part of [supercli](https://github.com/javimosch/supercli) ecosystem.

```bash
# Build from source (requires Go 1.22+)
go build -ldflags="-s -w" -o easyredmine-cli main.go
sudo mv easyredmine-cli /usr/local/bin/
```

## Quick start

```bash
# Via env var (no config file needed)
export EASYREDMINE_API_KEY=your-key
easyredmine-cli issue show 61809

# Or via config
easyredmine-cli config set --api-key your-key
```

## Usage

```bash
# JSON output (default)
easyredmine-cli issue show 61809
easyredmine-cli issue comment 61809 --text "Message"
easyredmine-cli issue edit 61809 --description "<p>New desc</p>"

# Smart search (word-by-word, dedup, rank by match)
easyredmine-cli issue search "correction statut message"
easyredmine-cli issue search "correction statut" --limit 50 --human

# Human-readable
easyredmine-cli issue show 61809 --human
```

## Smart search

EasyRedmine's API doesn't support full-text search. `easyredmine-cli` works around this by:
1. Fetching all open issues concurrently (parallel page requests)
2. Breaking your query into individual words
3. Matching each word against issue subjects
4. Ranking results by number of word matches (issues matching more words rank first)
5. Deduplicating and sorting by match count + recency

```bash
easyredmine-cli issue search "phrase with multiple words"
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0    | Success |
| 85   | Invalid argument / config error |
| 92   | Resource not found |
| 105  | Integration / API error |
| 110  | Internal error |

Errors on stderr as structured JSON.

---

This tool is a plugin for [supercli](https://github.com/javimosch/supercli) — an AI-friendly, config-driven dynamic CLI platform.
