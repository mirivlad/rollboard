<script lang="ts">
  import type { ActionDefinition, AmountFormula, CellDefinition, RuleSet } from '../lib/types';
  import { ACTION_GROUPS, ACTION_SCHEMAS, blankAction, schemaFor, type ActionField } from '../lib/action-schema';
  import { i18n } from '../lib/i18n.svelte';
  import FormulaEditor from './FormulaEditor.svelte';
  import ActionEditor from './ActionEditor.svelte';

  type Props = {
    actions: ActionDefinition[];
    rules: RuleSet;
    cells: CellDefinition[];
    onChange: (actions: ActionDefinition[]) => void;
    /** Nested lists are indented and get a lighter frame. */
    depth?: number;
  };

  let { actions, rules, cells, onChange, depth = 0 }: Props = $props();
  let t = $derived(i18n.t);

  let resourceNames = $derived(Object.keys(rules.resources ?? {}));
  let itemEntries = $derived(Object.entries(rules.items ?? {}));
  let slotNames = $derived(rules.equipmentSlots ?? []);

  function update(index: number, changes: Partial<ActionDefinition>) {
    onChange(actions.map((action, i) => (i === index ? { ...action, ...changes } : action)));
  }

  function changeType(index: number, type: string) {
    // Starting from a blank action drops fields the new type has no use for,
    // which stops an old amount silently surviving into an action that ignores it.
    onChange(actions.map((action, i) => (i === index ? blankAction(type) : action)));
  }

  function add(type: string) {
    if (!type) return;
    onChange([...actions, blankAction(type)]);
  }

  function remove(index: number) {
    onChange(actions.filter((_, i) => i !== index));
  }

  function move(index: number, delta: number) {
    const next = [...actions];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  }

  function value(action: ActionDefinition, field: ActionField): string {
    const raw = (action as unknown as Record<string, unknown>)[field.name];
    return raw === undefined || raw === null ? '' : String(raw);
  }

  function setField(index: number, field: ActionField, raw: string) {
    if (field.kind === 'number') {
      update(index, { [field.name]: raw === '' ? undefined : Number(raw) } as Partial<ActionDefinition>);
      return;
    }
    update(index, { [field.name]: raw === '' ? undefined : raw } as Partial<ActionDefinition>);
  }

  // --- choice and outcome lists ---------------------------------------------
  function updateOption(index: number, optionIndex: number, changes: Record<string, unknown>) {
    const options = [...(actions[index].options ?? [])];
    options[optionIndex] = { ...options[optionIndex], ...changes };
    update(index, { options });
  }

  function addOption(index: number) {
    const options = [...(actions[index].options ?? [])];
    options.push({ id: `option_${options.length + 1}`, title: '', then: [] });
    update(index, { options });
  }

  function removeOption(index: number, optionIndex: number) {
    update(index, { options: (actions[index].options ?? []).filter((_, i) => i !== optionIndex) });
  }
</script>

