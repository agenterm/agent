# AgenTerm CLI

A security gate for AI coding agents. Intercepts dangerous operations from Claude Code and Gemini CLI, sends approval requests to [AgenTerm](https://agenterm.app) for human review, and blocks or allows execution based on the response.

## Install

One-line install (auto-detects OS and architecture):

```bash
curl -fsSL https://raw.githubusercontent.com/agenterm/cli/main/install.sh | sh
```

Or download manually from [Releases](https://github.com/agenterm/cli/releases), or build from source:

```bash
go build -o agenterm ./cmd/agenterm
```

## Quick Start

```bash
# 1. Configure relay credentials (get push key from AgenTerm app)
agenterm init

# 2. Install hooks (all supported agents)
agenterm hook install

# Or install for a specific agent
agenterm hook install claude
agenterm hook install gemini

# Done. Dangerous operations will now require approval via AgenTerm.
```

## Commands

### `init`

Set up relay connection. Interactive by default, or pass flags directly:

```bash
agenterm init --push-key <KEY>
```

`--relay-url` is optional (defaults to `https://push.agenterm.app`).

Config is saved to `~/.agenterm/config.json`.

### `hook`

Manage hook integration with AI agents:

```bash
agenterm hook install              # Install to all supported agents
agenterm hook install claude       # Install to Claude Code only
agenterm hook install gemini       # Install to Gemini CLI only
agenterm hook uninstall            # Remove from all
agenterm hook uninstall claude     # Remove from Claude Code only
```

- Claude Code: writes `PermissionRequest` hook to `~/.claude/settings.json`
- Gemini CLI: writes `BeforeTool` hook to `~/.gemini/settings.json`

### `propose`

Create an approval proposal:

```bash
agenterm propose --title "Delete user data" --body "user_id=123"
```

| Flag | Default | Description |
|------|---------|-------------|
| `--title` | (required) | Proposal title |
| `--body` | (required) | Proposal body |
| `--wait` | `true` | Wait for approval result |
| `--timeout` | `60` | Wait timeout in seconds |

### `proposal status`

Check proposal status:

```bash
agenterm proposal status <proposal_id>
```

## Built-in Safety Rules

The gate matches these patterns and requires approval:

| Pattern | Description |
|---------|-------------|
| `rm -(rf\|fr\|r)` | Recursive delete |
| `git push --force` | Force push |
| `git push ...main` | Push to main branch |
| `git reset --hard` | Hard reset |
| `DROP TABLE` | Drop database table |
| `DELETE FROM` | Delete from database |
| `chmod 777` | Overly permissive file mode |
| `kill -9` | Force kill process |
| `> /...` | Overwrite file via redirect |

For non-Bash tools (Write, Edit, etc.), the entire tool input is scanned — so a `DROP TABLE` inside file content will also trigger approval.

## Release

Pushing a version tag triggers the [release workflow](.github/workflows/release.yml), which automatically:

1. Runs tests
2. Cross-compiles binaries for linux/darwin (amd64/arm64)
3. Generates SHA256 checksums
4. Creates a GitHub Release with auto-generated release notes

```bash
git tag v0.1.0
git push origin v0.1.0
```

Binaries in the release:

```
agenterm-linux-amd64
agenterm-linux-arm64
agenterm-darwin-amd64
agenterm-darwin-arm64
checksums.txt
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Approved / pending / no approval needed |
| 1 | Denied |
| 2 | Error (missing config, bad args, timeout) |
