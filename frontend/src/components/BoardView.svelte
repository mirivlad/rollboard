<script lang="ts">
  import type { Board, PlayerState, CellState } from '../lib/types';
  import CellView from './CellView.svelte';
  import EdgeLayer from './EdgeLayer.svelte';
  import TokenLayer from './TokenLayer.svelte';

  let { board, players, cellStates }: {
    board: Board;
    players: PlayerState[];
    cellStates: Record<string, CellState>;
  } = $props();
</script>

<div class="board-view">
  <div
    class="board-area"
    style="width: {board.width}px; height: {board.height}px;"
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
    />

    <!-- Cells -->
    {#each board.cells as cell (cell.id)}
      <div
        class="cell-wrapper"
        style="
          left: {cell.x}px;
          top: {cell.y}px;
          position: absolute;
        "
      >
        <CellView
          {cell}
          cellSize={board.cellSize}
          cellState={cellStates[cell.id]}
          {players}
        />
      </div>
    {/each}

    <!-- Tokens -->
    <TokenLayer
      {players}
      cells={board.cells}
      cellSize={board.cellSize}
    />
  </div>
</div>

<style>
  .board-view {
    overflow: auto;
    flex: 1;
  }
  .board-area {
    position: relative;
  }
  .grid-cell {
    position: absolute;
    border: 1px solid #1a2a4a;
    box-sizing: border-box;
  }
  .cell-wrapper {
    position: absolute;
  }
</style>
