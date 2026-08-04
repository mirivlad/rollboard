<script lang="ts">
  import type { CellDefinition, EdgeDefinition, RuleSet, ActionDefinition, ActionOption } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';
  import ActionEditor from './ActionEditor.svelte';
  import ImageField from './ImageField.svelte';

  let { cell, edges, rules, allCells = [], onCellChange, onDeleteCell, onDeleteEdge, selectedEdgeId, onEdgeSelect, onEdgeChange }: {
    cell: CellDefinition | null | undefined;
    edges: EdgeDefinition[];
    allCells?: CellDefinition[];
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
    <ImageField
      value={cell.visual.baseImage}
      onChange={(url) => updateVisual('baseImage', url)}
    />

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

    <!-- Actions -->
    {#if cell}
      <hr />
      <h4>{t('inspector.actions')}</h4>

      <div class="action-list-section">
        <span class="action-list-title">{t('inspector.onLand')}</span>
        <ActionEditor
          actions={cell.onLand ?? []}
          {rules}
          cells={allCells}
          onChange={(next) => onCellChange?.({ ...cell, onLand: next })}
        />
      </div>

      <div class="action-list-section">
        <span class="action-list-title">{t('inspector.onPass')}</span>
        <ActionEditor
          actions={cell.onPass ?? []}
          {rules}
          cells={allCells}
          onChange={(next) => onCellChange?.({ ...cell, onPass: next })}
        />
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
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow-y: auto;
    font-size: 13px;
  }
  .inspector h3, .inspector h4 {
    margin: 0 0 8px;
    color: var(--danger);
  }
  .inspector label {
    display: block;
    margin-bottom: 8px;
    color: var(--text-faint);
    font-size: 11px;
    text-transform: uppercase;
  }
  .inspector input, .inspector select {
    display: block;
    width: 100%;
    margin-top: 4px;
    padding: 6px 8px;
    background: var(--surface-sunken);
    border: 1px solid var(--border);
    color: var(--text);
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
    border-top: 1px solid var(--border);
    margin: 12px 0;
  }
  .hint {
    color: var(--text-faint);
    font-style: italic;
  }
  .delete {
    background: var(--danger-surface);
    color: var(--danger);
    border: 1px solid var(--danger);
    padding: 8px 16px;
    border-radius: 4px;
    cursor: pointer;
    width: 100%;
    font-size: 13px;
  }
  .delete:hover {
    background: var(--danger-surface);
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
    background: var(--surface-sunken);
  }
  .edge-row.selected {
    border: 1px solid var(--danger);
  }
  .edge-row:hover {
    background: var(--accent-surface);
  }
  .edge-route {
    font-size: 12px;
    color: var(--accent);
    margin: 4px 0 12px;
    font-family: monospace;
  }
  button.small {
    background: none;
    border: 1px solid var(--border-strong);
    color: var(--danger);
    padding: 2px 6px;
    cursor: pointer;
    border-radius: 3px;
  }

  /* Action editor styles */
  .action-list-section {
    margin-bottom: 12px;
  }
  .action-list-title {
    font-weight: bold;
    color: var(--accent);
    font-size: 12px;
    text-transform: uppercase;
  }
</style>
