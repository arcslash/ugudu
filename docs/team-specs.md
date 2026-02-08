---
layout: default
title: Team Specs
---

# Team Specifications

Team specs define the structure and behavior of your AI teams.

## Location

Specs are stored in `~/.ugudu/specs/`. Ugudu comes with built-in templates.

## Basic Structure

```yaml
apiVersion: ugudu/v1
kind: Team
metadata:
  name: my-team
  description: My custom team

client_facing:
  - pm

roles:
  pm:
    title: Product Manager
    visibility: client
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are the Product Manager...
    can_delegate:
      - engineer
      - qa

  engineer:
    title: Software Engineer
    visibility: internal
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are a Software Engineer...
    reports_to: pm
```

## Complete Reference

### Metadata

```yaml
metadata:
  name: team-name        # Required: unique identifier
  description: "..."     # Optional: human-readable description
```

### Client Facing

```yaml
client_facing:
  - pm      # Roles that can receive messages from users
  - ba
```

### Roles

Each role defines an agent:

```yaml
roles:
  role_name:
    # Display name
    title: "Product Manager"

    # Personal name (optional)
    name: "Sarah"

    # For multiple instances
    names: ["Alice", "Bob"]
    count: 2

    # Visibility
    visibility: client  # or "internal"

    # Model configuration
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
      temperature: 0.7        # Optional
      max_tokens: 4096        # Optional
      low_token_model: "..."  # Fallback for low token mode

    # Agent personality/instructions
    persona: |
      You are the Product Manager...

    # Condensed persona for low token mode
    persona_condensed: |
      PM. Coordinate team, delegate tasks.

    # Responsibilities (shown in prompts)
    responsibilities:
      - requirement_gathering
      - task_delegation
      - progress_tracking

    # Delegation permissions
    can_delegate:
      - engineer
      - qa

    # Reporting structure
    reports_to: pm
```

### Team Settings

```yaml
settings:
  token:
    mode: normal      # normal, low, minimal
    max_tokens: 4096
    context_history: 40

workflow:
  pattern: hub-spoke  # PM coordinates all
  auto_assign: true
```

## Built-in Templates

### dev-team

Full development team:
- PM - coordinates
- BA - requirements
- Engineer (x2) - coding
- QA - testing

```bash
ugudu team create myteam --spec dev-team
```

### research-team

Research and analysis:
- Lead Researcher
- Analyst (x2)
- Writer

### trading-team

Crypto/trading analysis:
- Analyst
- Risk Manager
- Trader

## Creating Custom Specs

### AI-Powered Creation

```bash
ugudu create "I need a team to build mobile apps with iOS and Android specialists"
```

This generates a spec based on your description.

### Manual Creation

```bash
# Interactive wizard
ugudu spec new my-team

# Or create file directly
cat > ~/.ugudu/specs/my-team.yaml << 'EOF'
apiVersion: ugudu/v1
kind: Team
metadata:
  name: my-team
...
EOF
```

### View/Edit Specs

```bash
# List all specs
ugudu spec list

# View a spec
ugudu spec show dev-team

# Edit (opens in $EDITOR)
ugudu spec edit my-team
```

## Role Tool Access

Each role automatically gets access to appropriate tools:

| Role | Tool Categories |
|------|-----------------|
| `engineer` | filesystem, command, git, communication |
| `pm` | planning, communication |
| `qa` | testing, command, communication |
| `ba` | documentation, communication |

### Tool Categories

```
filesystem:    read_file, write_file, edit_file, list_files, search_files
command:       run_command
git:           git_status, git_diff, git_commit, git_log, git_branch
planning:      create_task, update_task, list_tasks, assign_task, delegate_task
testing:       run_tests, create_bug_report, verify_fix, list_test_results
documentation: create_doc, create_requirement, create_spec
communication: ask_colleague, report_progress
```

## Example: Healthcare Dev Team

```yaml
apiVersion: ugudu/v1
kind: Team
metadata:
  name: healthcare-team
  description: Team for HIPAA-compliant healthcare software

client_facing:
  - pm

roles:
  pm:
    title: Product Manager
    name: Sarah
    visibility: client
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are a PM specializing in healthcare software.
      Ensure all work follows HIPAA guidelines.
    can_delegate:
      - ba
      - engineer
      - qa
      - healthcare_sme

  healthcare_sme:
    title: Healthcare SME
    name: Dr. Chen
    visibility: internal
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are a healthcare domain expert.
      Advise on FHIR, HL7, HIPAA, clinical workflows.
    reports_to: pm

  engineer:
    title: Software Engineer
    names: [Mike, Julia]
    count: 2
    visibility: internal
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are a healthcare software engineer.
      Write secure, HIPAA-compliant code.
    reports_to: pm

  qa:
    title: QA Engineer
    name: Chris
    visibility: internal
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are a QA engineer for healthcare software.
      Test for security, compliance, and reliability.
    reports_to: pm

  ba:
    title: Business Analyst
    name: Alex
    visibility: internal
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are a BA specializing in healthcare requirements.
      Create HIPAA-compliant specifications.
    reports_to: pm
```
