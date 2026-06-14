<script lang="ts">
  import type { EdgeDefinition, CellDefinition } from '../lib/types';

  let { edges, cells, cellSize, selectedEdgeId, onSelect }: {
    edges: EdgeDefinition[];
    cells: CellDefinition[];
    cellSize: number;
    selectedEdgeId?: string;
    onSelect?: (id: string) => void;
  } = $props();

  let cellMap = $derived(new Map(cells.map(c => [c.id, c])));

  function edgePoints(edge: EdgeDefinition): { x1: number; y1: number; x2: number; y2: number } | null {
    const from = cellMap.get(edge.from);
    const to = cellMap.get(edge.to);
    if (!from || !to) return null;
    const half = cellSize / 2;
    return {
      x1: from.x + half,
      y1: from.y + half,
      x2: to.x + half,
      y2: to.y + half,
    };
  }

  function handleClick(id: string, e: Event) {
    e.stopPropagation();
    onSelect?.(id);
  }
</script>

<svg
  class="edge-layer"
  width="100%"
  height="100%"
>
  {#each edges as edge (edge.id)}
    {@const pts = edgePoints(edge)}
    {#if pts}
      <!-- Invisible wider path for click detection -->
      <path
        d="M {pts.x1} {pts.y1} L {pts.x2} {pts.y2}"
        stroke="transparent"
        stroke-width={Math.max(cellSize * 0.5, 12)}
        fill="none"
        style="cursor: pointer;"
        onclick={(e) => handleClick(edge.id, e)}
        onkeydown={(e) => e.key === 'Enter' && handleClick(edge.id, e)}
        role="button"
        tabindex="0"
      />
      <!-- Visible arrow -->
      <line
        x1={pts.x1}
        y1={pts.y1}
        x2={pts.x2}
        y2={pts.y2}
        stroke={edge.id === selectedEdgeId ? '#e94560' : '#4fc3f7'}
        stroke-width="2"
        marker-end="url(#arrowhead)"
        style="pointer-events: none;"
      />
    {/if}
  {/each}
  <defs>
    <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
      <polygon points="0 0, 10 3.5, 0 7" fill="#4fc3f7" />
    </marker>
  </defs>
</svg>

<style>
  .edge-layer {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
    z-index: 1;
  }
  .edge-layer line, .edge-layer path {
    pointer-events: stroke;
  }
</style>
