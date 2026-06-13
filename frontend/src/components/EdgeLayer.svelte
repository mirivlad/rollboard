<script lang="ts">
  import type { EdgeDefinition, CellDefinition } from '../lib/types';

  let { edges, cells, selectedEdgeId, onSelect }: {
    edges: EdgeDefinition[];
    cells: CellDefinition[];
    selectedEdgeId?: string;
    onSelect?: (id: string) => void;
  } = $props();

  let cellMap = $derived(new Map(cells.map(c => [c.id, c])));

  function getArrowPath(edge: EdgeDefinition): string {
    const from = cellMap.get(edge.from);
    const to = cellMap.get(edge.to);
    if (!from || !to) return '';

    const cx = 96 / 2;
    const cy = 96 / 2;
    const x1 = from.x + cx;
    const y1 = from.y + cy;
    const x2 = to.x + cx;
    const y2 = to.y + cy;

    const dx = x2 - x1;
    const dy = y2 - y1;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (dist === 0) return '';

    const ux = dx / dist;
    const uy = dy / dist;

    const nx = -uy;
    const ny = ux;

    const r = 48;
    const arrowLen = 12;
    const arrowWidth = 6;

    const sx = x1 + ux * r;
    const sy = y1 + uy * r;
    const ex = x2 - ux * r;
    const ey = y2 - uy * r;

    const ax1 = ex - ux * arrowLen + nx * arrowWidth;
    const ay1 = ey - uy * arrowLen + ny * arrowWidth;
    const ax2 = ex - ux * arrowLen - nx * arrowWidth;
    const ay2 = ey - uy * arrowLen - ny * arrowWidth;

    return `M${sx},${sy} L${ex},${ey} M${ax1},${ay1} L${ex},${ey} L${ax2},${ay2}`;
  }
</script>

<svg class="edge-layer" width="100%" height="100%">
  {#each edges as edge}
    <path
      d={getArrowPath(edge)}
      class:selected={edge.id === selectedEdgeId}
      stroke={edge.id === selectedEdgeId ? '#e94560' : '#666'}
      stroke-width={edge.id === selectedEdgeId ? 3 : 2}
      fill="none"
      onclick={() => onSelect?.(edge.id)}
      onkeydown={(e) => e.key === 'Enter' && onSelect?.(edge.id)}
      role="button"
      tabindex="0"
      style="cursor: pointer;"
    />
  {/each}
</svg>

<style>
  .edge-layer {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: all;
  }
  path {
    marker-end: none;
  }
</style>
