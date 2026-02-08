<script lang="ts">
  import { currentView, currentTeamName, currentTeam, selectedMember, teamMembers, chatHistory, getChatKey, addMessage, specs, editingSpec, showSpecEditorModal } from '../stores/app';
  import { sendMessage, deleteSpec } from '../lib/api';
  import type { ChatMessage } from '../types/api';

  let messageInput = '';
  let sending = false;

  $: chatKey = $currentTeamName && $selectedMember ? getChatKey($currentTeamName, $selectedMember.id) : null;
  $: messages = chatKey ? ($chatHistory[chatKey] || []) : [];
  $: canChat = $selectedMember?.client_facing === true;

  async function handleSend() {
    if (!messageInput.trim() || !$currentTeamName || !$selectedMember || sending) return;

    const content = messageInput.trim();
    messageInput = '';
    sending = true;

    // Add user message
    addMessage($currentTeamName, $selectedMember.id, {
      from: 'You',
      content,
      type: 'user'
    });

    try {
      const response = await sendMessage($currentTeamName, content, $selectedMember.id);

      if (response.responses) {
        for (const resp of response.responses) {
          addMessage($currentTeamName, $selectedMember.id, {
            from: resp.from,
            content: resp.content,
            type: 'agent',
            role: resp.role
          });
        }
      } else if (response.response) {
        addMessage($currentTeamName, $selectedMember.id, {
          from: $selectedMember.name,
          content: response.response,
          type: 'agent',
          role: $selectedMember.role
        });
      }
    } catch (e: any) {
      addMessage($currentTeamName, $selectedMember.id, {
        from: 'System',
        content: `Error: ${e.message}`,
        type: 'agent'
      });
    } finally {
      sending = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  function openSpecEditor(specName: string | null = null) {
    editingSpec.set(specName);
    showSpecEditorModal.set(true);
  }

  async function handleDeleteSpec(name: string) {
    if (!confirm(`Delete spec "${name}"?`)) return;
    try {
      await deleteSpec(name);
      specs.update(list => list.filter(s => s.name !== name));
    } catch (e) {
      console.error('Failed to delete spec:', e);
    }
  }
</script>

<main class="main-content">
  {#if $currentView === 'dashboard'}
    <div class="dashboard">
      <h1>Welcome to Ugudu</h1>
      <p class="subtitle">AI Team Orchestration Platform</p>

      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-value">{$specs.length}</div>
          <div class="stat-label">Team Specs</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{$teamMembers.length}</div>
          <div class="stat-label">Active Agents</div>
        </div>
      </div>

      <div class="quick-start">
        <h2>Quick Start</h2>
        <p>Select a team from the sidebar or create a new one to get started.</p>
      </div>
    </div>

  {:else if $currentView === 'specs'}
    <div class="specs-view">
      <div class="specs-header">
        <h1>Team Specs</h1>
        <button class="btn-primary" on:click={() => openSpecEditor(null)}>
          + New Spec
        </button>
      </div>

      <div class="specs-grid">
        {#each $specs as spec}
          <div class="spec-card">
            <div class="spec-header">
              <h3>{spec.name}</h3>
              <div class="spec-actions">
                <button class="btn-icon" on:click={() => openSpecEditor(spec.name)} title="Edit">
                  ✏️
                </button>
                <button class="btn-icon danger" on:click={() => handleDeleteSpec(spec.name)} title="Delete">
                  🗑️
                </button>
              </div>
            </div>
            {#if spec.description}
              <p class="spec-description">{spec.description}</p>
            {/if}
            <div class="spec-meta">
              <span>{spec.members || Object.keys(spec.roles || {}).length} roles</span>
              {#if spec.provider}
                <span>{spec.provider}</span>
              {/if}
            </div>
          </div>
        {:else}
          <div class="empty-state">
            <p>No specs yet. Create your first team spec!</p>
          </div>
        {/each}
      </div>
    </div>

  {:else if $currentView === 'providers'}
    <div class="providers-view">
      <h1>LLM Providers</h1>
      <p class="subtitle">Configure your AI providers</p>

      <div class="provider-card">
        <div class="provider-icon">🤖</div>
        <div class="provider-info">
          <h3>OpenRouter</h3>
          <p>Access multiple AI models through a single API</p>
        </div>
        <div class="provider-status configured">Configured</div>
      </div>
    </div>

  {:else if $currentView === 'team' && $currentTeam}
    <div class="team-view">
      <div class="team-header">
        <div class="team-info">
          <h1>{$currentTeam.name}</h1>
          <span class="team-status-badge" class:running={$currentTeam.status === 'running'}>
            {$currentTeam.status}
          </span>
        </div>
      </div>

      {#if $selectedMember}
        <div class="chat-container">
          <div class="chat-header">
            <div class="chat-with">
              <span class="member-name">{$selectedMember.name}</span>
              <span class="member-title">{$selectedMember.title}</span>
            </div>
            {#if !canChat}
              <span class="activity-badge">Activity Log</span>
            {/if}
          </div>

          <div class="messages">
            {#each messages as msg}
              <div class="message" class:user={msg.type === 'user'} class:agent={msg.type === 'agent'}>
                <div class="message-header">
                  <span class="message-from">{msg.from}</span>
                  {#if msg.timestamp}
                    <span class="message-time">
                      {new Date(msg.timestamp).toLocaleTimeString()}
                    </span>
                  {/if}
                </div>
                <div class="message-content">{msg.content}</div>
              </div>
            {:else}
              <div class="empty-chat">
                {#if canChat}
                  <p>Start a conversation with {$selectedMember.name}</p>
                {:else}
                  <p>Activity log for {$selectedMember.name} will appear here</p>
                {/if}
              </div>
            {/each}
          </div>

          {#if canChat}
            <div class="chat-input-container">
              <textarea
                class="chat-input"
                bind:value={messageInput}
                on:keydown={handleKeydown}
                placeholder="Type your message..."
                disabled={sending}
              ></textarea>
              <button
                class="btn-send"
                on:click={handleSend}
                disabled={sending || !messageInput.trim()}
              >
                {sending ? '...' : 'Send'}
              </button>
            </div>
          {:else}
            <div class="readonly-notice">
              This agent is not client-facing. Chat with the PM to delegate tasks.
            </div>
          {/if}
        </div>
      {:else}
        <div class="no-member-selected">
          <p>Select a team member to view their chat or activity</p>
        </div>
      {/if}
    </div>
  {/if}
</main>

<style>
  .main-content {
    background: var(--bg-primary);
    padding: 32px;
    overflow-y: auto;
    height: 100vh;
  }

  h1 {
    font-size: 28px;
    font-weight: 700;
    margin-bottom: 8px;
  }

  .subtitle {
    color: var(--text-secondary);
    font-size: 14px;
    margin-bottom: 32px;
  }

  /* Dashboard */
  .dashboard {
    max-width: 800px;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 16px;
    margin-bottom: 32px;
  }

  .stat-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 24px;
    text-align: center;
  }

  .stat-value {
    font-size: 36px;
    font-weight: 700;
    color: var(--accent);
  }

  .stat-label {
    color: var(--text-secondary);
    font-size: 13px;
    margin-top: 4px;
  }

  .quick-start {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 24px;
  }

  .quick-start h2 {
    font-size: 18px;
    margin-bottom: 8px;
  }

  .quick-start p {
    color: var(--text-secondary);
  }

  /* Specs View */
  .specs-view {
    max-width: 1000px;
  }

  .specs-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
  }

  .specs-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }

  .spec-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
    transition: all 0.2s ease;
  }

  .spec-card:hover {
    border-color: var(--accent);
    box-shadow: 0 0 20px var(--accent-glow);
  }

  .spec-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 8px;
  }

  .spec-header h3 {
    font-size: 16px;
    font-weight: 600;
  }

  .spec-actions {
    display: flex;
    gap: 4px;
  }

  .spec-description {
    color: var(--text-secondary);
    font-size: 13px;
    margin-bottom: 12px;
  }

  .spec-meta {
    display: flex;
    gap: 12px;
    font-size: 12px;
    color: var(--text-muted);
  }

  /* Providers View */
  .providers-view {
    max-width: 600px;
  }

  .provider-card {
    display: flex;
    align-items: center;
    gap: 16px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
  }

  .provider-icon {
    font-size: 32px;
  }

  .provider-info {
    flex: 1;
  }

  .provider-info h3 {
    font-size: 16px;
    margin-bottom: 4px;
  }

  .provider-info p {
    font-size: 13px;
    color: var(--text-secondary);
  }

  .provider-status {
    padding: 6px 12px;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 500;
  }

  .provider-status.configured {
    background: rgba(124, 179, 66, 0.2);
    color: var(--success);
  }

  /* Team View */
  .team-view {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .team-header {
    margin-bottom: 24px;
  }

  .team-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .team-status-badge {
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 12px;
    background: var(--text-muted);
    color: var(--bg-primary);
  }

  .team-status-badge.running {
    background: var(--success);
  }

  /* Chat */
  .chat-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
  }

  .chat-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
  }

  .chat-with {
    display: flex;
    flex-direction: column;
  }

  .member-name {
    font-weight: 600;
    font-size: 15px;
  }

  .member-title {
    font-size: 12px;
    color: var(--text-muted);
  }

  .activity-badge {
    padding: 4px 10px;
    background: var(--bg-tertiary);
    border-radius: 12px;
    font-size: 11px;
    color: var(--text-secondary);
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .message {
    max-width: 80%;
    padding: 12px 16px;
    border-radius: 12px;
  }

  .message.user {
    align-self: flex-end;
    background: var(--accent);
    color: var(--bg-primary);
  }

  .message.agent {
    align-self: flex-start;
    background: var(--bg-card);
    border: 1px solid var(--border);
  }

  .message-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
    font-size: 12px;
  }

  .message-from {
    font-weight: 600;
  }

  .message.user .message-from {
    color: var(--bg-secondary);
  }

  .message.agent .message-from {
    color: var(--accent);
  }

  .message-time {
    opacity: 0.7;
    font-size: 11px;
  }

  .message-content {
    font-size: 14px;
    line-height: 1.5;
    white-space: pre-wrap;
  }

  .empty-chat {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
  }

  .chat-input-container {
    display: flex;
    gap: 12px;
    padding: 16px 20px;
    border-top: 1px solid var(--border);
  }

  .chat-input {
    flex: 1;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 16px;
    color: var(--text-primary);
    font-size: 14px;
    resize: none;
    min-height: 44px;
    max-height: 120px;
  }

  .chat-input:focus {
    outline: none;
    border-color: var(--accent);
  }

  .btn-send {
    padding: 12px 24px;
    background: var(--gradient-warm);
    border: none;
    border-radius: 8px;
    color: var(--bg-primary);
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-send:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 20px var(--accent-glow);
  }

  .btn-send:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .readonly-notice {
    padding: 16px 20px;
    background: var(--bg-tertiary);
    border-top: 1px solid var(--border);
    color: var(--text-muted);
    font-size: 13px;
    text-align: center;
  }

  .no-member-selected {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
  }

  /* Buttons */
  .btn-primary {
    padding: 10px 20px;
    background: var(--gradient-warm);
    border: none;
    border-radius: 8px;
    color: var(--bg-primary);
    font-weight: 600;
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-primary:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 20px var(--accent-glow);
  }

  .btn-icon {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-icon:hover {
    background: var(--bg-tertiary);
    border-color: var(--border-light);
  }

  .btn-icon.danger:hover {
    border-color: var(--error);
    background: rgba(229, 115, 115, 0.1);
  }

  .empty-state {
    grid-column: 1 / -1;
    text-align: center;
    padding: 40px;
    color: var(--text-muted);
  }
</style>
