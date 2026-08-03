<script lang="ts">
  import type { CellDefinition, EdgeDefinition, RuleSet, ActionDefinition, ActionOption } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';

  let { cell, edges, rules, onCellChange, onDeleteCell, onDeleteEdge, selectedEdgeId, onEdgeSelect, onEdgeChange }: {
    cell: CellDefinition | null | undefined;
    edges: EdgeDefinition[];
    rules: RuleSet;
    onCellChange?: (cell: CellDefinition) => void;
    onDeleteCell?: (id: string) => void;
    onDeleteEdge?: (id: string) => void;
    selectedEdgeId?: string;
    onEdgeSelect?: (id: string | undefined) => void;
    onEdgeChange?: (edge: EdgeDefinition) => void;
  } = $props();

  let t = $derived(i18n.t);
  let typeDef = $derived(cell ? rules.cellTypes[cell.type] : null);
  let cellEdges = $derived(edges.filter(e => e.from === cell?.id || e.to === cell?.id));

  // --- Action type definitions ---
  const ACTION_TYPES = [
    'gain_resource',
    'lose_resource',
    'transfer_resource',
    'set_cell_owner',
    'if_resource_ge',
    'finish_game',
    'log_message',
  ];

  const TARGET_OPTIONS = ['current', 'owner', 'bank'];

  function resourceList(): string[] {
    return Object.keys(rules.resources);
  }

  function updateField(key: string, value: any) {
    if (!cell || !onCellChange) return;
    const updated = { ...cell, fields: { ...cell.fields, [key]: value } };
    onCellChange(updated);
  }

  function updateVisual(key: string, value: string) {
    if (!cell || !onCellChange) return;
    const updated = { ...cell, visual: { ...cell.visual, [key]: value } };
    onCellChange(updated);
  }

  // --- Action helpers ---
  function getActions(list: 'onLand' | 'onPass'): ActionDefinition[] {
    if (!cell) return [];
    return cell[list] || [];
  }

  function setActions(list: 'onLand' | 'onPass', actions: ActionDefinition[]) {
    if (!cell || !onCellChange) return;
    if (list === 'onLand') {
      onCellChange({ ...cell, onLand: actions });
    } else {
      onCellChange({ ...cell, onPass: actions });
    }
  }

  function addAction(list: 'onLand' | 'onPass', type: string) {
    const actions = getActions(list);
    const newAction: ActionDefinition = { type };
    // Set defaults based on type
    if (type === 'gain_resource' || type === 'lose_resource') {
      newAction.resource = resourceList()[0] || 'gold';
      newAction.amount = 1;
    } else if (type === 'transfer_resource') {
      newAction.resource = resourceList()[0] || 'gold';
      newAction.amount = 1;
      newAction.target = 'owner';
    } else if (type === 'set_cell_owner') {
      newAction.target = 'current';
    } else if (type === 'if_resource_ge') {
      newAction.resource = resourceList()[0] || 'gold';
      newAction.amount = 1;
      newAction.then = [];
      newAction.else = [];
    } else if (type === 'log_message') {
      newAction.title = 'New message';
    }
    setActions(list, [...actions, newAction]);
  }

  function removeAction(list: 'onLand' | 'onPass', index: number) {
    const actions = getActions(list);
    setActions(list, actions.filter((_, i) => i !== index));
  }

  function moveAction(list: 'onLand' | 'onPass', index: number, dir: -1 | 1) {
    const actions = getActions(list);
    const newIndex = index + dir;
    if (newIndex < 0 || newIndex >= actions.length) return;
    const swapped = [...actions];
    [swapped[index], swapped[newIndex]] = [swapped[newIndex], swapped[index]];
    setActions(list, swapped);
  }

  function updateAction(list: 'onLand' | 'onPass', index: number, updates: Partial<ActionDefinition>) {
    const actions = getActions(list);
    const updated = { ...actions[index], ...updates };
    // Clear irrelevant fields when type changes
    if (updates.type) {
      const cleaned: ActionDefinition = { type: updated.type };
      if (updated.resource !== undefined) cleaned.resource = updated.resource;
      if (updated.amount !== undefined) cleaned.amount = updated.amount;
      if (updated.target !== undefined) cleaned.target = updated.target;
      if (updated.title !== undefined) cleaned.title = updated.title;
      if (updated.then !== undefined) cleaned.then = updated.then;
      if (updated.else !== undefined) cleaned.else = updated.else;
      updated.actionId && (cleaned.actionId = updated.actionId);
      Object.assign(updated, cleaned);
    }
    setActions(list, actions.map((a, i) => i === index ? updated : a));
  }

  function getActionField(action: ActionDefinition, field: string): any {
    return (action as any)[field];
  }

  function setActionField(list: 'onLand' | 'onPass', index: number, field: string, value: any) {
    updateAction(list, index, { [field]: value });
  }

  // --- Edge condition helpers ---
  const conditionTypes = [
    'always',
    'dice_total_even',
    'dice_total_odd',
    'dice_total_in',
    'player_resource_at_least',
    'manual_choice',
    'pay_resource',
  ];

  let selectedEdge = $derived(edges.find(e => e.id === selectedEdgeId) ?? null);

  function updateConditionType(type: string) {
    if (!selectedEdge || !onEdgeChange) return;
    onEdgeChange({
      ...selectedEdge,
      condition: { type, ...(selectedEdge.condition.values ? { values: selectedEdge.condition.values } : {}), ...(selectedEdge.condition.resource ? { resource: selectedEdge.condition.resource } : {}), ...(selectedEdge.condition.amount ? { amount: selectedEdge.condition.amount } : {}), ...(selectedEdge.condition.label ? { label: selectedEdge.condition.label } : {}) },
    });
  }

  function updateConditionField(field: string, value: any) {
    if (!selectedEdge || !onEdgeChange) return;
    const cond = { ...selectedEdge.condition, [field]: value };
    onEdgeChange({ ...selectedEdge, condition: cond });
  }

  function clearConditionFields(...fields: string[]) {
    if (!selectedEdge || !onEdgeChange) return;
    const cond = { ...selectedEdge.condition };
    for (const f of fields) delete cond[f as keyof typeof cond];
    onEdgeChange({ ...selectedEdge, condition: cond });
  }
