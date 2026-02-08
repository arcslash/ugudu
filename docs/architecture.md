---
layout: default
title: Architecture
---

# Architecture

Understanding how Ugudu works under the hood.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ugudu daemon                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────┐                                                    │
│  │     Manager         │  Orchestrates teams, providers, persistence        │
│  └──────────┬──────────┘                                                    │
│             │                                                                │
│  ┌──────────┼──────────────────────────────────────────────────────────┐   │
│  │          ▼          │                │                │             │   │
│  │  ┌─────────────┐   │  ┌─────────────┐   ┌─────────────┐            │   │
│  │  │ Team Alpha  │   │  │ Team Beta   │   │ Team Gamma  │   ...      │   │
│  │  │             │   │  │             │   │             │            │   │
│  │  │  PM ◄──► BA │   │  │  PM ◄──► BA │   │  Lead       │            │   │
│  │  │   │      │  │   │  │   │         │   │   │         │            │   │
│  │  │   ▼      ▼  │   │  │   ▼         │   │   ▼         │            │   │
│  │  │ Eng₁   Eng₂ │   │  │  Eng        │   │ Analyst₁    │            │   │
│  │  │   │      │  │   │  │   │         │   │ Analyst₂    │            │   │
│  │  │   └──┬───┘  │   │  │   ▼         │   │             │            │   │
│  │  │      ▼      │   │  │  QA         │   │             │            │   │
│  │  │     QA      │   │  │             │   │             │            │   │
│  │  └─────────────┘   │  └─────────────┘   └─────────────┘            │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐            │
│  │ Provider        │  │ Tool            │  │ Persistence     │            │
│  │ Registry        │  │ Registry        │  │ Store           │            │
│  │                 │  │                 │  │                 │            │
│  │ - Anthropic     │  │ - read_file     │  │ - Teams         │            │
│  │ - OpenAI        │  │ - write_file    │  │ - Conversations │            │
│  │ - Groq          │  │ - run_command   │  │ - Context       │            │
│  │ - Ollama        │  │ - git_*         │  │                 │            │
│  │ - OpenRouter    │  │ - create_task   │  │                 │            │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘            │
├─────────────────────────────────────────────────────────────────────────────┤
│                           Interfaces                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                    │
│  │   CLI    │  │ HTTP API │  │  Web UI  │  │   MCP    │                    │
│  │  ugudu   │  │  :8080   │  │ embedded │  │  server  │                    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### Manager

The central orchestrator that:
- Creates and manages teams
- Maintains provider registry
- Handles persistence
- Routes messages between CLI/API and teams

### Teams

Each team is an independent unit with:
- Multiple Members (agents)
- Internal message routing
- Task tracking
- Conversation history

### Members (Agents)

Each member:
- Has a specific role (PM, Engineer, QA, etc.)
- Runs as a goroutine
- Has its own inbox/outbox channels
- Gets a sandboxed tool registry

### Providers

LLM providers (Anthropic, OpenAI, etc.):
- Unified interface for all providers
- Rate limiting
- Model listing
- Health checks

## Message Flow

```
User Input
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ CLI: ugudu ask alpha "Build a login page"                   │
│   or                                                         │
│ Web UI: Chat message                                         │
│   or                                                         │
│ MCP: ugudu_ask tool call                                    │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Manager.Ask("alpha", "Build a login page")                  │
│   └─► Team.Ask(message)                                     │
│         └─► Routes to client-facing member (PM)             │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ PM Member                                                    │
│   1. Receives message in inbox                              │
│   2. Builds prompt with persona + history                   │
│   3. Calls LLM provider                                     │
│   4. Parses response for:                                   │
│      - DELEGATE TO engineer: Build the login form           │
│      - ASK CLIENT: What auth method?                        │
│      - COMPLETE: Here's the result...                       │
└─────────────────────────────────────────────────────────────┘
    │
    ▼ (if delegation)
┌─────────────────────────────────────────────────────────────┐
│ Engineer Member                                              │
│   1. Receives Task in inbox                                 │
│   2. Builds prompt with persona + task                      │
│   3. Calls LLM with tools available                         │
│   4. Executes tool calls:                                   │
│      - read_file("src/auth.go")                            │
│      - write_file("src/login.go", "...")                   │
│      - run_command("go build ./...")                       │
│   5. Reports completion back to PM                          │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Response flows back:                                         │
│   Engineer → PM → Client (via outbox channels)              │
└─────────────────────────────────────────────────────────────┘
```

## Sandboxed Tool Execution

Each agent gets isolated tool access:

```
┌─────────────────────────────────────────────────────────────┐
│ Engineer Member                                              │
│   toolRegistry: SandboxedRegistry                           │
│     ├─ role: "engineer"                                     │
│     ├─ allowed: [read_file, write_file, run_command, git_*] │
│     └─ sandbox: ~/ugudu_projects/proj/workspaces/engineer/  │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ SandboxedRegistry.Execute("write_file", {path: "main.go"})  │
│   1. Check: Is write_file allowed for engineer? ✓           │
│   2. Resolve path: main.go → .../workspaces/engineer/main.go│
│   3. Execute tool                                            │
│   4. Log activity                                            │
└─────────────────────────────────────────────────────────────┘
```

### Path Resolution

| Operation | Resolution |
|-----------|------------|
| Write | Always to sandbox: `workspaces/{role}/` |
| Read | Check sandbox first, then source, then shared paths |

This ensures:
- Agents can't overwrite each other's work
- Agents can read shared source code
- All writes are isolated

## Workspace Structure

```
~/ugudu_projects/
├── my-project/
│   ├── project.yaml           # Project config
│   ├── tasks/
│   │   └── tasks.json         # Shared task board
│   ├── activity/
│   │   ├── pm.log             # Activity logs (JSONL)
│   │   ├── engineer.log
│   │   └── qa.log
│   ├── artifacts/
│   │   ├── reports/           # PM reports
│   │   ├── specs/             # BA specifications
│   │   └── tests/             # QA test results
│   └── workspaces/
│       ├── pm/                # PM's sandbox
│       ├── engineer-1/        # Engineer sandboxes
│       ├── engineer-2/
│       └── qa/                # QA's sandbox
```

## Goroutine Architecture

All agents run as goroutines in a single process:

```go
// Each member runs its own goroutine
func (m *Member) run() {
    for {
        select {
        case <-m.ctx.Done():
            return
        case msg := <-m.inbox:
            m.handleMessage(msg)
        }
    }
}
```

Benefits:
- **Fast communication** - No network/IPC overhead
- **Shared memory** - Efficient resource sharing
- **Simple coordination** - Go channels for message passing
- **Single binary** - Everything in one process

## Persistence

SQLite database stores:
- Team configurations
- Conversation history
- Agent context (for continuity)
- Task states

```go
// Context is persisted after each message
m.addToContext("assistant", response)
    └─► Team.SaveMemberContext(memberID, role, content, sequence)
        └─► Store.SaveAgentContext(...)
```

When a team restarts, context is restored:

```go
// On team start
history := persistence.LoadContext(teamName, memberID, 50)
member.RestoreContext(history)
```

## Token Mode

Controls resource consumption:

| Mode | Max Tokens | Context | Prompt Style |
|------|-----------|---------|--------------|
| Normal | 4096 | 40 msgs | Full persona + responsibilities |
| Low | 1024 | 10 msgs | Condensed persona |
| Minimal | 512 | 5 msgs | Single line persona |

```go
func (m *Member) getEffectiveMaxTokens() *int {
    settings := m.Team.GetTokenSettings()
    return &settings.MaxTokens
}
```
