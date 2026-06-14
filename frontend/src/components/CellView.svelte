<script lang="ts">
  import type { CellDefinition, CellState, PlayerState } from '../lib/types';

  let { cell, cellSize, cellState, players, isSelected }: {
    cell: CellDefinition;
    cellSize: number;
    cellState?: CellState;
    players?: PlayerState[];
    isSelected?: boolean;
  } = $props();

  let owner = $derived(
    cellState?.ownerPlayerId
      ? (players || []).find(p => p.id === cellState!.ownerPlayerId)
      : undefined
  );
</script>

<div
  class="cell"
  class:selected={isSelected}
  style="
    left: {cell.x}px;
    top: {cell.y}px;
    width: {cellSize}px;
    height: {cellSize}px;
    background-color: {cell.visual.baseColor || '#eee'};
    background-image: {cell.visual.baseImage ? `url(${cell.visual.baseImage})` : 'none'};
    background-size: cover;
  "
>
  <div class="cell-content">
    <span class="cell-type">{cell.type}</span>
    <span class="cell-title">{cell.title}</span>
  </div>
  {#if owner}
    <div class="owner-stripe" style="background-color: {owner.color}"></div>
  {/if}
</div>

<style>
  .cell {
    position: absolute;
    border: 2px solid #333;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    overflow: hidden;
    box-sizing: border-box;
    font-size: 10px;
    text-align: center;
    transition: box-shadow 0.15s;
  }
  .cell:hover {
    box-shadow: 0 0 8px rgba(233, 69, 96, 0.5);
  }
  .cell.selected {
    border-color: #e94560;
    box-shadow: 0 0 12px rgba(233, 69, 96, 0.7);
  }
  .cell-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    z-index: 1;
    padding: 4px;
    text-shadow: 0 0 3px rgba(0,0,0,0.5);
    color: #111;
  }
  .cell-type {
    font-size: 9px;
    background: rgba(0,0,0,0.5);
    color: white;
    padding: 1px 4px;
    border-radius: 3px;
  }
  .cell-title {
    font-weight: 600;
    font-size: 10px;
  }
  .owner-stripe {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 6px;
  }
</style>
