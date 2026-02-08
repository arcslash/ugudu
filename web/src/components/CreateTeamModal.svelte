<script lang="ts">
  import { showCreateTeamModal, specs, teams } from '../stores/app';
  import { createTeam, getTeams } from '../lib/api';

  let teamName = '';
  let selectedSpec = '';
  let creating = false;
  let error = '';

  async function handleCreate() {
    if (!teamName.trim() || !selectedSpec) {
      error = 'Please fill in all fields';
      return;
    }

    creating = true;
    error = '';

    try {
      await createTeam(teamName.trim(), selectedSpec);
      const updatedTeams = await getTeams();
      teams.set(updatedTeams);
      close();
    } catch (e: any) {
      error = e.message || 'Failed to create team';
    } finally {
      creating = false;
    }
  }

  function close() {
    teamName = '';
    selectedSpec = '';
    error = '';
    showCreateTeamModal.set(false);
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      close();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      close();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="modal-backdrop" on:click={handleBackdropClick}>
  <div class="modal">
    <div class="modal-header">
      <h2>Create Team</h2>
      <button class="btn-close" on:click={close}>×</button>
    </div>

    <div class="modal-body">
      {#if error}
        <div class="error-message">{error}</div>
      {/if}

      <div class="form-group">
        <label for="team-name">Team Name</label>
        <input
          id="team-name"
          type="text"
          bind:value={teamName}
          placeholder="my-team"
          disabled={creating}
        />
      </div>

      <div class="form-group">
        <label for="team-spec">Team Spec</label>
        <select id="team-spec" bind:value={selectedSpec} disabled={creating}>
          <option value="">Select a spec...</option>
          {#each $specs as spec}
            <option value={spec.name}>{spec.name}</option>
          {/each}
        </select>
      </div>

      {#if $specs.length === 0}
        <div class="help-text">
          No specs available. Create a team spec first.
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      <button class="btn-secondary" on:click={close} disabled={creating}>
        Cancel
      </button>
      <button
        class="btn-primary"
        on:click={handleCreate}
        disabled={creating || !teamName.trim() || !selectedSpec}
      >
        {creating ? 'Creating...' : 'Create Team'}
      </button>
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(4px);
  }

  .modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 16px;
    width: 100%;
    max-width: 440px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid var(--border);
  }

  .modal-header h2 {
    font-size: 18px;
    font-weight: 600;
  }

  .btn-close {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 24px;
    cursor: pointer;
    border-radius: 6px;
    transition: all 0.2s ease;
  }

  .btn-close:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .modal-body {
    padding: 24px;
  }

  .error-message {
    background: rgba(229, 115, 115, 0.1);
    border: 1px solid var(--error);
    color: var(--error);
    padding: 12px;
    border-radius: 8px;
    font-size: 13px;
    margin-bottom: 16px;
  }

  .form-group {
    margin-bottom: 20px;
  }

  .form-group label {
    display: block;
    font-size: 13px;
    font-weight: 500;
    margin-bottom: 8px;
    color: var(--text-secondary);
  }

  .form-group input,
  .form-group select {
    width: 100%;
    padding: 12px 16px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 14px;
    transition: all 0.2s ease;
  }

  .form-group input:focus,
  .form-group select:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-glow);
  }

  .form-group input::placeholder {
    color: var(--text-muted);
  }

  .form-group select {
    cursor: pointer;
  }

  .form-group select option {
    background: var(--bg-secondary);
    color: var(--text-primary);
  }

  .help-text {
    font-size: 12px;
    color: var(--text-muted);
    padding: 12px;
    background: var(--bg-tertiary);
    border-radius: 8px;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 20px 24px;
    border-top: 1px solid var(--border);
  }

  .btn-secondary {
    padding: 10px 20px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-secondary:hover:not(:disabled) {
    background: var(--bg-card);
    border-color: var(--border-light);
  }

  .btn-primary {
    padding: 10px 20px;
    background: var(--gradient-warm);
    border: none;
    border-radius: 8px;
    color: var(--bg-primary);
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-primary:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 20px var(--accent-glow);
  }

  .btn-primary:disabled,
  .btn-secondary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
