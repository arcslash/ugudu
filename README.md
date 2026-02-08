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

#### 1. Configure Providers

First, set up your AI provider API keys:

1. Click **Providers** in the sidebar
2. Enter your API key for OpenRouter, Anthropic, OpenAI, or others
3. Click **Save** - the provider is ready immediately

#### 2. Create a Team Spec

Specs are blueprints that define your team structure:

1. Click **Specs** in the sidebar
2. Click **+ New Spec**
3. Enter a spec name (e.g., "my-dev-team")
4. Choose a default AI provider and model
5. Add roles using **+ Add Role** dropdown:
   - Select role type (PM, Engineer, QA, etc.)
   - Names and personas are auto-generated
   - Check **Client Facing** for roles that talk to you (usually PM)
   - Click **Advanced** to customize persona or use a different model per role
6. Click **Create Spec**

#### 3. Create a Team from Spec

1. Click **+ New Team** in the sidebar
2. Enter a team name
3. Select the spec you created
4. Click **Create**

The team appears in the sidebar with all members listed.

#### 4. Chat with Your Team

1. Click on your team in the sidebar
2. Click on a **client-facing** member (marked with 💬) - usually the PM
3. Type your request in the chat box, e.g.:
   - "Build me a REST API for user management"
   - "What's the status of the project?"
   - "Review the authentication code"
4. The PM will coordinate with other team members and report back

**Tips:**
- Only client-facing roles show the chat input
- Non-client-facing roles show their activity log
- The PM delegates to Engineers/QA automatically
- Status indicators show which agents are busy (pulsing) or idle

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
