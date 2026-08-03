<script lang="ts">
  import { i18n } from '../lib/i18n.svelte';
  import type { Board, CellDefinition, PlayerState, CellState } from '../lib/types';
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
  let t = $derived(i18n.t);

  let surfaceEl = $state<HTMLElement | null>(null);
  let connectFrom = $state<string | null>(null);
  let dragCellId = $state<string | null>(null);
  let dragOffsetX = $state(0);
  let dragOffsetY = $state(0);
  let hasDragged = $state(false);
  let pointerStart = $state<{ x: number; y: number } | null>(null);
  // Development aid, off by default: it used to open over the board.
  let debugVisible = $state(false);

  let mouseBoardX = $state(0);
  let mouseBoardY = $state(0);

  const DRAG_THRESHOLD = 4;

  let cols = $derived(Math.max(1, Math.floor(board.width / board.cellSize)));
  let rows = $derived(Math.max(1, Math.floor(board.height / board.cellSize)));

  function boardSurfaceWidth(): number { return cols * board.cellSize; }
  function boardSurfaceHeight(): number { return rows * board.cellSize; }

  function clientToBoard(clientX: number, clientY: number): { x: number; y: number } | null {
    if (!surfaceEl) return null;
    const rect = surfaceEl.getBoundingClientRect();
    const x = clientX - rect.left;
    const y = clientY - rect.top;
    if (x < 0 || y < 0 || x > boardSurfaceWidth() || y > boardSurfaceHeight()) return null;
    return { x, y };
  }

  function snapToGrid(value: number): number {
    return Math.round(value / board.cellSize) * board.cellSize;
  }

  function maxCellX(): number { return (cols - 1) * board.cellSize; }
  function maxCellY(): number { return (rows - 1) * board.cellSize; }

  function clampSnapped(x: number, y: number): { x: number; y: number } {
    const sx = snapToGrid(x);
    const sy = snapToGrid(y);
    return {
      x: Math.min(Math.max(0, sx), maxCellX()),
      y: Math.min(Math.max(0, sy), maxCellY()),
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

  function handleSurfaceClick(e: Event) {
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
      mouseBoardX = Math.min(pos.x, boardSurfaceWidth());
      mouseBoardY = Math.min(pos.y, boardSurfaceHeight());
    }

    if (!dragCellId || !pos || !pointerStart) return;

    const dx = Math.abs(pos.x - pointerStart.x);
    const dy = Math.abs(pos.y - pointerStart.y);
    if (!hasDragged && dx < DRAG_THRESHOLD && dy < DRAG_THRESHOLD) return;
    hasDragged = true;

    const { x, y } = clampSnapped(pos.x - dragOffsetX, pos.y - dragOffsetY);
    onCellMove?.(dragCellId, x, y);
  }

  function handlePointerUp() {
    dragCellId = null;
    pointerStart = null;
  }
</script>

<svelte:window onpointermove={handlePointerMove} onpointerup={handlePointerUp} />

<div class="canvas-wrapper">
  <div class="canvas-area" style="width: {boardSurfaceWidth()}px; height: {boardSurfaceHeight()}px;">
    <div
      class="board-surface"
      bind:this={surfaceEl}
      style="width: {boardSurfaceWidth()}px; height: {boardSurfaceHeight()}px;"
      onclick={handleSurfaceClick}
      onkeydown={(e) => e.key === 'Enter' && handleSurfaceClick(e)}
      role="button"
      tabindex="0"
    >
      <!-- Grid -->
      {#each Array(rows) as _, row}
        {#each Array(cols) as _, col}
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
          <div class="debug-title">{t('editor.debugTitle')}</div>
          <div class="debug-row">board cols/rows: <span class="debug-val">{cols} x {rows}</span></div>
          <div class="debug-row">board surface: <span class="debug-val">{boardSurfaceWidth()} x {boardSurfaceHeight()}</span></div>
          <div class="debug-row">selectedCellId: <span class="debug-val">{selectedCellId ?? 'null'}</span></div>
          <div class="debug-row">selectedEdgeId: <span class="debug-val">{selectedEdgeId ?? 'null'}</span></div>
          <div class="debug-row">mode: <span class="debug-val">{mode}</span></div>
          <div class="debug-row">dragCellId: <span class="debug-val">{dragCellId ?? 'null'}</span></div>
          <div class="debug-row">connectFrom: <span class="debug-val">{connectFrom ?? 'null'}</span></div>
          <div class="debug-row">hasDragged: <span class="debug-val">{String(hasDragged)}</span></div>
          <div class="debug-row">mouseBoard: <span class="debug-val">{mouseBoardX}, {mouseBoardY}</span></div>
        </div>
      {:else}
        <button class="debug-show" onclick={() => debugVisible = true}>{t('editor.debug')}</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .canvas-wrapper {
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-sunken);
    flex: 1;
  }
  .canvas-area {
    position: relative;
    user-select: none;
    -webkit-user-select: none;
  }
  .board-surface {
    position: relative;
  }
  .grid-cell {
    position: absolute;
    border: 1px solid var(--accent-surface);
    box-sizing: border-box;
  }
  .connect-hint {
    position: absolute;
    bottom: 8px;
    left: 8px;
    background: var(--accent);
    color: var(--accent-contrast);
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
    border: 1px solid var(--danger);
    border-radius: 4px;
    padding: 8px;
    font-size: 10px;
    font-family: monospace;
    color: var(--text-muted);
    z-index: 200;
    min-width: 180px;
  }
  .debug-title {
    color: var(--danger);
    font-weight: bold;
    margin-bottom: 4px;
    font-size: 11px;
  }
  .debug-row {
    margin: 2px 0;
  }
  .debug-val {
    color: var(--accent);
  }
  .debug-close {
    float: right;
    background: none;
    border: none;
    color: var(--danger);
    cursor: pointer;
    font-size: 12px;
    padding: 0 2px;
  }
  .debug-show {
    position: absolute;
    top: 4px;
    right: 4px;
    background: rgba(0,0,0,0.7);
    border: 1px solid var(--border-strong);
    color: var(--text-faint);
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 10px;
    cursor: pointer;
    z-index: 200;
  }
</style>
