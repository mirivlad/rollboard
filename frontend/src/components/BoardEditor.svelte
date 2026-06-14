<script lang="ts">
  import type { GameDefinition, CellDefinition } from '../lib/types';
  import BoardCanvas from './BoardCanvas.svelte';
  import CellInspector from './CellInspector.svelte';

  let { game, onsave }: {
    game: GameDefinition;
    onsave?: () => void;
  } = $props();

  let selectedCellId = $state<string | undefined>();
  let selectedEdgeId = $state<string | undefined>();
  let mode = $state<'select' | 'connect'>('select');

  let cols = $state(Math.max(1, Math.floor(game.board.width / game.board.cellSize)));
  let rows = $state(Math.max(1, Math.floor(game.board.height / game.board.cellSize)));
  let cellSize = $state(game.board.cellSize);

  let boardWidth = $derived(cols * cellSize);
  let boardHeight = $derived(rows * cellSize);

  let diceCount = $state(game.rules.dice.count);
  let diceSides = $state(game.rules.dice.sides);

  let selectedCell = $derived(
    game.board.cells.find(c => c.id === selectedCellId)
  );

  // Sync editor fields when switching to a different game
  let lastGameId = $state<string>(game.id);
  let title = $state(game.title);
  $effect(() => {
    if (game.id !== lastGameId) {
      cols = Math.max(1, Math.floor(game.board.width / game.board.cellSize));
      rows = Math.max(1, Math.floor(game.board.height / game.board.cellSize));
      cellSize = game.board.cellSize;
      diceCount = game.rules.dice.count;
      diceSides = game.rules.dice.sides;
      // Sync game title
      title = game.title;
      // Sync start bonus
      startBonus = game.rules.startBonus ?? 0;
      startBonusResource = game.rules.startBonusResource ?? '';
      lastGameId = game.id;
      // Normalize board dimensions on load:
      // ensure game.board.width/height match cols*cellSize, rows*cellSize
      game.board.width = cols * cellSize;
      game.board.height = rows * cellSize;
    }
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

  // Start bonus state
  let startBonus = $state(game.rules.startBonus ?? 0);
  let startBonusResource = $state(game.rules.startBonusResource ?? '');

  // Start bonus sync: called when values change
  function updateStartBonus() {
    if (startBonus < 0) startBonus = 0;
    game.rules.startBonus = startBonus;
  }

  function updateStartBonusResource() {
    game.rules.startBonusResource = startBonusResource;
  }

  function updateTitle() {
    if (title.trim() === '') title = 'Untitled Game';
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
      title: `Cell ${game.board.cells.length + 1}`,
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
</script>

<div class="editor">
  <div class="toolbar">
    <label>
      Title: <input type="text" bind:value={title} onchange={updateTitle} class="title-input" />
    </label>
    <span class="sep"></span>
    <label>
      Cols: <input type="number" bind:value={cols} onchange={updateCols} min="1" />
    </label>
    <label>
      Rows: <input type="number" bind:value={rows} onchange={updateRows} min="1" />
    </label>
    <label>
      Cell: <input type="number" bind:value={cellSize} onchange={updateCellSize} min="8" />
    </label>
    <span class="dim-hint">{boardWidth} x {boardHeight}</span>
    <span class="sep"></span>
    <label title="Dice count">
      Dice: <input type="number" bind:value={diceCount} onchange={updateDice} min="1" max="10" class="dice-num" />d
    </label>
    <input type="number" bind:value={diceSides} onchange={updateDice} min="2" max="100" class="dice-sides" />
    <span class="sep"></span>
    <label title="Start bonus resource">
      Bonus:
      <select bind:value={startBonusResource} onchange={updateStartBonusResource} class="bonus-resource">
        <option value="">none</option>
        {#each Object.keys(game.rules.resources) as res}
          <option value={res}>{res}</option>
        {/each}
      </select>
    </label>
    <input type="number" bind:value={startBonus} onchange={updateStartBonus} min="0" class="bonus-amount" />
    <span class="sep"></span>
    <button onclick={addCell}>+ Add Cell</button>
    <button
      class={mode === 'connect' ? 'active' : ''}
      onclick={() => { mode = mode === 'connect' ? 'select' : 'connect'; selectedCellId = undefined; }}
    >
      {mode === 'connect' ? 'Exit Connect' : 'Connect Cells'}
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
    />
  </div>
</div>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: 12px;
    height: calc(100vh - 120px);
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
  }
  .toolbar label {
    display: flex;
    align-items: center;
    gap: 4px;
    color: #aaa;
    font-size: 12px;
  }
  .toolbar input {
    width: 60px;
    padding: 4px 6px;
    background: #0d1b2a;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
    font-size: 13px;
  }
  .dim-hint {
    color: #4fc3f7;
    font-size: 11px;
    font-family: monospace;
  }
  .sep {
    width: 1px;
    height: 20px;
    background: #0f3460;
    margin: 0 4px;
  }
  .dice-num {
    width: 40px !important;
  }
  .dice-sides {
    width: 50px !important;
  }
  .title-input {
    width: 160px !important;
  }
  .bonus-resource {
    width: 80px;
    padding: 4px 6px;
    background: #0d1b2a;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
    font-size: 12px;
  }
  .bonus-amount {
    width: 60px !important;
  }
  .toolbar button {
    padding: 6px 12px;
    border: 1px solid #0f3460;
    background: #0d1b2a;
    color: #e0e0e0;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
  }
  .toolbar button:hover {
    background: #1a2a4a;
  }
  .toolbar button.active {
    background: #e94560;
    border-color: #e94560;
    color: white;
  }
  .editor-body {
    display: flex;
    gap: 12px;
    flex: 1;
    min-height: 0;
  }
</style>
