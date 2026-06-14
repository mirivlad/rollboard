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
  let boardWidth = $state(game.board.width);
  let boardHeight = $state(game.board.height);
  let cellSize = $state(game.board.cellSize);

  let selectedCell = $derived(
    game.board.cells.find(c => c.id === selectedCellId)
  );

  function addCell() {
    const id = `cell_${game.board.cells.length + 1}`;
    const x = game.board.cells.length * 100;
    const y = 200;
    const newCell: CellDefinition = {
      id,
      title: `Cell ${game.board.cells.length + 1}`,
      type: Object.keys(game.rules.cellTypes)[0] || 'empty',
      x: x - (x % cellSize),
      y: y - (y % cellSize),
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
    // Don't place cell on click, let user use Add Cell button
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

  function updateBoardDimensions() {
    game.board.width = boardWidth;
    game.board.height = boardHeight;
    game.board.cellSize = cellSize;
  }
</script>

<div class="editor">
  <div class="toolbar">
    <label>
      W: <input type="number" bind:value={boardWidth} onchange={updateBoardDimensions} />
    </label>
    <label>
      H: <input type="number" bind:value={boardHeight} onchange={updateBoardDimensions} />
    </label>
    <label>
      Cell: <input type="number" bind:value={cellSize} onchange={updateBoardDimensions} />
    </label>
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
