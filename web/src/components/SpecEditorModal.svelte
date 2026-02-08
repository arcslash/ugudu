<script lang="ts">
  import { showSpecEditorModal, editingSpec, specs } from '../stores/app';
  import { getSpec, saveSpec, getSpecs, getProviders } from '../lib/api';
  import { onMount } from 'svelte';
  import type { Provider } from '../types/api';

  let specName = '';
  let description = '';
  let defaultProvider = 'openrouter';
  let defaultModel = 'anthropic/claude-3.5-sonnet';
  let roles: Array<{
    key: string;
    title: string;
    name: string;
    persona: string;
    provider?: string;
    model?: string;
    useCustomModel: boolean;
  }> = [];
  let clientFacing: string[] = [];
  let saving = false;
  let loading = false;
  let error = '';
  let providers: Provider[] = [];

  const modelOptions = [
    { provider: 'openrouter', models: [
      'anthropic/claude-3.5-sonnet',
      'anthropic/claude-3-opus',
      'google/gemini-pro-1.5',
      'google/gemini-flash-1.5',
      'deepseek/deepseek-chat',
      'meta-llama/llama-3.1-70b-instruct',
      'openai/gpt-4-turbo',
      'openai/gpt-4o',
    ]},
    { provider: 'anthropic', models: [
      'claude-3-5-sonnet-20241022',
      'claude-3-opus-20240229',
      'claude-3-haiku-20240307',
    ]},
    { provider: 'openai', models: [
      'gpt-4-turbo',
      'gpt-4o',
      'gpt-4o-mini',
    ]},
  ];

  function getModelsForProvider(provider: string): string[] {
    const found = modelOptions.find(p => p.provider === provider);
    return found?.models || [];
  }

  onMount(async () => {
    try {
      providers = await getProviders();
    } catch (e) {
      console.error('Failed to load providers:', e);
    }

    if ($editingSpec) {
      loading = true;
      try {
        const spec = await getSpec($editingSpec);
        specName = spec.name;
        description = spec.description || '';
        defaultProvider = spec.provider || 'openrouter';
        defaultModel = spec.model || 'anthropic/claude-3.5-sonnet';
        clientFacing = spec.client_facing || [];

        if (spec.roles) {
          roles = Object.entries(spec.roles).map(([key, role]: [string, any]) => ({
            key,
            title: role.title || key,
            name: role.name || '',
            persona: role.persona || '',
            provider: role.provider,
            model: role.model,
            useCustomModel: !!(role.provider || role.model),
          }));
        }
      } catch (e) {
        error = 'Failed to load spec';
      } finally {
        loading = false;
      }
    } else {
      // Default roles for new spec
      roles = [
        { key: 'pm', title: 'Project Manager', name: '', persona: '', useCustomModel: false },
        { key: 'engineer', title: 'Software Engineer', name: '', persona: '', useCustomModel: false },
      ];
      clientFacing = ['pm'];
    }
  });

  function addRole() {
    roles = [...roles, {
      key: '',
      title: '',
      name: '',
      persona: '',
      useCustomModel: false
    }];
  }

  function removeRole(index: number) {
    const roleKey = roles[index].key;
    roles = roles.filter((_, i) => i !== index);
    clientFacing = clientFacing.filter(k => k !== roleKey);
  }

  function toggleClientFacing(roleKey: string) {
    if (clientFacing.includes(roleKey)) {
      clientFacing = clientFacing.filter(k => k !== roleKey);
    } else {
      clientFacing = [...clientFacing, roleKey];
    }
  }

  async function handleSave() {
    if (!specName.trim()) {
      error = 'Spec name is required';
      return;
    }

    if (roles.length === 0) {
      error = 'At least one role is required';
      return;
    }

    saving = true;
    error = '';

    try {
      const rolesObj: Record<string, any> = {};
      for (const role of roles) {
        if (!role.key.trim()) continue;
        rolesObj[role.key] = {
          title: role.title || role.key,
          ...(role.name && { name: role.name }),
          ...(role.persona && { persona: role.persona }),
          ...(role.useCustomModel && role.provider && { provider: role.provider }),
          ...(role.useCustomModel && role.model && { model: role.model }),
        };
      }

      await saveSpec({
        name: specName.trim(),
        description: description.trim() || undefined,
        provider: defaultProvider,
        model: defaultModel,
        roles: rolesObj,
        client_facing: clientFacing,
      });

      const updatedSpecs = await getSpecs();
      specs.set(updatedSpecs);
      close();
    } catch (e: any) {
      error = e.message || 'Failed to save spec';
    } finally {
      saving = false;
    }
  }

  function close() {
    editingSpec.set(null);
    showSpecEditorModal.set(false);
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

<div class="modal-backdrop" on:click={handleBackdropClick}>
  <div class="modal">
    <div class="modal-header">
      <h2>{$editingSpec ? 'Edit Spec' : 'Create Spec'}</h2>
      <button class="btn-close" on:click={close}>×</button>
    </div>

    <div class="modal-body">
      {#if loading}
        <div class="loading">Loading spec...</div>
      {:else}
        {#if error}
          <div class="error-message">{error}</div>
        {/if}

        <div class="form-section">
          <h3>Basic Info</h3>

          <div class="form-row">
            <div class="form-group">
              <label for="spec-name">Spec Name</label>
              <input
                id="spec-name"
                type="text"
                bind:value={specName}
                placeholder="dev-team"
                disabled={saving || !!$editingSpec}
              />
            </div>

            <div class="form-group">
              <label for="spec-desc">Description</label>
              <input
                id="spec-desc"
                type="text"
                bind:value={description}
                placeholder="Development team with PM and engineers"
                disabled={saving}
              />
            </div>
          </div>
        </div>

        <div class="form-section">
          <h3>Default Model</h3>
          <p class="section-hint">Default model for all roles. Can be overridden per role.</p>

          <div class="form-row">
            <div class="form-group">
              <label for="default-provider">Provider</label>
              <select id="default-provider" bind:value={defaultProvider} disabled={saving}>
                <option value="openrouter">OpenRouter</option>
                <option value="anthropic">Anthropic</option>
                <option value="openai">OpenAI</option>
              </select>
            </div>

            <div class="form-group">
              <label for="default-model">Model</label>
              <select id="default-model" bind:value={defaultModel} disabled={saving}>
                {#each getModelsForProvider(defaultProvider) as model}
                  <option value={model}>{model}</option>
                {/each}
              </select>
            </div>
          </div>
        </div>

        <div class="form-section">
          <div class="section-header">
            <h3>Roles</h3>
            <button class="btn-add" on:click={addRole} disabled={saving}>+ Add Role</button>
          </div>

          <div class="roles-list">
            {#each roles as role, i}
              <div class="role-card">
                <div class="role-header">
                  <div class="role-main">
                    <input
                      type="text"
                      bind:value={role.key}
                      placeholder="role-key"
                      class="role-key-input"
                      disabled={saving}
                    />
                    <input
                      type="text"
                      bind:value={role.title}
                      placeholder="Role Title"
                      class="role-title-input"
                      disabled={saving}
                    />
                  </div>
                  <div class="role-actions">
                    <label class="checkbox-label" title="Client can chat with this role">
                      <input
                        type="checkbox"
                        checked={clientFacing.includes(role.key)}
                        on:change={() => toggleClientFacing(role.key)}
                        disabled={saving}
                      />
                      <span>Client Facing</span>
                    </label>
                    <button class="btn-remove" on:click={() => removeRole(i)} disabled={saving}>×</button>
                  </div>
                </div>

                <div class="role-details">
                  <input
                    type="text"
                    bind:value={role.name}
                    placeholder="Agent name (optional)"
                    disabled={saving}
                  />

                  <label class="checkbox-label model-toggle">
                    <input
                      type="checkbox"
                      bind:checked={role.useCustomModel}
                      disabled={saving}
                    />
                    <span>Use custom model for this role</span>
                  </label>

                  {#if role.useCustomModel}
                    <div class="custom-model-row">
                      <select bind:value={role.provider} disabled={saving}>
                        <option value="">Same as default</option>
                        <option value="openrouter">OpenRouter</option>
                        <option value="anthropic">Anthropic</option>
                        <option value="openai">OpenAI</option>
                      </select>
                      <select bind:value={role.model} disabled={saving}>
                        <option value="">Same as default</option>
                        {#each getModelsForProvider(role.provider || defaultProvider) as model}
                          <option value={model}>{model}</option>
                        {/each}
                      </select>
                    </div>
                  {/if}

                  <textarea
                    bind:value={role.persona}
                    placeholder="Persona/instructions for this role..."
                    rows="3"
                    disabled={saving}
                  ></textarea>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      <button class="btn-secondary" on:click={close} disabled={saving}>
        Cancel
      </button>
      <button class="btn-primary" on:click={handleSave} disabled={saving || loading}>
        {saving ? 'Saving...' : 'Save Spec'}
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
    padding: 24px;
  }

  .modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 16px;
    width: 100%;
    max-width: 700px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
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
  }

  .btn-close:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex: 1;
  }

  .loading {
    text-align: center;
    color: var(--text-muted);
    padding: 40px;
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

  .form-section {
    margin-bottom: 28px;
  }

  .form-section h3 {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 12px;
    color: var(--text-primary);
  }

  .section-hint {
    font-size: 12px;
    color: var(--text-muted);
    margin: -8px 0 12px;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .section-header h3 {
    margin-bottom: 0;
  }

  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .form-group {
    margin-bottom: 16px;
  }

  .form-group label {
    display: block;
    font-size: 12px;
    font-weight: 500;
    margin-bottom: 6px;
    color: var(--text-secondary);
  }

  .form-group input,
  .form-group select,
  .form-group textarea {
    width: 100%;
    padding: 10px 14px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 14px;
  }

  .form-group input:focus,
  .form-group select:focus,
  .form-group textarea:focus {
    outline: none;
    border-color: var(--accent);
  }

  .form-group select option {
    background: var(--bg-secondary);
  }

  .btn-add {
    padding: 6px 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--accent);
    font-size: 12px;
    cursor: pointer;
  }

  .btn-add:hover {
    background: var(--bg-card);
  }

  .roles-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .role-card {
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 16px;
  }

  .role-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 12px;
    gap: 12px;
  }

  .role-main {
    display: flex;
    gap: 12px;
    flex: 1;
  }

  .role-key-input {
    width: 120px !important;
    font-family: monospace;
    font-size: 13px !important;
  }

  .role-title-input {
    flex: 1;
  }

  .role-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
    white-space: nowrap;
  }

  .checkbox-label input[type="checkbox"] {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
  }

  .model-toggle {
    margin: 8px 0;
  }

  .btn-remove {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-muted);
    font-size: 18px;
    cursor: pointer;
  }

  .btn-remove:hover {
    border-color: var(--error);
    color: var(--error);
  }

  .role-details {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .role-details input,
  .role-details select,
  .role-details textarea {
    width: 100%;
    padding: 10px 14px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 13px;
  }

  .role-details textarea {
    resize: vertical;
    min-height: 60px;
  }

  .custom-model-row {
    display: grid;
    grid-template-columns: 1fr 2fr;
    gap: 8px;
  }

  .custom-model-row select {
    font-size: 12px;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 20px 24px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
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
  }

  .btn-secondary:hover:not(:disabled) {
    background: var(--bg-card);
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
