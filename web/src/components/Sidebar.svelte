<script lang="ts">
  import { teams, currentTeamName, currentView, showCreateTeamModal, specs } from '../stores/app';
  import { getTeams, getSpecs, startTeam, getTeamMembers } from '../lib/api';
  import { teamMembers, selectedMember, currentTeam } from '../stores/app';

  async function selectTeam(name: string) {
    currentTeamName.set(name);
    currentView.set('team');

    try {
      // Start team if not running
      await startTeam(name);

      // Load members
      const data = await getTeamMembers(name);
      teamMembers.set(data.members || []);

      // Find the team in our list
      const team = $teams.find(t => t.name === name);
      if (team) {
        currentTeam.set(team);
      }

      // Select PM by default
      const pm = data.members?.find((m: any) => m.client_facing);
      if (pm) {
        selectedMember.set(pm);
      }
    } catch (e) {
      console.error('Failed to select team:', e);
    }
  }

  function goToDashboard() {
    currentView.set('dashboard');
    currentTeamName.set(null);
    selectedMember.set(null);
  }

  function goToSpecs() {
    currentView.set('specs');
    currentTeamName.set(null);
  }

  function goToProviders() {
    currentView.set('providers');
    currentTeamName.set(null);
  }
</script>

<aside class="sidebar">
  <div class="logo-section">
    <img src="/api/static/ugudu_orc.png" alt="Ugudu" class="logo-img" on:error={(e) => e.currentTarget.style.display = 'none'} />
    <div class="logo">Ugudu</div>
  </div>
  <div class="tagline">AI Team Orchestration</div>

  <nav class="nav-section">
    <div class="nav-title">Menu</div>
    <button
      class="nav-item"
      class:active={$currentView === 'dashboard'}
      on:click={goToDashboard}
    >
      <span class="nav-icon">🏠</span>
      <span>Dashboard</span>
    </button>
    <button
      class="nav-item"
      class:active={$currentView === 'specs'}
      on:click={goToSpecs}
    >
      <span class="nav-icon">📋</span>
      <span>Team Specs</span>
    </button>
    <button
      class="nav-item"
      class:active={$currentView === 'providers'}
      on:click={goToProviders}
    >
      <span class="nav-icon">⚡</span>
      <span>Providers</span>
    </button>
  </nav>

  <div class="teams-section">
    <div class="nav-title">Teams</div>
    <div class="team-list">
      {#each $teams as team}
        <button
          class="team-item"
          class:active={$currentTeamName === team.name}
          on:click={() => selectTeam(team.name)}
        >
          <span class="team-name">{team.name}</span>
          <div class="team-status" class:running={team.status === 'running'}></div>
        </button>
      {/each}
      {#if $teams.length === 0}
        <div class="empty-teams">No teams yet. Create one!</div>
      {/if}
    </div>
    <button class="btn-create-team" on:click={() => showCreateTeamModal.set(true)}>
      + Create Team
    </button>
  </div>
</aside>

<style>
  .sidebar {
    background: var(--bg-secondary);
    border-right: 1px solid var(--border);
    padding: 24px;
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
  }

  .logo-section {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
  }

  .logo-img {
    width: 48px;
    height: 48px;
    border-radius: 12px;
    object-fit: cover;
  }

  .logo {
    font-size: 26px;
    font-weight: 700;
    background: var(--gradient-warm);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .tagline {
    font-size: 12px;
    color: var(--text-muted);
    margin-bottom: 32px;
    padding-left: 60px;
  }

  .nav-section {
    margin-bottom: 28px;
  }

  .nav-title {
    font-size: 10px;
    text-transform: uppercase;
    color: var(--text-muted);
    margin-bottom: 12px;
    letter-spacing: 1px;
    font-weight: 600;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 10px;
    cursor: pointer;
    color: var(--text-secondary);
    transition: all 0.2s ease;
    margin-bottom: 4px;
    font-size: 14px;
    background: none;
    border: none;
    width: 100%;
    text-align: left;
  }

  .nav-item:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .nav-item.active {
    background: var(--accent);
    color: var(--bg-primary);
    font-weight: 500;
  }

  .nav-icon {
    font-size: 18px;
    width: 24px;
    text-align: center;
  }

  .teams-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .team-list {
    flex: 1;
    overflow-y: auto;
  }

  .team-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    border-radius: 10px;
    cursor: pointer;
    margin-bottom: 4px;
    transition: all 0.2s ease;
    border: 1px solid transparent;
    background: none;
    color: var(--text-primary);
    width: 100%;
    text-align: left;
  }

  .team-item:hover {
    background: var(--bg-tertiary);
    border-color: var(--border-light);
  }

  .team-item.active {
    background: var(--bg-card);
    border-color: var(--accent);
    box-shadow: 0 0 20px var(--accent-glow);
  }

  .team-name {
    font-weight: 500;
    font-size: 14px;
  }

  .team-status {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--text-muted);
    transition: all 0.3s ease;
  }

  .team-status.running {
    background: var(--success);
    box-shadow: 0 0 10px rgba(124, 179, 66, 0.5);
    animation: pulse 2s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.6; }
  }

  .empty-teams {
    color: var(--text-muted);
    font-size: 13px;
    padding: 12px;
  }

  .btn-create-team {
    width: 100%;
    margin-top: 16px;
    padding: 12px;
    background: var(--gradient-warm);
    border: none;
    border-radius: 10px;
    color: var(--bg-primary);
    font-weight: 600;
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-create-team:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 20px var(--accent-glow);
  }
</style>
