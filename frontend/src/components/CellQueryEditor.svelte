<script lang="ts">
  import type { CellDefinition, CellQuery, RuleSet } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';

  type Props = {
    query?: CellQuery;
    rules: RuleSet;
    cells: CellDefinition[];
    onChange: (query: CellQuery) => void;
  };

  let { query, rules, cells, onChange }: Props = $props();
  let t = $derived(i18n.t);
  let current = $derived(query ?? {});

  let cellTypes = $derived(Object.entries(rules.cellTypes ?? {}));

  /**
   * Every field any cell type declares, plus anything the board actually
   * carries.
   *
   * Authors add fields to cells directly as well as through the type, so
   * offering only the declared ones would hide half the board's vocabulary.
   */
  let fieldNames = $derived.by(() => {
    const names = new Set<string>();
    for (const [, type] of cellTypes) for (const name of Object.keys(type.fields ?? {})) names.add(name);
    for (const cell of cells) for (const name of Object.keys(cell.fields ?? {})) names.add(name);
    return [...names].sort();
  });

  /** The values that field actually holds on this board, so "blue" is a pick. */
  let fieldValues = $derived.by(() => {
    if (!current.field) return [];
    const values = new Set<string>();
    for (const cell of cells) {
      const raw = cell.fields?.[current.field];
      if (raw !== undefined && raw !== null && raw !== '') values.add(String(raw));
    }
    return [...values].sort();
  });

  function set(changes: Partial<CellQuery>) {
    const next: CellQuery = { ...current, ...changes };
    // Blank entries are dropped rather than stored as empty strings, so a
    // query reads as the filters the author actually set.
    for (const [key, value] of Object.entries(next)) {
      if (value === '' || value === undefined || value === false) delete (next as Record<string, unknown>)[key];
    }
    onChange(next);
  }
</script>

<div class="query">
  <p class="explain">{t('inspector.queryHint')}</p>

  <label>
    {t('inspector.queryType')}
    <select value={current.type ?? ''} onchange={(e) => set({ type: (e.target as HTMLSelectElement).value })}>
      <option value="">{t('inspector.queryAnyType')}</option>
      {#each cellTypes as [id, type] (id)}<option value={id}>{type.title || id}</option>{/each}
    </select>
  </label>

  <label>
    {t('inspector.queryOwner')}
    <select value={current.owner ?? ''} onchange={(e) => set({ owner: (e.target as HTMLSelectElement).value as CellQuery['owner'] })}>
      <option value="">{t('queryOwner.any')}</option>
      <option value="none">{t('queryOwner.none')}</option>
      <option value="current">{t('queryOwner.current')}</option>
      <option value="cellOwner">{t('queryOwner.cellOwner')}</option>
      <option value="other">{t('queryOwner.other')}</option>
    </select>
  </label>

  <label>
    {t('inspector.queryField')}
    <select value={current.field ?? ''} onchange={(e) => set({ field: (e.target as HTMLSelectElement).value })}>
      <option value="">{t('inspector.queryNoField')}</option>
      {#each fieldNames as name (name)}<option value={name}>{name}</option>{/each}
    </select>
  </label>

  {#if current.field}
    <label class="toggle">
      <input
        type="checkbox"
        checked={current.sameAsCell ?? false}
        onchange={(e) => set({ sameAsCell: (e.target as HTMLInputElement).checked, value: '' })}
      />
      {t('inspector.querySameAsCell')}
    </label>

    {#if !current.sameAsCell}
      <label>
        {t('inspector.queryValue')}
        <select value={current.value ?? ''} onchange={(e) => set({ value: (e.target as HTMLSelectElement).value })}>
          <option value="">{t('inspector.choose')}</option>
          {#each fieldValues as value (value)}<option {value}>{value}</option>{/each}
        </select>
      </label>
    {/if}
  {/if}

  <div class="row">
    <label>
      {t('inspector.queryMinLevel')}
      <input
        type="number"
        min="0"
        value={current.minLevel ?? ''}
        oninput={(e) => {
          const raw = (e.target as HTMLInputElement).value;
          set({ minLevel: raw === '' ? undefined : Number(raw) });
        }}
      />
    </label>
    <label class="toggle">
      <input
        type="checkbox"
        checked={current.excludeCurrentCell ?? false}
        onchange={(e) => set({ excludeCurrentCell: (e.target as HTMLInputElement).checked })}
      />
      {t('inspector.queryExcludeThis')}
    </label>
  </div>
</div>

<style>
  .query {
    container-type: inline-size;
    display: grid;
    gap: var(--space-1);
    padding: var(--space-2);
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
  }
  .explain {
    margin: 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  label {
    display: grid;
    gap: 2px;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-1);
    align-items: end;
  }
  @container (min-width: 260px) {
    .row {
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    }
  }
  select, input[type='number'] {
    min-width: 0;
    padding: var(--space-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--text-xs);
  }
</style>
