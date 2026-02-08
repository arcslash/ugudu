<script lang="ts">
  import { currentView, currentTeam, teamMembers, selectedMember, currentTeamName } from '../stores/app';
  import { stopTeam, deleteTeam } from '../lib/api';
  import { teams } from '../stores/app';
  import { wsConnected, activityFeed } from '../stores/websocket';

  async function handleStopTeam() {
    if (!$currentTeamName) return;
    try {
      await stopTeam($currentTeamName);
      teams.update(list =>
        list.map(t => t.name === $currentTeamName ? { ...t, status: 'stopped' as const } : t)
      );
    } catch (e) {
      console.error('Failed to stop team:', e);
    }
  }

  async function handleDeleteTeam() {
    if (!$currentTeamName) return;
    if (!confirm(`Delete team "${$currentTeamName}"? This cannot be undone.`)) return;

    try {
      await deleteTeam($currentTeamName);
      teams.update(list => list.filter(t => t.name !== $currentTeamName));
      currentTeamName.set(null);
      currentTeam.set(null);
      selectedMember.set(null);
    } catch (e) {
      console.error('Failed to delete team:', e);
    }
  }

  function selectMember(member: any) {
    selectedMember.set(member);
  }

  function getStatusColor(status: string): string {
    switch (status) {
      case 'busy':
      case 'working': return 'var(--warning)';
      case 'thinking': return 'var(--accent)';
      case 'error': return 'var(--error)';
      default: return 'var(--success)';
    }
  }

  function getStatusLabel(status: string): string {
    switch (status) {
      case 'busy': return 'Working';
      case 'working': return 'Working';
      case 'thinking': return 'Thinking';
      case 'error': return 'Error';
      default: return 'Idle';
    }
  }
</script>