<div class="action-list" class:nested={depth > 0}>
  {#each actions as action, index (index)}
    {@const schema = schemaFor(action.type)}
    <div class="action">
      <div class="action-head">
        <select
          class="type"
          value={action.type}
          onchange={(event) => changeType(index, (event.target as HTMLSelectElement).value)}
          aria-label={t('inspector.actionType')}
        >
          {#each ACTION_GROUPS as group (group)}
            <optgroup label={t(`actionGroup.${group}`)}>
              {#each ACTION_SCHEMAS.filter((s) => s.group === group) as option (option.type)}
                <option value={option.type}>{t(option.labelKey)}</option>
              {/each}
            </optgroup>
          {/each}
        </select>
        <div class="controls">
          <button class="small" aria-label={t('inspector.moveUp')} onclick={() => move(index, -1)} disabled={index === 0}>↑</button>
          <button class="small" aria-label={t('inspector.moveDown')} onclick={() => move(index, 1)} disabled={index === actions.length - 1}>↓</button>
          <button class="small danger" aria-label={t('inspector.removeAction')} onclick={() => remove(index)}>✕</button>
        </div>
      </div>

      {#if !schema}
        <!-- The engine tolerates unknown types for forward compatibility, so
             the editor says so rather than silently dropping the action. -->
        <p class="hint warn">{t('inspector.unknownAction', { type: action.type })}</p>
      {/if}

      <div class="fields">
        {#each schema?.fields ?? [] as field (field.name)}
          {#if field.kind === 'actions'}
            <div class="branch">
              <span class="branch-label">{t(field.labelKey)}</span>
              <ActionEditor
                actions={((action as unknown as Record<string, unknown>)[field.name] as ActionDefinition[]) ?? []}
                {rules}
                {cells}
                depth={depth + 1}
                onChange={(next) => update(index, { [field.name]: next } as Partial<ActionDefinition>)}
              />
            </div>
          {:else if field.kind === 'options'}
            <div class="branch">
              <span class="branch-label">{t(field.labelKey)}</span>
              {#each action.options ?? [] as option, optionIndex (optionIndex)}
                <div class="option">
                  <div class="option-head">
                    <input
                      class="option-title"
                      value={option.title ?? ''}
                      placeholder={t('inspector.optionTitle')}
                      oninput={(event) => updateOption(index, optionIndex, { title: (event.target as HTMLInputElement).value })}
                    />
                    <button class="small danger" aria-label={t('inspector.removeOption')} onclick={() => removeOption(index, optionIndex)}>✕</button>
                  </div>
                  <ActionEditor
                    actions={option.then ?? []}
                    {rules}
                    {cells}
                    depth={depth + 1}
                    onChange={(next) => updateOption(index, optionIndex, { then: next })}
                  />
                </div>
              {/each}
              <button class="add-option" onclick={() => addOption(index)}>{t('inspector.addOption')}</button>
            </div>
          {:else if field.kind === 'formula'}
            <FormulaEditor
              formula={action.formula}
              resources={resourceNames}
              onChange={(next: AmountFormula | undefined) => update(index, { formula: next })}
            />
          {:else}
            <label>
              {t(field.labelKey)}
              {#if field.kind === 'resource'}
                <select value={value(action, field)} onchange={(e) => setField(index, field, (e.target as HTMLSelectElement).value)}>
                  <option value="">{t('inspector.choose')}</option>
                  {#each resourceNames as name (name)}<option value={name}>{name}</option>{/each}
                </select>
              {:else if field.kind === 'item'}
                <select value={value(action, field)} onchange={(e) => setField(index, field, (e.target as HTMLSelectElement).value)}>
                  <option value="">{t('inspector.choose')}</option>
                  {#each itemEntries as [id, item] (id)}<option value={id}>{item.title || id}</option>{/each}
                </select>
              {:else if field.kind === 'slot'}
                <select value={value(action, field)} onchange={(e) => setField(index, field, (e.target as HTMLSelectElement).value)}>
                  <option value="">{t('inspector.choose')}</option>
                  {#each slotNames as slot (slot)}<option value={slot}>{slot}</option>{/each}
                </select>
              {:else if field.kind === 'cell'}
                <select value={value(action, field)} onchange={(e) => setField(index, field, (e.target as HTMLSelectElement).value)}>
                  <option value="">{t('inspector.choose')}</option>
                  {#each cells as cell (cell.id)}<option value={cell.id}>{cell.title || cell.id}</option>{/each}
                </select>
              {:else if field.kind === 'target'}
                <select value={value(action, field)} onchange={(e) => setField(index, field, (e.target as HTMLSelectElement).value)}>
                  <option value="">{t('inspector.choose')}</option>
                  <option value="current">{t('target.current')}</option>
                  <option value="owner">{t('target.owner')}</option>
                  <option value="bank">{t('target.bank')}</option>
                  <option value="none">{t('target.none')}</option>
                </select>
              {:else if field.kind === 'boolean'}
                <select value={value(action, field)} onchange={(e) => setField(index, field, (e.target as HTMLSelectElement).value)}>
                  <option value="true">{t('inspector.yes')}</option>
                  <option value="false">{t('inspector.no')}</option>
                </select>
              {:else if field.kind === 'number'}
                <input type="number" value={value(action, field)} oninput={(e) => setField(index, field, (e.target as HTMLInputElement).value)} />
              {:else}
                <input value={value(action, field)} oninput={(e) => setField(index, field, (e.target as HTMLInputElement).value)} />
              {/if}
            </label>
          {/if}
        {/each}
      </div>
    </div>
  {/each}

  <select
    class="add"
    value=""
    onchange={(event) => { add((event.target as HTMLSelectElement).value); (event.target as HTMLSelectElement).value = ''; }}
    aria-label={t('inspector.add')}
  >
    <option value="">{t('inspector.add')}</option>
    {#each ACTION_GROUPS as group (group)}
      <optgroup label={t(`actionGroup.${group}`)}>
        {#each ACTION_SCHEMAS.filter((s) => s.group === group) as option (option.type)}
          <option value={option.type}>{t(option.labelKey)}</option>
        {/each}
      </optgroup>
    {/each}
  </select>
</div>

<style>
  .action-list {
    display: grid;
    gap: var(--space-2);
  }
  .nested {
    margin-left: var(--space-3);
    padding-left: var(--space-3);
    border-left: 2px solid var(--border-subtle);
  }
  .action {
    display: grid;
    gap: var(--space-2);
    padding: var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
  }
  .action-head {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .type {
    flex: 1;
    min-width: 0;
  }
  .controls {
    display: flex;
    gap: var(--space-1);
  }
  .fields {
    display: grid;
    gap: var(--space-2);
  }
  label {
    display: grid;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  select, input {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
  }
  .branch {
    display: grid;
    gap: var(--space-1);
  }
  .branch-label {
    color: var(--text-faint);
    font-size: var(--text-xs);
    text-transform: uppercase;
    letter-spacing: .06em;
  }
  .option {
    display: grid;
    gap: var(--space-1);
    padding: var(--space-2);
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
  }
  .option-head {
    display: flex;
    gap: var(--space-1);
  }
  .option-title {
    flex: 1;
    min-width: 0;
  }
  .small, .add-option, .add {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-raised);
    color: var(--text);
    font: inherit;
    font-size: var(--text-xs);
    cursor: pointer;
  }
  .small:hover:not(:disabled), .add-option:hover, .add:hover {
    border-color: var(--accent);
    background: var(--accent-surface);
  }
  .small:disabled {
    opacity: .4;
    cursor: not-allowed;
  }
  .danger:hover {
    border-color: var(--danger);
    background: var(--danger-surface);
  }
  .hint {
    margin: 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .warn {
    color: var(--warning);
  }
</style>
