<script lang="ts">
  import type { CellDefinition, EdgeDefinition, RuleSet } from '../lib/types';

  let { cell, edges, rules, onCellChange, onDeleteCell, onDeleteEdge, selectedEdgeId, onEdgeSelect }: {
    cell: CellDefinition | null | undefined;
    edges: EdgeDefinition[];
    rules: RuleSet;
    onCellChange?: (cell: CellDefinition) => void;
    onDeleteCell?: (id: string) => void;
    onDeleteEdge?: (id: string) => void;
    selectedEdgeId?: string;
    onEdgeSelect?: (id: string | undefined) => void;
  } = $props();

  let typeDef = $derived(cell ? rules.cellTypes[cell.type] : null);
  let cellEdges = $derived(edges.filter(e => e.from === cell?.id || e.to === cell?.id));

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
</script>

<div class="inspector">
  {#if cell}
    <h3>Cell: {cell.id}</h3>
    <label>
      ID
      <input value={cell.id} oninput={(e) => onCellChange?.({ ...cell, id: (e.target as HTMLInputElement).value })} />
    </label>
    <label>
      Title
      <input value={cell.title} oninput={(e) => onCellChange?.({ ...cell, title: (e.target as HTMLInputElement).value })} />
    </label>
    <label>
      Type
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
      Position X
      <input type="number" value={cell.x} oninput={(e) => onCellChange?.({ ...cell, x: parseInt((e.target as HTMLInputElement).value) || 0 })} />
    </label>
    <label>
      Position Y
      <input type="number" value={cell.y} oninput={(e) => onCellChange?.({ ...cell, y: parseInt((e.target as HTMLInputElement).value) || 0 })} />
    </label>
    <hr />
    <h4>Visual</h4>
    <label>
      Color
      <input type="color" value={cell.visual.baseColor} oninput={(e) => updateVisual('baseColor', (e.target as HTMLInputElement).value)} />
    </label>
    <label>
      Image URL
      <input value={cell.visual.baseImage} oninput={(e) => updateVisual('baseImage', (e.target as HTMLInputElement).value)} placeholder="optional URL" />
    </label>

    {#if typeDef && Object.keys(typeDef.fields).length > 0}
      <hr />
      <h4>Fields</h4>
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

    <hr />
    <h4>Edges ({cellEdges.length})</h4>
    {#each cellEdges as edge}
      <div class="edge-row" class:selected={edge.id === selectedEdgeId} onclick={() => onEdgeSelect?.(edge.id)} onkeydown={(e) => e.key === 'Enter' && onEdgeSelect?.(edge.id)} role="button" tabindex="0">
        <span>{edge.from} → {edge.to}</span>
        <button class="small" onclick={() => onDeleteEdge?.(edge.id)}>✕</button>
      </div>
    {/each}

    <hr />
    <button class="delete" onclick={() => onDeleteCell?.(cell.id)}>Delete Cell</button>
  {:else}
    <p class="hint">Select a cell to edit</p>
  {/if}

  {#if selectedEdgeId && !cell}
    <hr />
    <h4>Selected Edge: {selectedEdgeId}</h4>
    <button class="delete" onclick={() => onDeleteEdge?.(selectedEdgeId)}>Delete Edge</button>
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
  button.small {
    background: none;
    border: 1px solid #555;
    color: #e94560;
    padding: 2px 6px;
    cursor: pointer;
    border-radius: 3px;
  }
</style>
