# Ugudu

<p align="center">
  <img src="images/ugudu_orc.png" alt="Ugudu" width="800">
</p>

AI Team Orchestration - Create and manage teams of AI agents that collaborate like a real development team.

## Install

**macOS/Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/arcslash/ugudu/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/arcslash/ugudu/main/install.ps1 | iex
```

**Homebrew:**
```bash
brew tap arcslash/ugudu https://github.com/arcslash/ugudu
brew install ugudu
```

**Go:**
```bash
go install github.com/arcslash/ugudu/cmd/ugudu@latest
```

## Quick Start

```bash
# 1. Configure API key
ugudu config init

# 2. Start daemon (includes web UI)
ugudu daemon

# 3. Open web UI
open http://localhost:8080
```

## Usage

### Create a Team

```bash
# AI-powered team creation
ugudu create "I need a team to build a REST API"

# Or use a template
ugudu team create my-team --spec dev-team
```

### Talk to Your Team

```bash
# Send a message (goes to PM by default)
ugudu ask my-team "Build a user authentication system"

# Talk to specific role
ugudu ask my-team "Review the login code" --to qa
```

### Web UI

The web UI is built into the binary. Start the daemon and open http://localhost:8080

- Create and manage teams
- Chat with team members
- View team status and progress

### MCP Integration (Claude Desktop)

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ugudu": {
      "command": "ugudu",
      "args": ["mcp"]
    }
  }
}
```

Now Claude can create teams and delegate work to them.

## Team Roles

| Role | What They Do |
|------|--------------|
| **PM** | Coordinates team, delegates tasks, reports to client |
| **BA** | Gathers requirements, creates specs |
| **Engineer** | Writes code using tools (read/write files, run commands) |
| **QA** | Tests code, reports bugs |

Each role has sandboxed access to only the tools they need.

## Configuration

Config file: `~/.ugudu/config.yaml`

```yaml
providers:
  anthropic:
    api_key: sk-ant-xxxxx

defaults:
  provider: anthropic
  model: claude-sonnet-4-20250514
```

Team specs go in: `~/.ugudu/specs/`

## License

MIT
