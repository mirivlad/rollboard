<script lang="ts">
  import type { PlayerState, CellDefinition } from '../lib/types';

  let { players, cells, cellSize }: {
    players: PlayerState[];
    cells: CellDefinition[];
    cellSize: number;
  } = $props();

  let cellMap = $derived(new Map(cells.map(c => [c.id, c])));

  let tokensByCell = $derived(() => {
    const map = new Map<string, PlayerState[]>();
    for (const p of players) {
      if (p.bankrupt) continue;
      const existing = map.get(p.positionCellId) || [];
      existing.push(p);
      map.set(p.positionCellId, existing);
    }
    return map;
  });
</script>

{#each [...tokensByCell().entries()] as [cellId, cellPlayers]}
  {@const cell = cellMap.get(cellId)}
  {#if cell}
    {#if cellPlayers.length === 1}
      <div
        class="token single"
        style="
          left: {cell.x + cellSize / 2}px;
          top: {cell.y + cellSize / 2}px;
          background: {cellPlayers[0].color};
        "
        title={cellPlayers[0].name}
      ></div>
    {:else}
      {#each cellPlayers as player, i}
        {@const angle = (i / cellPlayers.length) * 360}
        {@const r = Math.min(cellSize * 0.25, 16)}
        <div
          class="token stacked"
          style="
            left: {cell.x + cellSize / 2 + Math.cos(angle * Math.PI / 180) * r}px;
            top: {cell.y + cellSize / 2 + Math.sin(angle * Math.PI / 180) * r}px;
            background: {player.color};
          "
          title={player.name}
        ></div>
      {/each}
    {/if}
  {/if}
{/each}

<style>
  .token {
    position: absolute;
    border-radius: 50%;
    border: 2px solid white;
    box-shadow: 0 1px 4px rgba(0,0,0,0.5);
    z-index: 10;
    pointer-events: none;
    transform: translate(-50%, -50%);
    transition: left 0.3s ease, top 0.3s ease;
  }
  .token.single {
    width: 20px;
    height: 20px;
  }
  .token.stacked {
    width: 14px;
    height: 14px;
  }
</style>
