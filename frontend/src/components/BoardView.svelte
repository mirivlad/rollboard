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

    <!-- Cells.
         CellView places itself at the cell's own coordinates, so it goes
         straight into the board area. Wrapping it in a second positioned
         element offset by the same coordinates put every tile at twice its
         distance from the origin: the grid, the tokens and the edges all agreed
         with each other and the tiles alone drifted off to the right. -->
    {#each board.cells as cell (cell.id)}
      <CellView
        {cell}
        cellSize={board.cellSize}
        cellState={cellStates[cell.id]}
        {players}
      />
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
    display: flex;
    justify-content: center;
    align-items: flex-start;
    flex: 1;
    padding: 16px;
    min-height: 300px;
  }
  .board-area {
    position: relative;
  }
  .grid-cell {
    position: absolute;
    border: 1px solid var(--accent-surface);
    box-sizing: border-box;
  }
</style>
