<script lang="ts">
  import type { Board, CellDefinition, EdgeDefinition, PlayerState, CellState } from '../lib/types';
  import CellView from './CellView.svelte';
  import EdgeLayer from './EdgeLayer.svelte';

  let { board, players, cellStates, selectedCellId, selectedEdgeId, mode, onCellSelect, onEdgeSelect, onCellMove, onCanvasClick, onAddEdge }: {
    board: Board;
    players?: PlayerState[];
    cellStates?: Record<string, CellState>;
    selectedCellId?: string;
    selectedEdgeId?: string;
    mode: 'select' | 'connect';
    onCellSelect?: (id: string) => void;
    onEdgeSelect?: (id: string) => void;
    onCellMove?: (id: string, x: number, y: number) => void;
    onCanvasClick?: (x: number, y: number) => void;
    onAddEdge?: (from: string, to: string) => void;
  } = $props();

  let connectFrom = $state<string | null>(null);
  let dragCellId = $state<string | null>(null);
  let dragOffsetX = $state(0);
  let dragOffsetY = $state(0);

  function handleCanvasClick(e: MouseEvent) {
    if (mode === 'select') {
      const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
      const x = Math.round((e.clientX - rect.left) / board.cellSize) * board.cellSize;
      const y = Math.round((e.clientY - rect.top) / board.cellSize) * board.cellSize;
      onCanvasClick?.(x, y);
    }
  }

  function handleCellClick(id: string, e: MouseEvent) {
    if (mode === 'connect') {
      if (connectFrom === null) {
        connectFrom = id;
      } else {
        if (connectFrom !== id) {
          onAddEdge?.(connectFrom, id);
        }
        connectFrom = null;
      }
      return;
    }
    onCellSelect?.(id);
  }

  function handleCellMouseDown(id: string, e: MouseEvent) {
    if (mode !== 'select') return;
    const cell = board.cells.find(c => c.id === id);
    if (!cell) return;
    dragCellId = id;
    dragOffsetX = e.clientX - cell.x;
    dragOffsetY = e.clientY - cell.y;
  }

  function handleMouseMove(e: MouseEvent) {
    if (!dragCellId) return;
    const rect = document.querySelector('.canvas-area')?.getBoundingClientRect();
    if (!rect) return;
    let x = Math.round((e.clientX - rect.left) / board.cellSize) * board.cellSize;
    let y = Math.round((e.clientY - rect.top) / board.cellSize) * board.cellSize;
    x = Math.max(0, Math.min(x, board.width - board.cellSize));
    y = Math.max(0, Math.min(y, board.height - board.cellSize));
    onCellMove?.(dragCellId, x, y);
  }

  function handleMouseUp() {
    dragCellId = null;
  }
</script>

<svelte:window onmousemove={handleMouseMove} onmouseup={handleMouseUp} />

<div class="canvas-wrapper">
  <div
    class="canvas-area"
    style="width: {board.width}px; height: {board.height}px;"
    onclick={handleCanvasClick}
    onkeydown={(e) => e.key === 'Enter' && handleCanvasClick(e as unknown as MouseEvent)}
    role="button"
    tabindex="0"
  >
    <!-- Grid -->
    {#each Array(Math.ceil(board.height / board.cellSize)) as _, row}
      {#each Array(Math.ceil(board.width / board.cellSize)) as _, col}
        <div
          class="grid-cell"
          style="
            left: {col * board.cellSize}px;
            top: {row * board.cellSize}px;
            width: {board.cellSize}px;
            height: {board.cellSize}px;
          "
        ></div>
      {/each}
    {/each}

    <!-- Edges -->
    <EdgeLayer
      edges={board.edges}
      cells={board.cells}
      {selectedEdgeId}
      onSelect={onEdgeSelect}
    />

    <!-- Cells -->
    {#each board.cells as cell (cell.id)}
      <div
        style="position: absolute;"
        onmousedown={(e) => handleCellMouseDown(cell.id, e)}
        role="button"
        tabindex="0"
      >
        <CellView
          cell={cell}
          cellState={cellStates?.[cell.id]}
          {players}
          isSelected={cell.id === selectedCellId}
        />
      </div>
    {/each}

    <!-- Connect mode hint -->
    {#if mode === 'connect' && connectFrom}
      <div class="connect-hint">
        Selected {connectFrom} — click another cell to connect
      </div>
    {/if}
  </div>
</div>

<style>
  .canvas-wrapper {
    overflow: auto;
    border: 1px solid #0f3460;
    border-radius: 8px;
    background: #0d1b2a;
    flex: 1;
  }
  .canvas-area {
    position: relative;
    cursor: crosshair;
  }
  .grid-cell {
    position: absolute;
    border: 1px solid #1a2a4a;
    box-sizing: border-box;
  }
  .connect-hint {
    position: absolute;
    bottom: 8px;
    left: 8px;
    background: #e94560;
    color: white;
    padding: 4px 12px;
    border-radius: 4px;
    font-size: 12px;
    z-index: 10;
  }
</style>
