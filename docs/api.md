---
layout: default
title: API Reference
---

# API Reference

Ugudu exposes an HTTP API on port 8080 when the daemon is running.

## Base URL

```
http://localhost:8080/api
```

## Teams

### List Teams

```http
GET /api/teams
```

**Response:**
```json
[
  {
    "name": "alpha",
    "status": "running",
    "spec": "dev-team",
    "members": [
      {
        "id": "pm-1",
        "role": "pm",
        "title": "Product Manager",
        "name": "Sarah",
        "status": "idle",
        "client_facing": true
      },
      {
        "id": "engineer-1",
        "role": "engineer",
        "title": "Software Engineer",
        "name": "Mike",
        "status": "working",
        "current_task": "Implementing login endpoint",
        "client_facing": false
      }
    ]
  }
]
```

### Create Team

```http
POST /api/teams
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "my-team",
  "spec": "dev-team"
}
```

**Response:**
```json
{
  "name": "my-team",
  "status": "stopped",
  "spec": "dev-team"
}
```

### Get Team Status

```http
GET /api/teams/{name}
```

**Response:**
```json
{
  "name": "alpha",
  "status": "running",
  "spec": "dev-team",
  "members": [...],
  "created_at": "2024-01-15T10:30:00Z",
  "token_mode": "normal"
}
```

### Start Team

```http
POST /api/teams/{name}/start
```

**Response:**
```json
{
  "status": "running"
}
```

### Stop Team

```http
POST /api/teams/{name}/stop
```

**Response:**
```json
{
  "status": "stopped"
}
```

### Delete Team

```http
DELETE /api/teams/{name}
```

**Response:**
```json
{
  "deleted": true
}
```

## Communication

### Send Message to Team

```http
POST /api/teams/{name}/ask
Content-Type: application/json
```

**Request Body:**
```json
{
  "message": "Build a REST API for user management",
  "to": "pm"
}
```

The `to` field is optional. If omitted, the message goes to the default client-facing member (usually PM).

**Response:**
```json
{
  "response": "I'll coordinate the team to build this API...",
  "from": {
    "id": "pm-1",
    "role": "pm",
    "name": "Sarah"
  }
}
```

### Get Pending Questions

```http
GET /api/teams/{name}/questions
```

**Response:**
```json
{
  "questions": [
    {
      "id": "q-123",
      "from": {
        "id": "engineer-1",
        "role": "engineer",
        "name": "Mike"
      },
      "question": "Should we use JWT or session-based authentication?",
      "asked_at": "2024-01-15T11:00:00Z"
    }
  ]
}
```

### Answer Question

```http
POST /api/teams/{name}/questions/{question_id}/answer
Content-Type: application/json
```

**Request Body:**
```json
{
  "answer": "Use JWT with refresh tokens"
}
```

**Response:**
```json
{
  "acknowledged": true
}
```

## Conversation

### Get Conversation History

```http
GET /api/teams/{name}/conversation
```

**Query Parameters:**
- `limit` - Number of messages (default: 50)
- `before` - Messages before this timestamp

**Response:**
```json
{
  "messages": [
    {
      "id": "msg-1",
      "role": "user",
      "content": "Build a login page",
      "timestamp": "2024-01-15T10:30:00Z"
    },
    {
      "id": "msg-2",
      "role": "assistant",
      "from": {
        "id": "pm-1",
        "role": "pm",
        "name": "Sarah"
      },
      "content": "I'll coordinate the team...",
      "timestamp": "2024-01-15T10:30:05Z"
    }
  ]
}
```

### Clear Conversation

```http
DELETE /api/teams/{name}/conversation
```

**Response:**
```json
{
  "cleared": true
}
```

## Token Mode

### Set Token Mode

```http
POST /api/teams/{name}/token-mode
Content-Type: application/json
```

**Request Body:**
```json
{
  "mode": "low"
}
```

Valid modes: `normal`, `low`, `minimal`

**Response:**
```json
{
  "mode": "low",
  "max_tokens": 1024,
  "context_history": 10
}
```

### Get Token Mode

```http
GET /api/teams/{name}/token-mode
```

**Response:**
```json
{
  "mode": "normal",
  "max_tokens": 4096,
  "context_history": 40
}
```

## Specs

### List Available Specs

```http
GET /api/specs
```

**Response:**
```json
{
  "specs": [
    {
      "name": "dev-team",
      "description": "Full development team with PM, BA, Engineers, and QA",
      "roles": ["pm", "ba", "engineer", "qa"]
    },
    {
      "name": "research-team",
      "description": "Research and analysis team",
      "roles": ["lead", "analyst", "writer"]
    }
  ]
}
```

### Get Spec Details

```http
GET /api/specs/{name}
```

**Response:**
```json
{
  "name": "dev-team",
  "description": "Full development team",
  "roles": {
    "pm": {
      "title": "Product Manager",
      "visibility": "client",
      "can_delegate": ["ba", "engineer", "qa"]
    },
    "engineer": {
      "title": "Software Engineer",
      "count": 2,
      "visibility": "internal",
      "reports_to": "pm"
    }
  }
}
```

## Specialists

### List Specialist Templates

```http
GET /api/specialists
```

**Response:**
```json
{
  "specialists": [
    {
      "name": "security-engineer",
      "title": "Security Engineer",
      "description": "Specializes in security audits and secure coding"
    },
    {
      "name": "devops-engineer",
      "title": "DevOps Engineer",
      "description": "CI/CD, infrastructure, deployment"
    }
  ]
}
```

## Daemon

### Health Check

```http
GET /api/health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "uptime": "2h30m15s"
}
```

### Daemon Status

```http
GET /api/status
```

**Response:**
```json
{
  "running": true,
  "teams_count": 3,
  "active_agents": 12,
  "providers": {
    "anthropic": "connected",
    "openai": "connected",
    "ollama": "not configured"
  }
}
```

## WebSocket

### Real-time Updates

```
WS /api/ws
```

Connect to receive real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.type) {
    case 'message':
      // New message from team member
      console.log(`${data.from.name}: ${data.content}`);
      break;
    case 'status':
      // Agent status change
      console.log(`${data.agent} is now ${data.status}`);
      break;
    case 'task':
      // Task update
      console.log(`Task ${data.task_id}: ${data.action}`);
      break;
  }
};
```

**Message Types:**

| Type | Description |
|------|-------------|
| `message` | New message from a team member |
| `status` | Agent status change (idle, working, waiting) |
| `task` | Task created, updated, or completed |
| `question` | Agent asking a question |
| `delegation` | Task delegated between agents |

## Error Responses

All endpoints return errors in this format:

```json
{
  "error": "Team not found",
  "code": "TEAM_NOT_FOUND",
  "details": {
    "team": "alpha"
  }
}
```

**Common Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `TEAM_NOT_FOUND` | 404 | Team doesn't exist |
| `TEAM_NOT_RUNNING` | 400 | Team is stopped |
| `SPEC_NOT_FOUND` | 404 | Spec doesn't exist |
| `INVALID_REQUEST` | 400 | Malformed request body |
| `PROVIDER_ERROR` | 502 | LLM provider error |
| `RATE_LIMITED` | 429 | Too many requests |

## Rate Limiting

The API implements rate limiting per client IP:

- 100 requests per minute for most endpoints
- 20 requests per minute for `/ask` endpoints (LLM calls)

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705320000
```
