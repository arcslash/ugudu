---
layout: default
title: MCP Integration
---

# MCP Integration

Use Ugudu with Claude Desktop and other MCP-compatible AI assistants.

## What is MCP?

Model Context Protocol (MCP) allows AI assistants to use external tools. With Ugudu's MCP server, Claude Desktop can:

- Create and manage AI teams
- Delegate complex tasks to teams
- Monitor team progress
- Answer team questions

## Setup

### 1. Ensure Ugudu is Installed

```bash
ugudu version
```

### 2. Start the Daemon

```bash
ugudu daemon
```

Keep this running (or run as a service).

### 3. Configure Claude Desktop

Edit `~/.claude/claude_desktop_config.json`:

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

### 4. Restart Claude Desktop

The Ugudu tools will now be available to Claude.

## Available Tools

### Team Management

| Tool | Description |
|------|-------------|
| `ugudu_list_teams` | List all teams |
| `ugudu_create_team` | Create a team from a spec |
| `ugudu_team_status` | Get team member status |
| `ugudu_start_team` | Start a stopped team |
| `ugudu_stop_team` | Stop a running team |
| `ugudu_delete_team` | Delete a team |

### Communication

| Tool | Description |
|------|-------------|
| `ugudu_ask` | Send message to a team |
| `ugudu_pending_questions` | Get questions awaiting answers |
| `ugudu_answer_question` | Answer a team's question |

### Project Workflow

| Tool | Description |
|------|-------------|
| `ugudu_start_project` | Start a collaborative project |
| `ugudu_project_status` | Get project progress |
| `ugudu_project_communications` | View team chat log |

### Configuration

| Tool | Description |
|------|-------------|
| `ugudu_list_specs` | List available team specs |
| `ugudu_list_specialists` | List specialist templates |
| `ugudu_daemon_status` | Check if daemon is running |
| `ugudu_set_token_mode` | Set token consumption mode |
| `ugudu_clear_conversation` | Clear team conversation history |

## Usage Examples

### Example 1: Simple Task Delegation

```
You: Create a dev team and have them build a todo list API

Claude: I'll create a team and start the project.
        [Uses ugudu_create_team with name="todo-team", spec="dev-team"]
        [Uses ugudu_ask with team="todo-team", message="Build a REST API..."]

        The team is working on it. Here's the initial response from Sarah (PM):
        "I'll coordinate the team to build this API..."
```

### Example 2: Interactive Project

```
You: Start a project to build a user authentication system

Claude: [Uses ugudu_create_team]
        [Uses ugudu_start_project]

        The team has started working. They have a question:
        "Should we use JWT or session-based authentication?"

You: Use JWT with refresh tokens

Claude: [Uses ugudu_answer_question]

        Thanks! The team is continuing with JWT implementation.
        Current progress:
        - PM assigned tasks to engineers
        - Engineer 1 is working on the JWT service
        - Engineer 2 is building the auth middleware
```

### Example 3: Checking Progress

```
You: What's the status of the todo-team?

Claude: [Uses ugudu_team_status]

        Team: todo-team
        Status: running

        Members:
        - Sarah (PM) - idle
        - Alex (BA) - idle
        - Mike (Engineer) - working on "Implement DELETE endpoint"
        - Julia (Engineer) - idle
        - Chris (QA) - idle
```

### Example 4: Token Optimization

```
You: The team is using too many tokens. Switch to low token mode.

Claude: [Uses ugudu_set_token_mode with team="todo-team", mode="low"]

        Done! The team is now in low token mode:
        - Condensed prompts
        - Reduced context history (10 messages)
        - Max 1024 tokens per response
```

## Best Practices

### 1. Let Claude Manage Teams

Instead of manually creating teams, describe what you need:

```
"I need a team to build a mobile app backend with expertise in
real-time features and push notifications"
```

Claude will choose appropriate specs or create custom teams.

### 2. Be Specific with Requirements

The more detail you provide, the better the team performs:

```
"Build a REST API with:
- User registration with email verification
- Login with JWT
- Password reset flow
- Rate limiting on auth endpoints
Use Go with the Gin framework and PostgreSQL"
```

### 3. Answer Questions Promptly

Teams may ask clarifying questions. Claude will surface these:

```
Claude: The team has a question: "Should passwords have
        specific complexity requirements?"
```

Answer to unblock the team.

### 4. Check Progress Periodically

```
"What's the team working on now?"
"Show me the project status"
"Any blockers?"
```

### 5. Use Token Modes Appropriately

- **Normal** - Complex tasks requiring full context
- **Low** - Simple tasks, cost-conscious
- **Minimal** - Very simple tasks, maximum savings

## Troubleshooting

### "Daemon not running"

Start the daemon:
```bash
ugudu daemon
```

### "Tool not found"

Ensure Claude Desktop config is correct and restart Claude.

### "Team not found"

List available teams:
```bash
ugudu team list
```

### Slow Responses

Teams with many agents or complex tasks take time. Consider:
- Using low token mode
- Simpler team structures
- Breaking large tasks into smaller ones
