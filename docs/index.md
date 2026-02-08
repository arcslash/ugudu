---
layout: default
title: Home
---

# Ugudu

**AI Team Orchestration** - Create and manage teams of AI agents that collaborate like a real development team.

## What is Ugudu?

Ugudu lets you create teams of AI agents with distinct roles (PM, BA, Engineers, QA) that work together on complex tasks. Unlike simple AI chat, Ugudu agents:

- **Collaborate** - Agents communicate, delegate, and coordinate
- **Use Tools** - Engineers can read/write files, run commands
- **Stay in Role** - Each agent has specific responsibilities and permissions
- **Work in Parallel** - Multiple agents can work simultaneously

## Quick Example

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/arcslash/ugudu/main/install.sh | bash

# Configure
ugudu config init

# Start daemon (includes web UI)
ugudu daemon

# Create a team and give them work
ugudu team create alpha --spec dev-team
ugudu ask alpha "Build a REST API for user management"
```

The PM will analyze the request, delegate to engineers, coordinate with QA, and report back.

## Features

| Feature | Description |
|---------|-------------|
| **Multi-Agent Teams** | PM, BA, Engineers, QA working together |
| **Role-Based Tools** | Each role gets appropriate tools only |
| **Sandboxed Execution** | Agents work in isolated workspaces |
| **Web UI** | Built-in browser interface at localhost:8080 |
| **MCP Integration** | Use with Claude Desktop |
| **Single Binary** | Everything in one executable |

## Getting Started

1. [Installation](installation) - Install Ugudu on your system
2. [Quick Start](quickstart) - Create your first team
3. [Configuration](configuration) - Set up API keys and preferences
4. [Team Specs](team-specs) - Define custom team structures

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                     ugudu daemon                         │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │ Team Alpha  │  │ Team Beta   │  │ Team Gamma  │     │
│  │  PM ←→ BA   │  │  PM ←→ Eng  │  │  PM ←→ QA   │     │
│  │   ↓    ↓    │  │   ↓         │  │   ↓         │     │
│  │ Eng₁  Eng₂  │  │  QA         │  │  Eng        │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
├─────────────────────────────────────────────────────────┤
│        CLI  ←→  HTTP API  ←→  Web UI  ←→  MCP          │
└─────────────────────────────────────────────────────────┘
```

## License

MIT License - Free for personal and commercial use.
