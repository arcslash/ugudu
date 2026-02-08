---
layout: default
title: Configuration
---

# Configuration

Ugudu stores configuration in `~/.ugudu/`.

## Directory Structure

```
~/.ugudu/
├── config.yaml      # Main configuration
├── specs/           # Team specifications
│   ├── dev-team.yaml
│   └── my-custom-team.yaml
├── data/            # Runtime data
└── ugudu.db         # SQLite database
```

## Configuration File

`~/.ugudu/config.yaml`:

```yaml
# LLM Providers
providers:
  anthropic:
    api_key: sk-ant-xxxxx

  openai:
    api_key: sk-xxxxx
    base_url: https://api.openai.com/v1  # Optional, for proxies

  groq:
    api_key: gsk_xxxxx

  ollama:
    url: http://localhost:11434

  openrouter:
    api_key: sk-or-xxxxx

# Defaults
defaults:
  provider: anthropic
  model: claude-sonnet-4-20250514

# Daemon settings
daemon:
  tcp_addr: :8080  # HTTP API port
```

## Environment Variables

Environment variables override config file values:

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GROQ_API_KEY` | Groq API key |
| `OLLAMA_URL` | Ollama server URL |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `UGUDU_HOME` | Override config directory (default: ~/.ugudu) |
| `UGUDU_PROJECTS` | Override projects directory (default: ~/ugudu_projects) |

## Supported Providers

### Anthropic (Recommended)

```yaml
providers:
  anthropic:
    api_key: sk-ant-xxxxx
```

Models: `claude-sonnet-4-20250514`, `claude-opus-4-20250514`, `claude-haiku-3-20240307`

### OpenAI

```yaml
providers:
  openai:
    api_key: sk-xxxxx
```

Models: `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo`

### Groq (Fast inference)

```yaml
providers:
  groq:
    api_key: gsk_xxxxx
```

Models: `llama-3.1-70b-versatile`, `mixtral-8x7b-32768`

### Ollama (Local)

```yaml
providers:
  ollama:
    url: http://localhost:11434
```

Models: Any model you've pulled (e.g., `llama3`, `codellama`, `mistral`)

### OpenRouter (Multi-provider)

```yaml
providers:
  openrouter:
    api_key: sk-or-xxxxx
```

Access to Claude, GPT, Gemini, Mistral, and many more through one API.

## Token Modes

Control token consumption for cost savings:

| Mode | Max Tokens | Context History | Use Case |
|------|-----------|-----------------|----------|
| `normal` | 4096 | 40 messages | Full capabilities |
| `low` | 1024 | 10 messages | Reduced cost |
| `minimal` | 512 | 5 messages | Maximum savings |

Set per-request:
```bash
ugudu ask myteam "..." --low-token
```

Or in team spec:
```yaml
settings:
  token:
    mode: low
    max_tokens: 1024
    context_history: 10
```

## Multi-Provider Teams

Different roles can use different providers:

```yaml
roles:
  pm:
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514

  engineer:
    model:
      provider: openai
      model: gpt-4o

  qa:
    model:
      provider: groq
      model: llama-3.1-70b-versatile
```

## Projects Directory

Agent workspaces are stored in `~/ugudu_projects/`:

```
~/ugudu_projects/
├── my-project/
│   ├── project.yaml
│   ├── tasks/
│   ├── activity/
│   ├── artifacts/
│   └── workspaces/
│       ├── pm/
│       ├── engineer/
│       └── qa/
```

Override with:
```bash
export UGUDU_PROJECTS=/path/to/projects
```

## CLI Commands

```bash
# Initialize config interactively
ugudu config init

# Show current config
ugudu config show

# Set a value
ugudu config set defaults.provider openai
```
