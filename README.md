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

**From Source:**
```bash
git clone https://github.com/arcslash/ugudu.git
cd ugudu
make install
```

## Quick Start

```bash
# 1. Configure API key
ugudu setup

# 2. Start daemon (includes web UI)
ugudu daemon

# 3. Open web UI
open http://localhost:9741
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

The web UI is built into the binary. Start the daemon and open http://localhost:9741

- Create and manage teams
- Chat with team members
- Edit team specs with per-role model configuration
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

Each role can use different LLM providers/models. Configure per-role models in the web UI's spec editor.

## Configuration

Config file: `~/.ugudu/config.yaml`

```yaml
providers:
  openrouter:
    api_key: sk-or-xxxxx
  anthropic:
    api_key: sk-ant-xxxxx

defaults:
  provider: openrouter
  model: anthropic/claude-3.5-sonnet
```

Team specs go in: `~/.ugudu/specs/`

## License

MIT
