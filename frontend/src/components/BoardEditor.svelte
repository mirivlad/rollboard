<script lang="ts">
  import type { GameDefinition, CellDefinition, EdgeDefinition } from '../lib/types';
  import BoardCanvas from './BoardCanvas.svelte';
  import CellInspector from './CellInspector.svelte';
  import { i18n } from '../lib/i18n.svelte';

  let { game, onsave }: {
    game: GameDefinition;
    onsave?: () => void;
  } = $props();

  let selectedCellId = $state<string | undefined>();
  let selectedEdgeId = $state<string | undefined>();
  let mode = $state<'select' | 'connect'>('select');
  let t = $derived(i18n.t);

  // Board geometry state — initialized from game on load
  let cols = $state(1);
  let rows = $state(1);
  let cellSize = $state(96);

  let boardWidth = $derived(cols * cellSize);
  let boardHeight = $derived(rows * cellSize);

  // Dice and rules state
  let diceCount = $state(1);
  let diceSides = $state(6);
  let title = $state('');
  let startBonus = $state(0);
  let startBonusResource = $state('');

  let selectedCell = $derived(
    game.board.cells.find(c => c.id === selectedCellId)
  );

  // Sync editor fields when switching to a different game
  // We read game.id reactively inside $effect so that when the parent
  // passes a new GameDefinition (different game), all fields update
  $effect(() => {
    const gId = game.id;
    const b = game.board;
    const r = game.rules;
    cols = Math.max(1, Math.floor(b.width / b.cellSize));
    rows = Math.max(1, Math.floor(b.height / b.cellSize));
    cellSize = b.cellSize;
    diceCount = r.dice.count;
    diceSides = r.dice.sides;
    title = game.title;
    startBonus = r.startBonus ?? 0;
    startBonusResource = r.startBonusResource ?? '';
    // Normalize board dimensions: ensure width/height match cols*cellSize
    game.board.width = cols * cellSize;
    game.board.height = rows * cellSize;
  });

  function normalizeBoard() {
    game.board.width = boardWidth;
    game.board.height = boardHeight;
    game.board.cellSize = cellSize;
  }

  function updateCols() {
    if (cols < 1) cols = 1;
    normalizeBoard();
  }

  function updateRows() {
    if (rows < 1) rows = 1;
    normalizeBoard();
  }

  function updateCellSize() {
    if (cellSize < 8) cellSize = 8;
    // Recalculate cols/rows from current dimensions
    cols = Math.max(1, Math.floor(game.board.width / cellSize));
    rows = Math.max(1, Math.floor(game.board.height / cellSize));
    normalizeBoard();
  }

  function updateDice() {
    if (diceCount < 1) diceCount = 1;
    if (diceCount > 10) diceCount = 10;
    if (diceSides < 2) diceSides = 2;
    if (diceSides > 100) diceSides = 100;
    game.rules.dice.count = diceCount;
    game.rules.dice.sides = diceSides;
  }

  // Start bonus sync: called when values change
  function updateStartBonus() {
    if (startBonus < 0) startBonus = 0;
    game.rules.startBonus = startBonus;
  }

  function updateStartBonusResource() {
    game.rules.startBonusResource = startBonusResource;
  }

  function updateTitle() {
    if (title.trim() === '') title = t('game.untitled');
    game.title = title;
  }

  function addCell() {
    const id = `cell_${game.board.cells.length + 1}`;
    // Find next free grid slot
    const occupied = new Set(game.board.cells.map(c => `${c.x},${c.y}`));
    let nx = 0;
    let ny = 0;
    let found = false;
    for (let r = 0; r < rows && !found; r++) {
      for (let c = 0; c < cols && !found; c++) {
        const key = `${c * cellSize},${r * cellSize}`;
        if (!occupied.has(key)) {
          nx = c * cellSize;
          ny = r * cellSize;
          found = true;
        }
      }
    }
    const newCell: CellDefinition = {
      id,
      title: t('game.newCell', { number: game.board.cells.length + 1 }),
      type: Object.keys(game.rules.cellTypes)[0] || 'empty',
      x: nx,
      y: ny,
      visual: { baseColor: '#ccc', baseImage: '' },
      fields: {},
    };
    game.board.cells = [...game.board.cells, newCell];
    selectedCellId = id;
  }

  function deleteCell(id: string) {
    game.board.cells = game.board.cells.filter(c => c.id !== id);
    game.board.edges = game.board.edges.filter(e => e.from !== id && e.to !== id);
    if (selectedCellId === id) selectedCellId = undefined;
  }

  function handleCellMove(id: string, x: number, y: number) {
    game.board.cells = game.board.cells.map(c =>
      c.id === id ? { ...c, x, y } : c
    );
  }

  function handleCellSelect(id: string | undefined) {
    selectedCellId = id;
    selectedEdgeId = undefined;
  }

  function handleEdgeSelect(id: string | undefined) {
    selectedEdgeId = id;
    selectedCellId = undefined;
  }

  function handleCellChange(cell: CellDefinition) {
    const idx = game.board.cells.findIndex(c => c.id === cell.id);
    if (idx !== -1) {
      const cells = [...game.board.cells];
      cells[idx] = cell;
      game.board.cells = cells;
      selectedCellId = cell.id;
    }
  }

  function handleCanvasClick(x: number, y: number) {
    selectedCellId = undefined;
    selectedEdgeId = undefined;
  }

  function handleAddEdge(from: string, to: string) {
    const id = `e${game.board.edges.length + 1}_${Date.now()}`;
    game.board.edges = [
      ...game.board.edges,
      { id, from, to, condition: { type: 'always' } },
    ];
  }

  function deleteEdge(id: string) {
    game.board.edges = game.board.edges.filter(e => e.id !== id);
    selectedEdgeId = undefined;
  }

  function handleEdgeChange(edge: EdgeDefinition) {
    const idx = game.board.edges.findIndex(e => e.id === edge.id);
    if (idx !== -1) {
      const edges = [...game.board.edges];
      edges[idx] = edge;
      game.board.edges = edges;
    }
  }
