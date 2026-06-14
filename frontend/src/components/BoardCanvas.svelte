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
    onCellSelect?: (id: string | undefined) => void;
    onEdgeSelect?: (id: string | undefined) => void;
    onCellMove?: (id: string, x: number, y: number) => void;
    onCanvasClick?: (x: number, y: number) => void;
    onAddEdge?: (from: string, to: string) => void;
  } = $props();

  let canvasEl = $state<HTMLElement | null>(null);
  let connectFrom = $state<string | null>(null);
  let dragCellId = $state<string | null>(null);
  let dragOffsetX = $state(0);
  let dragOffsetY = $state(0);
  let hasDragged = $state(false);
  let pointerStart = $state<{ x: number; y: number } | null>(null);
  let debugVisible = $state(true);

  let mouseBoardX = $state(0);
  let mouseBoardY = $state(0);

  const DRAG_THRESHOLD = 4;

  function clientToBoard(clientX: number, clientY: number): { x: number; y: number } | null {
    if (!canvasEl) return null;
    const rect = canvasEl.getBoundingClientRect();
    return {
      x: clientX - rect.left,
      y: clientY - rect.top,
    };
  }

  function snap(value: number): number {
    return Math.round(value / board.cellSize) * board.cellSize;
  }

  function clampCellPosition(x: number, y: number): { x: number; y: number } {
    const maxX = Math.max(0, board.width - board.cellSize);
    const maxY = Math.max(0, board.height - board.cellSize);
    return {
      x: Math.min(Math.max(0, x), maxX),
      y: Math.min(Math.max(0, y), maxY),
    };
  }

  function handleCellPointerDown(id: string, e: PointerEvent) {
    e.stopPropagation();
    e.preventDefault();
    const cell = board.cells.find(c => c.id === id);
    if (!cell) return;

    if (mode === 'connect') return;

    const pos = clientToBoard(e.clientX, e.clientY);
    if (!pos) return;

    dragCellId = id;
    dragOffsetX = pos.x - cell.x;
    dragOffsetY = pos.y - cell.y;
    hasDragged = false;
    pointerStart = pos;
  }

  function handleCellClick(id: string, e: Event) {
    e.stopPropagation();
    if (hasDragged) return;

    if (mode === 'connect') {
      if (connectFrom === null) {
        connectFrom = id;
      } else {
        if (connectFrom !== id) {
          const dup = board.edges.some(ed => ed.from === connectFrom && ed.to === id);
          if (!dup) {
            onAddEdge?.(connectFrom, id);
          }
        }
        connectFrom = null;
      }
      return;
    }

    onCellSelect?.(id);
    if (selectedEdgeId) onEdgeSelect?.(undefined);
  }

  function handleCanvasClick(e: Event) {
    if (hasDragged) return;

    if (mode === 'connect') {
      connectFrom = null;
      return;
    }
    onCellSelect?.(undefined);
    onEdgeSelect?.(undefined);
  }

  function handlePointerMove(e: PointerEvent) {
    const pos = clientToBoard(e.clientX, e.clientY);
    if (pos) {
      mouseBoardX = pos.x;
      mouseBoardY = pos.y;
    }

    if (!dragCellId || !pos || !pointerStart) return;

    const dx = Math.abs(pos.x - pointerStart.x);
    const dy = Math.abs(pos.y - pointerStart.y);
    if (!hasDragged && dx < DRAG_THRESHOLD && dy < DRAG_THRESHOLD) return;
    hasDragged = true;

    let x = snap(pos.x - dragOffsetX);
    let y = snap(pos.y - dragOffsetY);
    ({ x, y } = clampCellPosition(x, y));
    onCellMove?.(dragCellId, x, y);
  }

  function handlePointerUp() {
    dragCellId = null;
    pointerStart = null;
  }
</script>

<svelte:window onpointermove={handlePointerMove} onpointerup={handlePointerUp} />

<div class="canvas-wrapper">
  <div
    class="canvas-area"
    bind:this={canvasEl}
    style="width: {board.width}px; height: {board.height}px;"
    onclick={handleCanvasClick}
    onkeydown={(e) => e.key === 'Enter' && handleCanvasClick(e)}
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
      cellSize={board.cellSize}
      {selectedEdgeId}
      onSelect={onEdgeSelect}
    />

    <!-- Cells -->
    {#each board.cells as cell (cell.id)}
      <div
        style="position: absolute;"
        onpointerdown={(e) => handleCellPointerDown(cell.id, e)}
        onclick={(e) => handleCellClick(cell.id, e)}
        onkeydown={(e) => e.key === 'Enter' && handleCellClick(cell.id, e)}
        role="button"
        tabindex="0"
      >
        <CellView
          {cell}
          cellSize={board.cellSize}
          cellState={cellStates?.[cell.id]}
          {players}
          isSelected={cell.id === selectedCellId}
        />
      </div>
    {/each}

    <!-- Connect mode hint -->
    {#if mode === 'connect' && connectFrom}
      <div class="connect-hint">
        Source: {connectFrom} — click another cell to connect
      </div>
    {/if}

    <!-- Debug panel -->
    {#if debugVisible}
      <div class="debug-panel" role="none" onpointerdown={(e) => e.stopPropagation()}>
        <button class="debug-close" onclick={() => debugVisible = false}>✕</button>
        <div class="debug-title">Editor Debug</div>
        <div class="debug-row">selectedCellId: <span class="debug-val">{selectedCellId ?? 'null'}</span></div>
        <div class="debug-row">selectedEdgeId: <span class="debug-val">{selectedEdgeId ?? 'null'}</span></div>
        <div class="debug-row">mode: <span class="debug-val">{mode}</span></div>
        <div class="debug-row">dragCellId: <span class="debug-val">{dragCellId ?? 'null'}</span></div>
        <div class="debug-row">connectFrom: <span class="debug-val">{connectFrom ?? 'null'}</span></div>
        <div class="debug-row">hasDragged: <span class="debug-val">{String(hasDragged)}</span></div>
        <div class="debug-row">mouseBoard: <span class="debug-val">{mouseBoardX}, {mouseBoardY}</span></div>
      </div>
    {:else}
      <button class="debug-show" onclick={() => debugVisible = true}>Debug</button>
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
    user-select: none;
    -webkit-user-select: none;
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
    z-index: 100;
    pointer-events: none;
  }
  .debug-panel {
    position: absolute;
    top: 4px;
    right: 4px;
    background: rgba(0,0,0,0.85);
    border: 1px solid #e94560;
    border-radius: 4px;
    padding: 8px;
    font-size: 10px;
    font-family: monospace;
    color: #ccc;
    z-index: 200;
    min-width: 180px;
  }
  .debug-title {
    color: #e94560;
    font-weight: bold;
    margin-bottom: 4px;
    font-size: 11px;
  }
  .debug-row {
    margin: 2px 0;
  }
  .debug-val {
    color: #4fc3f7;
  }
  .debug-close {
    float: right;
    background: none;
    border: none;
    color: #e94560;
    cursor: pointer;
    font-size: 12px;
    padding: 0 2px;
  }
  .debug-show {
    position: absolute;
    top: 4px;
    right: 4px;
    background: rgba(0,0,0,0.7);
    border: 1px solid #555;
    color: #aaa;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 10px;
    cursor: pointer;
    z-index: 200;
  }
</style>
