<script lang="ts">
  import { HIDDEN_CELL_TYPE } from '../lib/types';
  import { tileRect } from '../lib/board-layout';
  import { i18n } from '../lib/i18n.svelte';
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

  let t = $derived(i18n.t);
  let isHidden = $derived(cell.type === HIDDEN_CELL_TYPE);
  let rect = $derived(tileRect(cell.x, cell.y, cellSize));
</script>

<div
  class="cell"
  class:selected={isSelected}
  class:hidden-cell={isHidden}
  style="
    left: {rect.left}px;
    top: {rect.top}px;
    width: {rect.size}px;
    height: {rect.size}px;
    background-color: {isHidden ? '' : cell.visual.baseColor || '#eee'};
    background-image: {cell.visual.baseImage ? `url(${cell.visual.baseImage})` : 'none'};
    background-size: cover;
  "
>
  {#if isHidden}
    <!-- The server never sends what is under a face-down cell, so there is
         nothing here to reveal by reading the page. -->
    <div class="cell-back" aria-label={t('rpg.unexplored')}>?</div>
  {:else}
    <div class="cell-content">
      <span class="cell-type">{cell.type}</span>
      <span class="cell-title">{cell.title}</span>
    </div>
  {/if}
  {#if owner}
    <div class="owner-stripe" style="background-color: {owner.color}"></div>
  {/if}
</div>

<style>
  .hidden-cell {
    background-image: repeating-linear-gradient(
      45deg,
      var(--surface-raised),
      var(--surface-raised) 6px,
      var(--surface-sunken) 6px,
      var(--surface-sunken) 12px
    ) !important;
    border-color: var(--border-strong);
  }
  .cell-back {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-faint);
    font-size: 1.75rem;
    font-weight: var(--weight-black);
  }
  .cell {
    position: absolute;
    /* Above the edge layer, which is z-index 1. Without this the tiles had no
       stacking order at all and the paths were drawn over them. */
    z-index: 2;
    border: 2px solid rgba(0, 0, 0, 0.35);
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
    border-color: var(--danger);
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
