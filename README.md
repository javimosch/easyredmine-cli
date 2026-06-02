# easyredmine-cli
easyredmine-cli — Redmine API client for EasyRedmine. Read issues, post comments, edit descriptions.

Part of [supercli](https://github.com/javimosch/supercli) ecosystem.

```bash
# Build from source (requires Go 1.22+)
cd ~/ai/easyredmine-cli
go build -ldflags="-s -w" -o easyredmine-cli main.go
sudo mv easyredmine-cli /usr/local/bin/
```

## Configuration

```bash
easyredmine-cli config set
```

Token stored in `~/.config/easyredmine-cli/config.json`.

## Usage

```bash
easyredmine-cli issue show 61809
easyredmine-cli issue show 61809 --json

easyredmine-cli issue comment 61809 --text "Looks good to me"
easyredmine-cli issue edit 61809 --description "<p>Updated</p>"
```

---

This tool is a plugin for [supercli](https://github.com/javimosch/supercli) — an AI-friendly, config-driven dynamic CLI platform.
