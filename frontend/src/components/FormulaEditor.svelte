<script lang="ts">
  import type { AmountFormula, AmountTerm, CellDefinition, CellQuery, RuleSet } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';
  import CellQueryEditor from './CellQueryEditor.svelte';

  type Props = {
    formula?: AmountFormula;
    resources: string[];
    rules: RuleSet;
    cells: CellDefinition[];
    onChange: (formula: AmountFormula | undefined) => void;
  };

  let { formula, resources, rules, cells, onChange }: Props = $props();
  let t = $derived(i18n.t);
  let open = $derived(formula !== undefined);

  type Slot = 'base' | 'plus' | 'minus' | 'times' | 'dividedBy';
  const SLOTS: Slot[] = ['base', 'plus', 'minus', 'times', 'dividedBy'];

  /**
   * A term is a plain number, a value read from the player or the cell, or a
   * count of matching cells — the whole vocabulary an author needs: "10", "the
   * cell's damage field", "my defence including armour", "how many stations
   * this owner holds".
   */
  function termKind(term?: AmountTerm): string {
    return term?.kind ?? '';
  }

  function setTerm(slot: Slot, changes: Partial<AmountTerm> | null) {
    const next: AmountFormula = { ...(formula ?? {}) };
    if (changes === null) {
      delete next[slot];
    } else {
      next[slot] = { kind: 'const', value: 0, ...(next[slot] ?? {}), ...changes };
    }
    onChange(next);
  }

  function setClamp(key: 'min' | 'max', raw: string) {
    const next: AmountFormula = { ...(formula ?? {}) };
    if (raw === '') delete next[key];
    else next[key] = Number(raw);
    onChange(next);
  }

  function toggle() {
    onChange(open ? undefined : { base: { kind: 'const', value: 0 } });
  }
</script>

<div class="formula">
  <label class="toggle">
    <input type="checkbox" checked={open} onchange={toggle} />
    {t('inspector.useFormula')}
  </label>

  {#if formula}
    <p class="explain">{t('inspector.formulaHint')}</p>
    {#each SLOTS as slot (slot)}
      {@const term = formula[slot]}
      <div class="term">
        <span class="slot-label">{t(`inspector.formula.${slot}`)}</span>
        <select
          value={termKind(term)}
          onchange={(event) => {
            const kind = (event.target as HTMLSelectElement).value;
            if (!kind) setTerm(slot, null);
            else setTerm(slot, { kind: kind as AmountTerm['kind'], name: '', value: 0 });
          }}
        >
          <option value="">{t('inspector.formula.nothing')}</option>
          <option value="const">{t('inspector.formula.const')}</option>
          <option value="field">{t('inspector.formula.field')}</option>
          <option value="stat">{t('inspector.formula.stat')}</option>
          <option value="resource">{t('inspector.formula.resource')}</option>
          <option value="cells">{t('inspector.formula.cells')}</option>
        </select>

        {#if term?.kind === 'const'}
          <input
            type="number"
            value={term.value ?? 0}
            oninput={(event) => setTerm(slot, { value: Number((event.target as HTMLInputElement).value) })}
          />
        {:else if term?.kind === 'field'}
          <input
            value={term.name ?? ''}
            placeholder={t('inspector.formula.fieldName')}
            oninput={(event) => setTerm(slot, { name: (event.target as HTMLInputElement).value })}
          />
        {:else if term?.kind === 'stat' || term?.kind === 'resource'}
          <select value={term.name ?? ''} onchange={(event) => setTerm(slot, { name: (event.target as HTMLSelectElement).value })}>
            <option value="">{t('inspector.choose')}</option>
            {#each resources as name (name)}<option value={name}>{name}</option>{/each}
          </select>
        {/if}

        {#if term?.kind === 'cells'}
          <div class="term-query">
            <CellQueryEditor
              query={term.query}
              {rules}
              {cells}
              onChange={(query: CellQuery) => setTerm(slot, { query })}
            />
          </div>
        {/if}
      </div>
    {/each}

    <div class="clamps">
      {#each ['min', 'max'] as const as key (key)}
        <label>
          {t(`inspector.formula.${key}`)}
          <input
            type="number"
            value={formula[key] ?? ''}
            oninput={(event) => setClamp(key, (event.target as HTMLInputElement).value)}
          />
        </label>
      {/each}
    </div>
  {/if}
</div>

<style>
  .formula {
    container-type: inline-size;
    display: grid;
    gap: var(--space-1);
    padding: var(--space-2);
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .explain {
    margin: 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .term {
    display: grid;
    /* Stacked by default so it fits a narrow inspector, side by side once
       there is room. */
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-1);
    align-items: center;
  }
  @container (min-width: 260px) {
    .term {
      grid-template-columns: 4rem minmax(0, 1fr) minmax(0, 1fr);
    }
  }
  /* A query needs the full width whatever the term row is doing. */
  .term-query {
    grid-column: 1 / -1;
  }
  .slot-label {
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .clamps {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(70px, 1fr));
    gap: var(--space-1);
  }
  label {
    display: grid;
    gap: 2px;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  select, input {
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