</script>

<div class="editor">
  <div class="toolbar">
    <label>
      {t('editor.title')} <input type="text" bind:value={title} onchange={updateTitle} class="title-input" />
    </label>
    <span class="sep"></span>
    <label>
      {t('editor.cols')} <input type="number" bind:value={cols} onchange={updateCols} min="1" />
    </label>
    <label>
      {t('editor.rows')} <input type="number" bind:value={rows} onchange={updateRows} min="1" />
    </label>
    <label>
      {t('editor.cell')} <input type="number" bind:value={cellSize} onchange={updateCellSize} min="8" />
    </label>
    <span class="dim-hint">{boardWidth} x {boardHeight}</span>
    <span class="sep"></span>
    <label title={t('editor.diceCountTitle')}>
      {t('editor.dice')} <input type="number" bind:value={diceCount} onchange={updateDice} min="1" max="10" class="dice-num" />d
    </label>
    <input type="number" bind:value={diceSides} onchange={updateDice} min="2" max="100" class="dice-sides" />
    <span class="sep"></span>
    <label title={t('editor.startBonusTitle')}>
      {t('editor.bonus')}
      <select bind:value={startBonusResource} onchange={updateStartBonusResource} class="bonus-resource">
        <option value="">{t('editor.bonusNone')}</option>
        {#each Object.keys(game.rules.resources) as res}
          <option value={res}>{res}</option>
        {/each}
      </select>
    </label>
    <input type="number" bind:value={startBonus} onchange={updateStartBonus} min="0" class="bonus-amount" />
    <span class="sep"></span>
    <button onclick={addCell}>{t('editor.addCell')}</button>
    <button
      class={mode === 'connect' ? 'active' : ''}
      onclick={() => { mode = mode === 'connect' ? 'select' : 'connect'; selectedCellId = undefined; }}
    >
      {mode === 'connect' ? t('editor.exitConnect') : t('editor.connect')}
    </button>
  </div>

  <div class="editor-body">
    <BoardCanvas
      board={game.board}
      {selectedCellId}
      {selectedEdgeId}
      {mode}
      onCellSelect={handleCellSelect}
      onEdgeSelect={handleEdgeSelect}
      onCellMove={handleCellMove}
      onCanvasClick={handleCanvasClick}
      onAddEdge={handleAddEdge}
    />
    <CellInspector
      cell={selectedCell}
      edges={game.board.edges}
      rules={game.rules}
      onCellChange={handleCellChange}
      onDeleteCell={deleteCell}
      onDeleteEdge={deleteEdge}
      {selectedEdgeId}
      onEdgeSelect={handleEdgeSelect}
      onEdgeChange={handleEdgeChange}
    />
  </div>
</div>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    width: min(var(--page-full), 100%);
    margin: 0 auto;
    height: calc(100vh - 160px);
    min-height: 480px;
  }
  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-3);
    padding: var(--space-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }
  .toolbar label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--text-muted);
    font-size: var(--text-sm);
    white-space: nowrap;
  }
  .toolbar input {
    width: 4.5rem;
    padding: var(--space-2);
    background: var(--surface-sunken);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: var(--radius-sm);
    font: inherit;
    font-size: var(--text-sm);
  }
  .dim-hint {
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-family: var(--font-mono);
  }
  .sep {
    width: 1px;
    height: 20px;
    background: var(--border);
    margin: 0 4px;
  }
  .dice-num {
    width: 40px !important;
  }
  .dice-sides {
    width: 50px !important;
  }
  .title-input {
    width: min(14rem, 40vw) !important;
  }
  .bonus-resource {
    width: 80px;
    padding: 4px 6px;
    background: var(--surface-sunken);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 4px;
    font-size: 12px;
  }
  .bonus-amount {
    width: 60px !important;
  }
  .toolbar button {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-strong);
    background: var(--surface-raised);
    color: var(--text);
    border-radius: var(--radius-sm);
    cursor: pointer;
    font: inherit;
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    white-space: nowrap;
  }
  .toolbar button:hover {
    border-color: var(--accent);
    background: var(--accent-surface);
  }
  .toolbar button.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .editor-body {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 320px;
    gap: var(--space-3);
    flex: 1;
    min-height: 0;
  }

  /* Below this width the side-by-side canvas and inspector left the board a
     few pixels wide. Stack them and give the board a workable height instead. */
  @media (max-width: 900px) {
    .editor {
      height: auto;
    }
    .editor-body {
      grid-template-columns: 1fr;
    }
    .editor-body :global(.canvas-wrapper) {
      min-height: 55vh;
    }
    .sep {
      display: none;
    }
  }

  @media (max-width: 560px) {
    .toolbar {
      gap: var(--space-2);
    }
    .toolbar label {
      font-size: var(--text-xs);
    }
    .title-input {
      width: 100% !important;
    }
    .toolbar > label:first-of-type {
      width: 100%;
    }
  }
</style>