<aside class="right-panel">
  {#if $currentView === 'team' && $currentTeam}
    <div class="panel-header">
      <h2>Team Members</h2>
      <div class="header-meta">
        <span class="ws-status" class:connected={$wsConnected} title={$wsConnected ? 'Real-time connected' : 'Connecting...'}>
          {$wsConnected ? '●' : '○'}
        </span>
        <span class="member-count">{$teamMembers.length} agents</span>
      </div>
    </div>

    <div class="members-list">
      {#each $teamMembers as member}
        <button
          class="member-card"
          class:active={$selectedMember?.id === member.id}
          class:client-facing={member.client_facing}
          on:click={() => selectMember(member)}
        >
          <div class="member-avatar">
            {member.name.charAt(0).toUpperCase()}
          </div>
          <div class="member-info">
            <div class="member-name">{member.name}</div>
            <div class="member-title">{member.title}</div>
            {#if member.model}
              <div class="member-model">{member.model}</div>
            {/if}
          </div>
          <div class="member-status">
            <div
              class="status-dot"
              class:pulsing={member.status === 'busy' || member.status === 'working'}
              style="background: {getStatusColor(member.status)}"
              title={member.statusMessage || getStatusLabel(member.status)}
            ></div>
            {#if member.client_facing}
              <span class="client-facing-badge" title="Client Facing">💬</span>
            {/if}
          </div>
        </button>
      {:else}
        <div class="empty-members">
          <p>No team members loaded</p>
        </div>
      {/each}
    </div>

    {#if $currentTeam}
      <div class="team-actions">
        <button class="btn-action" on:click={handleStopTeam}>
          ⏹️ Stop Team
        </button>
        <button class="btn-action danger" on:click={handleDeleteTeam}>
          🗑️ Delete Team
        </button>
      </div>
    {/if}

    {#if $selectedMember}
      <div class="member-details">
        <h3>Agent Details</h3>
        <div class="detail-row">
          <span class="detail-label">Name</span>
          <span class="detail-value">{$selectedMember.name}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">Role</span>
          <span class="detail-value">{$selectedMember.role}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">Title</span>
          <span class="detail-value">{$selectedMember.title}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">Status</span>
          <span class="detail-value status" style="color: {getStatusColor($selectedMember.status)}">
            {getStatusLabel($selectedMember.status)}
          </span>
        </div>
        {#if $selectedMember.statusMessage}
          <div class="detail-row">
            <span class="detail-label">Activity</span>
            <span class="detail-value task">{$selectedMember.statusMessage}</span>
          </div>
        {/if}
        {#if $selectedMember.provider}
          <div class="detail-row">
            <span class="detail-label">Provider</span>
            <span class="detail-value">{$selectedMember.provider}</span>
          </div>
        {/if}
        {#if $selectedMember.model}
          <div class="detail-row">
            <span class="detail-label">Model</span>
            <span class="detail-value">{$selectedMember.model}</span>
          </div>
        {/if}
        <div class="detail-row">
          <span class="detail-label">Client Facing</span>
          <span class="detail-value">{$selectedMember.client_facing ? 'Yes' : 'No'}</span>
        </div>
        {#if $selectedMember.task}
          <div class="detail-row">
            <span class="detail-label">Current Task</span>
            <span class="detail-value task">{$selectedMember.task}</span>
          </div>
        {/if}
      </div>
    {/if}

  {:else if $currentView === 'specs'}
    <div class="panel-header">
      <h2>Spec Info</h2>
    </div>
    <div class="help-content">
      <p>Team specs define the structure and roles for your AI teams.</p>
      <h4>Components:</h4>
      <ul>
        <li><strong>Roles</strong> - Define agent responsibilities</li>
        <li><strong>Provider</strong> - LLM service to use</li>
        <li><strong>Model</strong> - Default AI model</li>
        <li><strong>Client Facing</strong> - Who can chat with users</li>
      </ul>
    </div>

  {:else if $currentView === 'providers'}
    <div class="panel-header">
      <h2>Provider Info</h2>
    </div>
    <div class="help-content">
      <p>Configure API keys in your environment or config file.</p>
      <h4>Supported Providers:</h4>
      <ul>
        <li>OpenRouter</li>
        <li>OpenAI</li>
        <li>Anthropic</li>
        <li>Google AI</li>
      </ul>
    </div>

  {:else}
    <div class="panel-header">
      <h2>Getting Started</h2>
    </div>
    <div class="help-content">
      <h4>Quick Start:</h4>
      <ol>
        <li>Create a team spec with roles</li>
        <li>Create a team from your spec</li>
        <li>Chat with the PM to assign tasks</li>
        <li>Watch your AI team work!</li>
      </ol>
    </div>
  {/if}
</aside>

<style>
  .right-panel {
    background: var(--bg-secondary);
    border-left: 1px solid var(--border);
    padding: 24px;
    height: 100vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }

  .panel-header h2 {
    font-size: 16px;
    font-weight: 600;
  }

  .header-meta {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .ws-status {
    font-size: 10px;
    color: var(--text-muted);
    transition: color 0.3s;
  }

  .ws-status.connected {
    color: var(--success);
  }

  .member-count {
    font-size: 12px;
    color: var(--text-muted);
    background: var(--bg-tertiary);
    padding: 4px 10px;
    border-radius: 12px;
  }

  .members-list {
    flex: 1;
    overflow-y: auto;
    margin-bottom: 16px;
  }

  .member-card {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 12px;
    background: none;
    border: 1px solid transparent;
    border-radius: 10px;
    cursor: pointer;
    margin-bottom: 8px;
    transition: all 0.2s ease;
    text-align: left;
    color: var(--text-primary);
  }

  .member-card:hover {
    background: var(--bg-tertiary);
    border-color: var(--border-light);
  }

  .member-card.active {
    background: var(--bg-card);
    border-color: var(--accent);
    box-shadow: 0 0 15px var(--accent-glow);
  }

  .member-card.client-facing {
    border-left: 3px solid var(--accent);
  }

  .member-avatar {
    width: 40px;
    height: 40px;
    border-radius: 10px;
    background: var(--gradient-warm);
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    font-size: 16px;
    color: var(--bg-primary);
    flex-shrink: 0;
  }

  .member-info {
    flex: 1;
    min-width: 0;
  }

  .member-name {
    font-weight: 600;
    font-size: 14px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .member-title {
    font-size: 12px;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .member-model {
    font-size: 10px;
    color: var(--text-muted);
    margin-top: 2px;
  }

  .member-status {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    transition: background 0.3s ease;
  }

  .status-dot.pulsing {
    animation: pulse 1.5s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% {
      transform: scale(1);
      opacity: 1;
    }
    50% {
      transform: scale(1.3);
      opacity: 0.7;
    }
  }

  .client-facing-badge {
    font-size: 14px;
  }

  .empty-members {
    color: var(--text-muted);
    text-align: center;
    padding: 20px;
    font-size: 13px;
  }

  .team-actions {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 20px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }

  .btn-action {
    width: 100%;
    padding: 10px 16px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.2s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .btn-action:hover {
    background: var(--bg-card);
    border-color: var(--border-light);
  }

  .btn-action.danger:hover {
    border-color: var(--error);
    color: var(--error);
  }

  .member-details {
    background: var(--bg-tertiary);
    border-radius: 10px;
    padding: 16px;
  }

  .member-details h3 {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 12px;
    color: var(--text-secondary);
  }

  .detail-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }

  .detail-row:last-child {
    border-bottom: none;
  }

  .detail-label {
    font-size: 12px;
    color: var(--text-muted);
  }

  .detail-value {
    font-size: 12px;
    font-weight: 500;
    text-align: right;
    max-width: 60%;
  }

  .detail-value.task {
    font-size: 11px;
    color: var(--text-secondary);
  }

  .help-content {
    color: var(--text-secondary);
    font-size: 13px;
    line-height: 1.6;
  }

  .help-content h4 {
    font-size: 13px;
    color: var(--text-primary);
    margin: 16px 0 8px;
  }

  .help-content ul, .help-content ol {
    margin-left: 16px;
  }

  .help-content li {
    margin-bottom: 6px;
  }

  @media (max-width: 900px) {
    .right-panel {
      display: none;
    }
  }
</style>
