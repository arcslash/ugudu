<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import MainContent from './components/MainContent.svelte';
  import RightPanel from './components/RightPanel.svelte';
  import CreateTeamModal from './components/CreateTeamModal.svelte';
  import SpecEditorModal from './components/SpecEditorModal.svelte';
  import { teams, specs, loadChatHistory, showCreateTeamModal, showSpecEditorModal } from './stores/app';
  import { getTeams, getSpecs } from './lib/api';

  onMount(async () => {
    loadChatHistory();

    try {
      const [teamsData, specsData] = await Promise.all([
        getTeams(),
        getSpecs()
      ]);
      teams.set(teamsData);
      specs.set(specsData);
    } catch (e) {
      console.error('Failed to load initial data:', e);
    }
  });
</script>

<div class="app">
  <Sidebar />
  <MainContent />
  <RightPanel />
</div>

{#if $showCreateTeamModal}
  <CreateTeamModal />
{/if}

{#if $showSpecEditorModal}
  <SpecEditorModal />
{/if}

<style>
  :global(:root) {
    --bg-primary: #1a1613;
    --bg-secondary: #231f1b;
    --bg-tertiary: #2d2722;
    --bg-card: #352f29;
    --text-primary: #f5e6d3;
    --text-secondary: #a89888;
    --text-muted: #6d5f52;
    --accent: #c9a66b;
    --accent-hover: #dbb87d;
    --accent-glow: rgba(201, 166, 107, 0.3);
    --success: #7cb342;
    --warning: #f9a825;
    --error: #e57373;
    --border: #3d352d;
    --border-light: #4a413a;
    --gradient-warm: linear-gradient(135deg, #c9a66b 0%, #8b6914 100%);
  }

  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: var(--bg-primary);
    color: var(--text-primary);
    min-height: 100vh;
  }

  :global(button) {
    font-family: inherit;
  }

  :global(::-webkit-scrollbar) {
    width: 8px;
  }

  :global(::-webkit-scrollbar-track) {
    background: var(--bg-primary);
  }

  :global(::-webkit-scrollbar-thumb) {
    background: var(--border);
    border-radius: 4px;
  }

  .app {
    display: grid;
    grid-template-columns: 280px 1fr 380px;
    min-height: 100vh;
  }

  @media (max-width: 1200px) {
    .app {
      grid-template-columns: 260px 1fr 320px;
    }
  }

  @media (max-width: 900px) {
    .app {
      grid-template-columns: 1fr;
    }
  }
</style>
