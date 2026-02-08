<script lang="ts">
  import { showSpecEditorModal, editingSpec, specs } from '../stores/app';
  import { getSpec, saveSpec, getSpecs, getProviders, getSettings, type SettingsResponse } from '../lib/api';
  import { onMount } from 'svelte';
  import type { Provider } from '../types/api';

  let specName = '';
  let description = '';
  let defaultProvider = 'openrouter';
  let defaultModel = 'anthropic/claude-3.5-sonnet';
  let roles: Array<{
    id: string;
    roleType: string;
    name: string;
    provider?: string;
    model?: string;
    useCustomModel: boolean;
    showAdvanced: boolean;
    persona: string;
  }> = [];
  let clientFacing: string[] = [];
  let saving = false;
  let loading = false;
  let error = '';
  let providers: Provider[] = [];
  let configuredProviders: Set<string> = new Set();
  let prevProvider = '';

  // Role presets with auto-generated personas
  const rolePresets = {
    pm: {
      title: 'Project Manager',
      key: 'pm',
      names: ['Sarah', 'Alex', 'Jordan', 'Taylor', 'Morgan', 'Casey'],
      basePersona: `You are {name}, a friendly PM. Keep client messages SHORT (1-2 sentences max).

WORKFLOW:
1. Get client request
2. DELEGATE TO ba: to break it into smaller tasks
3. For each task, DELEGATE TO engineer: with detailed spec
4. Only ask client when BA/engineer need client-specific info

RESPONSE FORMAT - Pick ONE:
1. DELEGATE TO ba: [request to analyze and break down]
2. DELEGATE TO engineer: [detailed technical spec]
3. DELEGATE TO qa: [test scenarios]
4. Brief friendly message to client (ONLY when necessary)

RULES:
- First delegate to BA for task breakdown on complex requests
- Keep ALL technical details internal - never show to client
- Only message client for: greetings, clarifying questions, final delivery
- Short friendly responses only`,
      traits: [
        'You coordinate between BA and engineer.',
        'You break complex work into smaller tasks.',
        'You shield clients from technical details.',
        'You keep client messages brief.',
        'You delegate before doing.'
      ]
    },
    engineer: {
      title: 'Software Engineer',
      key: 'engineer',
      names: ['Mike', 'Julia', 'Sam', 'Dev', 'Riley', 'Quinn'],
      basePersona: `You are {name}, a skilled Software Engineer. You implement features using your tools.

WORKFLOW:
- Work on ONE task at a time
- If unclear, ASK ba or pm for clarification (not client)
- Use your tools to write actual code
- Report completion to pm when done

ASKING FOR HELP:
ASK pm: [question about priorities or scope]
ASK ba: [question about requirements or specs]

Only ask client through pm if absolutely necessary.`,
      traits: [
        'You work on one task at a time.',
        'You ask BA for requirement clarity.',
        'You write clean, working code.',
        'You use tools to implement features.',
        'You report when tasks complete.'
      ]
    },
    qa: {
      title: 'QA Engineer',
      key: 'qa',
      names: ['Chris', 'Pat', 'Jamie', 'Drew', 'Avery', 'Blake'],
      basePersona: `You are {name}, a QA Engineer. You test features and report issues.

WORKFLOW:
- Review code/features for quality
- Report bugs to engineer
- Verify fixes work correctly
- Ask BA for expected behavior if unclear`,
      traits: [
        'You catch edge cases others miss.',
        'You write clear bug reports.',
        'You verify fixes thoroughly.',
        'You balance speed with quality.',
        'You ask BA when behavior is unclear.'
      ]
    },
    ba: {
      title: 'Business Analyst',
      key: 'ba',
      names: ['Robin', 'Dana', 'Lee', 'Sage', 'River', 'Phoenix'],
      basePersona: `You are {name}, a Business Analyst. You break down requests into clear tasks.

WORKFLOW:
- Receive requests from PM
- Break into smaller, actionable tasks
- Define acceptance criteria for each
- Answer engineer/QA questions about requirements
- Only escalate to PM if client input truly needed`,
      traits: [
        'You break big requests into small tasks.',
        'You write clear acceptance criteria.',
        'You answer technical team questions.',
        'You only escalate when necessary.',
        'You keep requirements organized.'
      ]
    },
    designer: {
      title: 'UI/UX Designer',
      key: 'designer',
      names: ['Sky', 'Kai', 'Ellis', 'Finley', 'Rowan', 'Emery'],
      basePersona: 'You are {name}, a creative UI/UX Designer. You create intuitive and beautiful user experiences.',
      traits: [
        'You advocate for the user in every decision.',
        'You balance aesthetics with functionality.',
        'You love clean, minimal designs.',
        'You prototype quickly to test ideas.',
        'You stay current with design trends.'
      ]
    }
  };

  const modelOptions = [
    { provider: 'openrouter', models: [
      'anthropic/claude-sonnet-4',
      'anthropic/claude-3.5-sonnet',
      'anthropic/claude-3.5-haiku',
      'google/gemini-2.0-flash-001',
      'google/gemini-2.5-pro-preview',
      'google/gemini-2.5-flash-preview',
      'deepseek/deepseek-chat-v3-0324',
      'openai/gpt-4o',
      'openai/gpt-4o-mini',
      'meta-llama/llama-3.3-70b-instruct',
      'mistralai/mistral-large-2411',
    ]},
    { provider: 'anthropic', models: [
      'claude-sonnet-4-20250514',
      'claude-3-5-sonnet-20241022',
      'claude-3-5-haiku-20241022',
      'claude-3-opus-20240229',
    ]},
    { provider: 'openai', models: [
      'gpt-4o',
      'gpt-4o-mini',
      'gpt-4-turbo',
      'o1',
      'o1-mini',
    ]},
    { provider: 'groq', models: [
      'llama-3.3-70b-versatile',
      'llama-3.1-8b-instant',
      'mixtral-8x7b-32768',
      'gemma2-9b-it',
    ]},
    { provider: 'ollama', models: [
      'llama3.2',
      'llama3.1',
      'mistral',
      'codellama',
      'deepseek-coder-v2',
    ]},
  ];

  function getModelsForProvider(provider: string): string[] {
    const found = modelOptions.find(p => p.provider === provider);
    return found?.models || [];
  }

  function generateUniqueId(): string {
    return Math.random().toString(36).substring(2, 8);
  }

  function pickRandom<T>(arr: T[]): T {
    return arr[Math.floor(Math.random() * arr.length)];
  }

  function generatePersona(roleType: string, name: string): string {
    const preset = rolePresets[roleType as keyof typeof rolePresets];
    if (!preset) return '';

    const base = preset.basePersona.replace('{name}', name);
    const trait = pickRandom(preset.traits);
    return `${base}\n\n${trait}`;
  }

  function getDefaultName(roleType: string): string {
    const preset = rolePresets[roleType as keyof typeof rolePresets];
    if (!preset) return '';
    return pickRandom(preset.names);
  }

  // Reset model when provider changes
  $: if (defaultProvider && defaultProvider !== prevProvider) {
    const models = getModelsForProvider(defaultProvider);
    if (models.length > 0 && !models.includes(defaultModel)) {
      defaultModel = models[0];
    }
    prevProvider = defaultProvider;
  }

  onMount(async () => {
    try {
      providers = await getProviders();
    } catch (e) {
      console.error('Failed to load providers:', e);
    }

    // Fetch settings to know which providers are configured
    try {
      const settings = await getSettings();
      if (settings.providers) {
        if (settings.providers.anthropic?.configured) configuredProviders.add('anthropic');
        if (settings.providers.openai?.configured) configuredProviders.add('openai');
        if (settings.providers.openrouter?.configured) configuredProviders.add('openrouter');
        if (settings.providers.groq?.configured) configuredProviders.add('groq');
        if (settings.providers.ollama?.configured) configuredProviders.add('ollama');
        configuredProviders = configuredProviders; // trigger reactivity
      }
    } catch (e) {
      console.error('Failed to load settings:', e);
    }

    if ($editingSpec) {
      loading = true;
      try {
        const spec = await getSpec($editingSpec);
        specName = spec.name;
        description = spec.description || '';
        defaultProvider = spec.provider || 'openrouter';
        defaultModel = spec.model || 'anthropic/claude-3.5-sonnet';
        // Set prevProvider AFTER loading to prevent reactive model reset
        prevProvider = defaultProvider;
        clientFacing = spec.client_facing || [];

        if (spec.roles) {
          // Only mark as custom model if it differs from the spec default
          roles = Object.entries(spec.roles).map(([key, role]: [string, any]) => {
            const hasCustomProvider = role.provider && role.provider !== defaultProvider;
            const hasCustomModel = role.model && role.model !== defaultModel;
            const useCustom = hasCustomProvider || hasCustomModel;
            return {
              id: generateUniqueId(),
              roleType: key,
              name: role.name || '',
              persona: role.persona || '',
              provider: useCustom ? role.provider : undefined,
              model: useCustom ? role.model : undefined,
              useCustomModel: useCustom,
              showAdvanced: false,
            };
          });
        }
      } catch (e) {
        error = 'Failed to load spec';
      } finally {
        loading = false;
      }
    } else {
      prevProvider = defaultProvider;
      // Default roles for new spec
      const pmName = getDefaultName('pm');
      const engName = getDefaultName('engineer');
      roles = [
        {
          id: generateUniqueId(),
          roleType: 'pm',
          name: pmName,
          persona: generatePersona('pm', pmName),
          useCustomModel: false,
          showAdvanced: false,
        },
        {
          id: generateUniqueId(),
          roleType: 'engineer',
          name: engName,
          persona: generatePersona('engineer', engName),
          useCustomModel: false,
          showAdvanced: false,
        },
      ];
      clientFacing = ['pm'];
    }
  });

  function addRole(roleType: string = 'engineer') {
    const name = getDefaultName(roleType);
    roles = [...roles, {
      id: generateUniqueId(),
      roleType,
      name,
      persona: generatePersona(roleType, name),
      useCustomModel: false,
      showAdvanced: false,
    }];
  }

  function removeRole(id: string) {
    const role = roles.find(r => r.id === id);
    if (role) {
      clientFacing = clientFacing.filter(k => k !== role.roleType);
    }
    roles = roles.filter(r => r.id !== id);
  }

  function updateRoleType(id: string, newType: string) {
    roles = roles.map(r => {
      if (r.id === id) {
        const newName = getDefaultName(newType);
        return {
          ...r,
          roleType: newType,
          name: newName,
          persona: generatePersona(newType, newName),
        };
      }
      return r;
    });
  }

  function updateRoleName(id: string, newName: string) {
    roles = roles.map(r => {
      if (r.id === id) {
        return {
          ...r,
          name: newName,
          persona: generatePersona(r.roleType, newName),
        };
      }
      return r;
    });
  }

  function regeneratePersona(id: string) {
    roles = roles.map(r => {
      if (r.id === id) {
        return {
          ...r,
          persona: generatePersona(r.roleType, r.name),
        };
      }
      return r;
    });
  }

  function toggleClientFacing(roleType: string) {
    if (clientFacing.includes(roleType)) {
      clientFacing = clientFacing.filter(k => k !== roleType);
    } else {
      clientFacing = [...clientFacing, roleType];
    }
  }

  function toggleAdvanced(id: string) {
    roles = roles.map(r => {
      if (r.id === id) {
        return { ...r, showAdvanced: !r.showAdvanced };
      }
      return r;
    });
  }

  function getRoleTitle(roleType: string): string {
    return rolePresets[roleType as keyof typeof rolePresets]?.title || roleType;
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
      // Collect all role types for delegation purposes
      const allRoleTypes = roles.map(r => r.roleType).filter(t => t.trim());

      for (const role of roles) {
        if (!role.roleType.trim()) continue;

        // Set can_delegate based on role type for proper team communication
        let canDelegate: string[] | undefined;
        switch (role.roleType) {
          case 'pm':
            // PM can delegate to all team members
            canDelegate = allRoleTypes.filter(t => t !== 'pm');
            break;
          case 'engineer':
            // Engineer can ask BA and PM for clarification
            canDelegate = allRoleTypes.filter(t => t === 'ba' || t === 'pm');
            break;
          case 'qa':
            // QA can report to engineer and ask BA
            canDelegate = allRoleTypes.filter(t => t === 'engineer' || t === 'ba');
            break;
          case 'ba':
            // BA can escalate to PM
            canDelegate = allRoleTypes.filter(t => t === 'pm');
            break;
          default:
            canDelegate = undefined;
        }

        rolesObj[role.roleType] = {
          title: getRoleTitle(role.roleType),
          ...(role.name && { name: role.name }),
          ...(role.persona && { persona: role.persona }),
          ...(canDelegate && canDelegate.length > 0 && { can_delegate: canDelegate }),
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

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="modal-backdrop" on:click={handleBackdropClick}>
  <div class="modal">
    <div class="modal-header">
      <h2>{$editingSpec ? 'Edit Team Spec' : 'Create Team Spec'}</h2>
      <button class="btn-close" on:click={close}>&times;</button>
    </div>

    <div class="modal-body">
      {#if loading}
        <div class="loading">Loading spec...</div>
      {:else}
        {#if error}
          <div class="error-message">{error}</div>
        {/if}

        <!-- Spec Info -->
        <div class="form-section">
          <div class="form-row two-cols">
            <div class="form-group">
              <label for="spec-name">Spec Name</label>
              <input
                id="spec-name"
                type="text"
                bind:value={specName}
                placeholder="my-dev-team"
                disabled={saving || !!$editingSpec}
              />
            </div>

            <div class="form-group">
              <label for="spec-desc">Description</label>
              <input
                id="spec-desc"
                type="text"
                bind:value={description}
                placeholder="What does this team do?"
                disabled={saving}
              />
            </div>
          </div>
        </div>

        <!-- Default Model -->
        <div class="form-section">
          <label class="section-label">Default AI Model</label>
          <div class="model-selector">
            <select bind:value={defaultProvider} disabled={saving}>
              <option value="openrouter" disabled={!configuredProviders.has('openrouter')}>
                OpenRouter {configuredProviders.has('openrouter') ? '' : '(not configured)'}
              </option>
              <option value="anthropic" disabled={!configuredProviders.has('anthropic')}>
                Anthropic {configuredProviders.has('anthropic') ? '' : '(not configured)'}
              </option>
              <option value="openai" disabled={!configuredProviders.has('openai')}>
                OpenAI {configuredProviders.has('openai') ? '' : '(not configured)'}
              </option>
              <option value="groq" disabled={!configuredProviders.has('groq')}>
                Groq {configuredProviders.has('groq') ? '' : '(not configured)'}
              </option>
              <option value="ollama" disabled={!configuredProviders.has('ollama')}>
                Ollama {configuredProviders.has('ollama') ? '' : '(not configured)'}
              </option>
            </select>
            <select bind:value={defaultModel} disabled={saving}>
              {#each getModelsForProvider(defaultProvider) as model}
                <option value={model}>{model.split('/').pop()}</option>
              {/each}
            </select>
          </div>
        </div>

        <!-- Roles -->
        <div class="form-section">
          <div class="section-header">
            <label class="section-label">Roles</label>
            <div class="add-role-dropdown">
              <button class="btn-add" type="button">+ Add Role</button>
              <div class="dropdown-menu">
                {#each Object.entries(rolePresets) as [key, preset]}
                  <button type="button" on:click={() => addRole(key)}>
                    {preset.title}
                  </button>
                {/each}
              </div>
            </div>
          </div>

          <div class="roles-grid">
            {#each roles as role (role.id)}
              <div class="role-card" class:client-facing={clientFacing.includes(role.roleType)}>
                <div class="role-top">
                  <select
                    class="role-type-select"
                    value={role.roleType}
                    on:change={(e) => updateRoleType(role.id, e.currentTarget.value)}
                    disabled={saving}
                  >
                    {#each Object.entries(rolePresets) as [key, preset]}
                      <option value={key}>{preset.title}</option>
                    {/each}
                  </select>

                  <button class="btn-remove" on:click={() => removeRole(role.id)} disabled={saving} title="Remove">
                    &times;
                  </button>
                </div>

                <div class="role-name-row">
                  <input
                    type="text"
                    value={role.name}
                    on:input={(e) => updateRoleName(role.id, e.currentTarget.value)}
                    placeholder="Agent name"
                    disabled={saving}
                  />
                  <button
                    class="btn-dice"
                    on:click={() => {
                      updateRoleName(role.id, getDefaultName(role.roleType));
                    }}
                    title="Random name"
                    disabled={saving}
                  >
                    🎲
                  </button>
                </div>

                <div class="role-options">
                  <label class="toggle-option">
                    <input
                      type="checkbox"
                      checked={clientFacing.includes(role.roleType)}
                      on:change={() => toggleClientFacing(role.roleType)}
                      disabled={saving}
                    />
                    <span>Client Facing</span>
                  </label>

                  <button
                    class="btn-advanced"
                    on:click={() => toggleAdvanced(role.id)}
                    type="button"
                  >
                    {role.showAdvanced ? 'Hide' : 'Advanced'} ▾
                  </button>
                </div>

                {#if role.showAdvanced}
                  <div class="advanced-section">
                    <label class="toggle-option">
                      <input
                        type="checkbox"
                        bind:checked={role.useCustomModel}
                        disabled={saving}
                      />
                      <span>Custom model</span>
                    </label>

                    {#if role.useCustomModel}
                      <div class="custom-model">
                        <select bind:value={role.provider} disabled={saving}>
                          <option value="">Default</option>
                          <option value="openrouter" disabled={!configuredProviders.has('openrouter')}>
                            OpenRouter {configuredProviders.has('openrouter') ? '' : '(not configured)'}
                          </option>
                          <option value="anthropic" disabled={!configuredProviders.has('anthropic')}>
                            Anthropic {configuredProviders.has('anthropic') ? '' : '(not configured)'}
                          </option>
                          <option value="openai" disabled={!configuredProviders.has('openai')}>
                            OpenAI {configuredProviders.has('openai') ? '' : '(not configured)'}
                          </option>
                          <option value="groq" disabled={!configuredProviders.has('groq')}>
                            Groq {configuredProviders.has('groq') ? '' : '(not configured)'}
                          </option>
                          <option value="ollama" disabled={!configuredProviders.has('ollama')}>
                            Ollama {configuredProviders.has('ollama') ? '' : '(not configured)'}
                          </option>
                        </select>
                        <select bind:value={role.model} disabled={saving}>
                          <option value="">Default</option>
                          {#each getModelsForProvider(role.provider || defaultProvider) as model}
                            <option value={model}>{model.split('/').pop()}</option>
                          {/each}
                        </select>
                      </div>
                    {/if}

                    <div class="persona-section">
                      <div class="persona-header">
                        <span>Persona</span>
                        <button
                          class="btn-regenerate"
                          on:click={() => regeneratePersona(role.id)}
                          title="Regenerate persona"
                          type="button"
                        >
                          🔄 Regenerate
                        </button>
                      </div>
                      <textarea
                        bind:value={role.persona}
                        rows="4"
                        disabled={saving}
                      ></textarea>
                    </div>
                  </div>
                {/if}
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
        {saving ? 'Saving...' : ($editingSpec ? 'Update Spec' : 'Create Spec')}
      </button>
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(8px);
    padding: 20px;
  }

  .modal {
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 20px;
    width: 100%;
    max-width: 640px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 25px 80px rgba(0, 0, 0, 0.6);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 24px 28px;
    border-bottom: 1px solid var(--border);
  }

  .modal-header h2 {
    font-size: 20px;
    font-weight: 600;
    background: var(--gradient-warm);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .btn-close {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 20px;
    cursor: pointer;
    border-radius: 10px;
    transition: all 0.2s;
  }

  .btn-close:hover {
    background: var(--bg-card);
    color: var(--text-primary);
    border-color: var(--border-light);
  }

  .modal-body {
    padding: 24px 28px;
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
    padding: 12px 16px;
    border-radius: 10px;
    font-size: 13px;
    margin-bottom: 20px;
  }

  .form-section {
    margin-bottom: 24px;
  }

  .section-label {
    display: block;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 10px;
    color: var(--text-muted);
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .form-row.two-cols {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .form-group label {
    display: block;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 8px;
    color: var(--text-muted);
  }

  .form-group input {
    width: 100%;
    padding: 12px 16px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    color: var(--text-primary);
    font-size: 14px;
    transition: all 0.2s;
  }

  .form-group input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-glow);
  }

  .model-selector {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 12px;
  }

  .model-selector select {
    padding: 12px 16px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    color: var(--text-primary);
    font-size: 14px;
    cursor: pointer;
  }

  .model-selector select:focus {
    outline: none;
    border-color: var(--accent);
  }

  .add-role-dropdown {
    position: relative;
  }

  .btn-add {
    padding: 8px 16px;
    background: var(--accent);
    border: none;
    border-radius: 8px;
    color: var(--bg-primary);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-add:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px var(--accent-glow);
  }

  .dropdown-menu {
    display: none;
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 4px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 6px;
    min-width: 160px;
    z-index: 10;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
  }

  .add-role-dropdown:hover .dropdown-menu {
    display: block;
  }

  .dropdown-menu button {
    display: block;
    width: 100%;
    padding: 10px 14px;
    background: none;
    border: none;
    color: var(--text-primary);
    font-size: 13px;
    text-align: left;
    cursor: pointer;
    border-radius: 6px;
    transition: background 0.15s;
  }

  .dropdown-menu button:hover {
    background: var(--bg-tertiary);
  }

  .roles-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .role-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 16px;
    transition: all 0.2s;
  }

  .role-card.client-facing {
    border-left: 3px solid var(--accent);
  }

  .role-card:hover {
    border-color: var(--border-light);
  }

  .role-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .role-type-select {
    padding: 8px 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
  }

  .btn-remove {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: 1px solid transparent;
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 18px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .btn-remove:hover {
    border-color: var(--error);
    color: var(--error);
    background: rgba(229, 115, 115, 0.1);
  }

  .role-name-row {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
  }

  .role-name-row input {
    flex: 1;
    padding: 10px 14px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 14px;
  }

  .role-name-row input:focus {
    outline: none;
    border-color: var(--accent);
  }

  .btn-dice {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 18px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .btn-dice:hover {
    background: var(--bg-card);
    border-color: var(--border-light);
    transform: rotate(15deg);
  }

  .role-options {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .toggle-option {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text-secondary);
    cursor: pointer;
  }

  .toggle-option input[type="checkbox"] {
    width: 16px;
    height: 16px;
    accent-color: var(--accent);
    cursor: pointer;
  }

  .btn-advanced {
    padding: 6px 12px;
    background: none;
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .btn-advanced:hover {
    background: var(--bg-tertiary);
    color: var(--text-secondary);
  }

  .advanced-section {
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .custom-model {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .custom-model select {
    padding: 8px 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text-primary);
    font-size: 12px;
  }

  .persona-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .persona-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
    color: var(--text-muted);
  }

  .btn-regenerate {
    padding: 4px 10px;
    background: none;
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 11px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .btn-regenerate:hover {
    background: var(--bg-tertiary);
    color: var(--text-secondary);
  }

  .persona-section textarea {
    padding: 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 12px;
    line-height: 1.5;
    resize: vertical;
    min-height: 80px;
  }

  .persona-section textarea:focus {
    outline: none;
    border-color: var(--accent);
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 20px 28px;
    border-top: 1px solid var(--border);
  }

  .btn-secondary {
    padding: 12px 24px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 10px;
    color: var(--text-primary);
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-secondary:hover:not(:disabled) {
    background: var(--bg-card);
    border-color: var(--border-light);
  }

  .btn-primary {
    padding: 12px 28px;
    background: var(--gradient-warm);
    border: none;
    border-radius: 10px;
    color: var(--bg-primary);
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-primary:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 6px 24px var(--accent-glow);
  }

  .btn-primary:disabled,
  .btn-secondary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