</script>

<div class="inspector">
  {#if cell}
    <h3>{t('inspector.cell', { id: cell.id })}</h3>
    <label>
      {t('inspector.id')}
      <input value={cell.id} oninput={(e) => onCellChange?.({ ...cell, id: (e.target as HTMLInputElement).value })} />
    </label>
    <label>
      {t('inspector.title')}
      <input value={cell.title} oninput={(e) => onCellChange?.({ ...cell, title: (e.target as HTMLInputElement).value })} />
    </label>
    <label>
      {t('inspector.type')}
      <select
        value={cell.type}
        onchange={(e) => onCellChange?.({ ...cell, type: (e.target as HTMLSelectElement).value, fields: {} })}
      >
        {#each Object.entries(rules.cellTypes) as [key, ct]}
          <option value={key}>{ct.title}</option>
        {/each}
      </select>
    </label>
    <label>
      {t('inspector.posX')}
      <input type="number" value={cell.x} oninput={(e) => onCellChange?.({ ...cell, x: parseInt((e.target as HTMLInputElement).value) || 0 })} />
    </label>
    <label>
      {t('inspector.posY')}
      <input type="number" value={cell.y} oninput={(e) => onCellChange?.({ ...cell, y: parseInt((e.target as HTMLInputElement).value) || 0 })} />
    </label>
    <hr />
    <h4>{t('inspector.visual')}</h4>
    <label>
      {t('inspector.color')}
      <input type="color" value={cell.visual.baseColor} oninput={(e) => updateVisual('baseColor', (e.target as HTMLInputElement).value)} />
    </label>
    <label>
      {t('inspector.imageUrl')}
      <input value={cell.visual.baseImage} oninput={(e) => updateVisual('baseImage', (e.target as HTMLInputElement).value)} placeholder={t('inspector.imageUrlPlaceholder')} />
    </label>

    {#if typeDef && Object.keys(typeDef.fields).length > 0}
      <hr />
      <h4>{t('inspector.fields')}</h4>
      {#each Object.entries(typeDef.fields) as [key, fieldDef]}
        <label>
          {fieldDef.label}
          {#if fieldDef.type === 'string'}
            <input
              value={cell.fields[key] ?? fieldDef.default ?? ''}
              oninput={(e) => updateField(key, (e.target as HTMLInputElement).value)}
            />
          {:else if fieldDef.type === 'number'}
            <input
              type="number"
              value={cell.fields[key] ?? fieldDef.default ?? 0}
              oninput={(e) => updateField(key, parseInt((e.target as HTMLInputElement).value) || 0)}
            />
          {:else if fieldDef.type === 'boolean'}
            <input
              type="checkbox"
              checked={cell.fields[key] ?? fieldDef.default ?? false}
              onchange={(e) => updateField(key, (e.target as HTMLInputElement).checked)}
            />
          {:else if fieldDef.type === 'select' && fieldDef.options}
            <select
              value={cell.fields[key] ?? fieldDef.default ?? ''}
              onchange={(e) => updateField(key, (e.target as HTMLSelectElement).value)}
            >
              {#each fieldDef.options as opt}
                <option value={opt}>{opt}</option>
              {/each}
            </select>
          {/if}
        </label>
      {/each}
    {/if}

    <!-- Actions Editor -->
    {#if cell}
      <hr />
      <h4>{t('inspector.actions')}</h4>

      <!-- On Land -->
      <div class="action-list-section">
        <div class="action-list-header">
          <span class="action-list-title">{t('inspector.onLand')}</span>
          <select class="action-type-select" onchange={(e) => { if ((e.target as HTMLSelectElement).value) { addAction('onLand', (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = ''; } }}>
            <option value="">{t('inspector.add')}</option>
            {#each ACTION_TYPES as at}
              <option value={at}>{t(`action.${at}`)}</option>
            {/each}
          </select>
        </div>
        {#each getActions('onLand') as action, i}
          <div class="action-item">
            <div class="action-header">
              <select class="action-type" value={action.type} onchange={(e) => setActionField('onLand', i, 'type', (e.target as HTMLSelectElement).value)}>
                {#each ACTION_TYPES as at}
                  <option value={at}>{t(`action.${at}`)}</option>
                {/each}
              </select>
              <div class="action-controls">
                <button class="small" aria-label={t('inspector.moveUp')} onclick={() => moveAction('onLand', i, -1)} disabled={i === 0}>↑</button>
                <button class="small" aria-label={t('inspector.moveDown')} onclick={() => moveAction('onLand', i, 1)} disabled={i === getActions('onLand').length - 1}>↓</button>
                <button class="small danger" aria-label={t('inspector.removeAction')} onclick={() => removeAction('onLand', i)}>✕</button>
              </div>
            </div>
            <!-- Action fields -->
            <div class="action-fields">
              {#if action.type === 'gain_resource' || action.type === 'lose_resource'}
                <label>{t('inspector.resource')}
                  <select value={action.resource || ''} onchange={(e) => setActionField('onLand', i, 'resource', (e.target as HTMLSelectElement).value)}>
                    {#each resourceList() as res}
                      <option value={res}>{res}</option>
                    {/each}
                  </select>
                </label>
                <label>{t('inspector.amount')}
                  <input type="number" value={action.amount || 1} oninput={(e) => setActionField('onLand', i, 'amount', parseInt((e.target as HTMLInputElement).value) || 0)} min="1" />
                </label>
              {:else if action.type === 'transfer_resource'}
                <label>{t('inspector.resource')}
                  <select value={action.resource || ''} onchange={(e) => setActionField('onLand', i, 'resource', (e.target as HTMLSelectElement).value)}>
                    {#each resourceList() as res}
                      <option value={res}>{res}</option>
                    {/each}
                  </select>
                </label>
                <label>{t('inspector.amount')}
                  <input type="number" value={action.amount || 1} oninput={(e) => setActionField('onLand', i, 'amount', parseInt((e.target as HTMLInputElement).value) || 0)} min="1" />
                </label>
                <label>{t('inspector.target')}
                  <select value={action.target || 'owner'} onchange={(e) => setActionField('onLand', i, 'target', (e.target as HTMLSelectElement).value)}>
                    {#each TARGET_OPTIONS as to}
                      <option value={to}>{t(`target.${to}`)}</option>
                    {/each}
                  </select>
                </label>
              {:else if action.type === 'set_cell_owner'}
                <label>{t('inspector.owner')}
                  <select value={action.target || 'current'} onchange={(e) => setActionField('onLand', i, 'target', (e.target as HTMLSelectElement).value)}>
                    {#each TARGET_OPTIONS as to}
                      <option value={to}>{t(`target.${to}`)}</option>
                    {/each}
                  </select>
                </label>
              {:else if action.type === 'if_resource_ge'}
                <label>{t('inspector.resource')}
                  <select value={action.resource || ''} onchange={(e) => setActionField('onLand', i, 'resource', (e.target as HTMLSelectElement).value)}>
                    {#each resourceList() as res}
                      <option value={res}>{res}</option>
                    {/each}
                  </select>
                </label>
                <label>{t('inspector.amount')}
                  <input type="number" value={action.amount || 1} oninput={(e) => setActionField('onLand', i, 'amount', parseInt((e.target as HTMLInputElement).value) || 0)} min="1" />
                </label>
                <p class="hint">{t('inspector.nestedHint')}</p>
              {:else if action.type === 'finish_game'}
                <p class="hint">{t('inspector.finishHint')}</p>
              {:else if action.type === 'log_message'}
                <label>{t('inspector.message')}
                  <input value={action.title || ''} oninput={(e) => setActionField('onLand', i, 'title', (e.target as HTMLInputElement).value)} placeholder={t('inspector.logPlaceholder')} />
                </label>
              {/if}
            </div>
          </div>
        {/each}
        {#if getActions('onLand').length === 0}
          <p class="hint">{t('inspector.noOnLand')}</p>
        {/if}
      </div>

      <!-- On Pass -->
      <div class="action-list-section">
        <div class="action-list-header">
          <span class="action-list-title">{t('inspector.onPass')}</span>
          <select class="action-type-select" onchange={(e) => { if ((e.target as HTMLSelectElement).value) { addAction('onPass', (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = ''; } }}>
            <option value="">{t('inspector.add')}</option>
            {#each ACTION_TYPES as at}
              <option value={at}>{t(`action.${at}`)}</option>
            {/each}
          </select>
        </div>
        {#each getActions('onPass') as action, i}
          <div class="action-item">
            <div class="action-header">
              <select class="action-type" value={action.type} onchange={(e) => setActionField('onPass', i, 'type', (e.target as HTMLSelectElement).value)}>
                {#each ACTION_TYPES as at}
                  <option value={at}>{t(`action.${at}`)}</option>
                {/each}
              </select>
              <div class="action-controls">
                <button class="small" aria-label={t('inspector.moveUp')} onclick={() => moveAction('onPass', i, -1)} disabled={i === 0}>↑</button>
                <button class="small" aria-label={t('inspector.moveDown')} onclick={() => moveAction('onPass', i, 1)} disabled={i === getActions('onPass').length - 1}>↓</button>
                <button class="small danger" aria-label={t('inspector.removeAction')} onclick={() => removeAction('onPass', i)}>✕</button>
              </div>
            </div>
            <div class="action-fields">
              {#if action.type === 'gain_resource' || action.type === 'lose_resource'}
                <label>{t('inspector.resource')}
                  <select value={action.resource || ''} onchange={(e) => setActionField('onPass', i, 'resource', (e.target as HTMLSelectElement).value)}>
                    {#each resourceList() as res}
                      <option value={res}>{res}</option>
                    {/each}
                  </select>
                </label>
                <label>{t('inspector.amount')}
                  <input type="number" value={action.amount || 1} oninput={(e) => setActionField('onPass', i, 'amount', parseInt((e.target as HTMLInputElement).value) || 0)} min="1" />
                </label>
              {:else if action.type === 'transfer_resource'}
                <label>{t('inspector.resource')}
                  <select value={action.resource || ''} onchange={(e) => setActionField('onPass', i, 'resource', (e.target as HTMLSelectElement).value)}>
                    {#each resourceList() as res}
                      <option value={res}>{res}</option>
                    {/each}
                  </select>
                </label>
                <label>{t('inspector.amount')}
                  <input type="number" value={action.amount || 1} oninput={(e) => setActionField('onPass', i, 'amount', parseInt((e.target as HTMLInputElement).value) || 0)} min="1" />
                </label>
                <label>{t('inspector.target')}
                  <select value={action.target || 'owner'} onchange={(e) => setActionField('onPass', i, 'target', (e.target as HTMLSelectElement).value)}>
                    {#each TARGET_OPTIONS as to}
                      <option value={to}>{t(`target.${to}`)}</option>
                    {/each}
                  </select>
                </label>
              {:else if action.type === 'set_cell_owner'}
                <label>{t('inspector.owner')}
                  <select value={action.target || 'current'} onchange={(e) => setActionField('onPass', i, 'target', (e.target as HTMLSelectElement).value)}>
                    {#each TARGET_OPTIONS as to}
                      <option value={to}>{t(`target.${to}`)}</option>
                    {/each}
                  </select>
                </label>
              {:else if action.type === 'if_resource_ge'}
                <label>{t('inspector.resource')}
                  <select value={action.resource || ''} onchange={(e) => setActionField('onPass', i, 'resource', (e.target as HTMLSelectElement).value)}>
                    {#each resourceList() as res}
                      <option value={res}>{res}</option>
                    {/each}
                  </select>
                </label>
                <label>{t('inspector.amount')}
                  <input type="number" value={action.amount || 1} oninput={(e) => setActionField('onPass', i, 'amount', parseInt((e.target as HTMLInputElement).value) || 0)} min="1" />
                </label>
                <p class="hint">{t('inspector.nestedHint')}</p>
              {:else if action.type === 'finish_game'}
                <p class="hint">{t('inspector.finishHint')}</p>
              {:else if action.type === 'log_message'}
                <label>{t('inspector.message')}
                  <input value={action.title || ''} oninput={(e) => setActionField('onPass', i, 'title', (e.target as HTMLInputElement).value)} placeholder={t('inspector.logPlaceholder')} />
                </label>
              {/if}
            </div>
          </div>
        {/each}
        {#if getActions('onPass').length === 0}
          <p class="hint">{t('inspector.noOnPass')}</p>
        {/if}
      </div>
    {/if}

    <hr />
    <h4>{t('inspector.edges', { count: cellEdges.length })}</h4>
    {#each cellEdges as edge}
      <div class="edge-row" class:selected={edge.id === selectedEdgeId} onclick={() => onEdgeSelect?.(edge.id)} onkeydown={(e) => e.key === 'Enter' && onEdgeSelect?.(edge.id)} role="button" tabindex="0">
        <span>{edge.from} → {edge.to}</span>
        <button class="small" aria-label={t('inspector.removeEdge')} onclick={() => onDeleteEdge?.(edge.id)}>✕</button>
      </div>
    {/each}

    <hr />
    <button class="delete" onclick={() => onDeleteCell?.(cell.id)}>{t('inspector.deleteCell')}</button>
  {:else}
    <p class="hint">{t('inspector.selectCell')}</p>
  {/if}

  {#if selectedEdgeId && !cell && selectedEdge}
    <hr />
    <h4>{t('inspector.edge', { id: selectedEdge.id })}</h4>
    <p class="edge-route">{selectedEdge.from} → {selectedEdge.to}</p>
    <label>
      {t('inspector.conditionType')}
      <select value={selectedEdge.condition.type} onchange={(e) => updateConditionType((e.target as HTMLSelectElement).value)}>
        {#each conditionTypes as ct}
          <option value={ct}>{t(`condition.${ct}`)}</option>
        {/each}
      </select>
    </label>

    {#if selectedEdge.condition.type === 'dice_total_in'}
      <label>
        {t('inspector.values')}
        <input
          value={(selectedEdge.condition.values ?? []).join(',')}
          oninput={(e) => updateConditionField('values', (e.target as HTMLInputElement).value.split(',').map(v => parseInt(v.trim()) || 0).filter(v => v > 0))}
          placeholder="1,3,5"
        />
      </label>
    {/if}

    {#if selectedEdge.condition.type === 'manual_choice' || selectedEdge.condition.type === 'pay_resource'}
      <label>
        {t('inspector.label')}
        <input
          value={selectedEdge.condition.label ?? ''}
          oninput={(e) => updateConditionField('label', (e.target as HTMLInputElement).value)}
          placeholder={t('inspector.labelPlaceholder')}
        />
      </label>
    {/if}

    {#if selectedEdge.condition.type === 'player_resource_at_least' || selectedEdge.condition.type === 'pay_resource'}
      <label>
        {t('inspector.resource')}
        <input
          value={selectedEdge.condition.resource ?? ''}
          oninput={(e) => updateConditionField('resource', (e.target as HTMLInputElement).value)}
          placeholder={t('inspector.resourcePlaceholder')}
        />
      </label>
      <label>
        {t('inspector.amount')}
        <input
          type="number"
          value={selectedEdge.condition.amount ?? 1}
          oninput={(e) => updateConditionField('amount', parseInt((e.target as HTMLInputElement).value) || 0)}
          min="1"
        />
      </label>
    {/if}

    {#if selectedEdge.condition.type === 'dice_total_even' || selectedEdge.condition.type === 'dice_total_odd'}
      <p class="hint">{t('inspector.noConditionFields')}</p>
    {/if}

    <hr />
    <button class="delete" onclick={() => onDeleteEdge?.(selectedEdgeId!)}>{t('inspector.deleteEdge')}</button>
  {/if}
</div>

<style>
  .inspector {
    width: 260px;
    padding: 16px;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    overflow-y: auto;
    font-size: 13px;
  }
  .inspector h3, .inspector h4 {
    margin: 0 0 8px;
    color: #e94560;
  }
  .inspector label {
    display: block;
    margin-bottom: 8px;
    color: #aaa;
    font-size: 11px;
    text-transform: uppercase;
  }
  .inspector input, .inspector select {
    display: block;
    width: 100%;
    margin-top: 4px;
    padding: 6px 8px;
    background: #0d1b2a;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
    box-sizing: border-box;
    font-size: 13px;
  }
  .inspector input[type="color"] {
    height: 32px;
    padding: 2px;
  }
  .inspector input[type="checkbox"] {
    width: auto;
    margin-top: 6px;
  }
  .inspector hr {
    border: none;
    border-top: 1px solid #0f3460;
    margin: 12px 0;
  }
  .hint {
    color: #666;
    font-style: italic;
  }
  .delete {
    background: #5c1a1a;
    color: #e94560;
    border: 1px solid #e94560;
    padding: 8px 16px;
    border-radius: 4px;
    cursor: pointer;
    width: 100%;
    font-size: 13px;
  }
  .delete:hover {
    background: #7a2222;
  }
  .edge-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
    margin-bottom: 4px;
    background: #0d1b2a;
  }
  .edge-row.selected {
    border: 1px solid #e94560;
  }
  .edge-row:hover {
    background: #1a2a4a;
  }
  .edge-route {
    font-size: 12px;
    color: #4fc3f7;
    margin: 4px 0 12px;
    font-family: monospace;
  }
  button.small {
    background: none;
    border: 1px solid #555;
    color: #e94560;
    padding: 2px 6px;
    cursor: pointer;
    border-radius: 3px;
  }

  /* Action editor styles */
  .action-list-section {
    margin-bottom: 12px;
  }
  .action-list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
  }
  .action-list-title {
    font-weight: bold;
    color: #4fc3f7;
    font-size: 12px;
    text-transform: uppercase;
  }
  .action-type-select {
    font-size: 11px;
    padding: 2px 4px;
    background: #0d1b2a;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 3px;
    width: auto;
  }
  .action-item {
    background: #0d1b2a;
    border: 1px solid #0f3460;
    border-radius: 4px;
    margin-bottom: 6px;
    padding: 6px;
  }
  .action-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 4px;
  }
  .action-type {
    flex: 1;
    font-size: 11px;
    padding: 2px 4px;
    background: #16213e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 3px;
  }
  .action-controls {
    display: flex;
    gap: 2px;
  }
  .action-controls .small {
    padding: 1px 4px;
    font-size: 10px;
  }
  .action-controls .small:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
  .small.danger {
    color: #e94560;
    border-color: #e94560;
  }
  .action-fields {
    margin-top: 6px;
  }
  .action-fields label {
    display: block;
    margin-bottom: 4px;
    font-size: 10px;
    color: #888;
  }
  .action-fields input, .action-fields select {
    display: block;
    width: 100%;
    margin-top: 2px;
    padding: 3px 6px;
    background: #16213e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 3px;
    box-sizing: border-box;
    font-size: 11px;
  }
  .action-fields .hint {
    font-size: 10px;
    color: #555;
    margin: 4px 0 0;
  }
</style>
