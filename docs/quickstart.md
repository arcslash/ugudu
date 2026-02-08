---
layout: default
title: Quick Start
---

# Quick Start

Get up and running with Ugudu in 5 minutes.

## 1. Configure API Keys

Run the interactive configuration:

```bash
ugudu config init
```

This creates `~/.ugudu/config.yaml` with your API keys.

Or set environment variables:

```bash
export ANTHROPIC_API_KEY=sk-ant-xxxxx
# or
export OPENAI_API_KEY=sk-xxxxx
```

## 2. Start the Daemon

```bash
ugudu daemon
```

This starts:
- Unix socket for CLI communication
- HTTP API on port 8080
- Web UI at http://localhost:8080

Keep this running in a terminal or run as a background service.

## 3. Open the Web UI

Open http://localhost:8080 in your browser.

The web UI lets you:
- Create and manage teams
- Chat with team members
- View team status
- Control token usage mode

## 4. Create a Team

### Using the Web UI

1. Click "Create Team"
2. Enter a name (e.g., "alpha")
3. Select a spec (e.g., "dev-team")
4. Click Create

### Using the CLI

```bash
# List available specs
ugudu spec list

# Create a team from a spec
ugudu team create alpha --spec dev-team

# Start the team
ugudu team start alpha
```

## 5. Talk to Your Team

### Web UI

Select your team from the sidebar and type in the chat box.

### CLI

```bash
# Send a message (goes to PM by default)
ugudu ask alpha "Build a hello world API in Go"

# Talk to a specific role
ugudu ask alpha "Review the code for bugs" --to qa
```

## What Happens Next?

When you send a request:

1. **PM receives it** - Analyzes and plans the approach
2. **PM delegates** - Assigns tasks to appropriate team members
3. **Engineers work** - Use tools to read/write files, run commands
4. **QA reviews** - Tests and validates the work
5. **PM reports** - Summarizes results back to you

You can watch the activity in the web UI or check status:

```bash
ugudu team ps alpha
```

## Example Workflow

```bash
# Create a dev team
ugudu team create myteam --spec dev-team

# Give them a task
ugudu ask myteam "Create a REST API with these endpoints:
- GET /users - list all users
- POST /users - create a user
- GET /users/:id - get user by ID
- DELETE /users/:id - delete a user

Use Go with the Gin framework."

# Check progress
ugudu team ps myteam

# View what files they created
ls -la  # (files appear in your current directory or project workspace)
```

## Token Saving Mode

To reduce API costs:

```bash
# Low token mode - condensed prompts, smaller context
ugudu ask myteam "..." --low-token

# Minimal token mode - bare minimum
ugudu ask myteam "..." --minimal-token
```

Or set in the web UI under "Token Mode".

## Next Steps

- [Configuration](configuration) - Detailed config options
- [Team Specs](team-specs) - Create custom team structures
- [MCP Integration](mcp) - Use with Claude Desktop
- [Architecture](architecture) - Understand how it works
