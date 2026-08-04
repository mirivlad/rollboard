<script lang="ts">
  import type { EdgeDefinition, CellDefinition, EdgeCondition } from '../lib/types';

  let { edges, cells, cellSize, selectedEdgeId, onSelect }: {
    edges: EdgeDefinition[];
    cells: CellDefinition[];
    cellSize: number;
    selectedEdgeId?: string;
    onSelect?: (id: string) => void;
  } = $props();

  import { TILE_INSET } from '../lib/board-layout';

  let cellMap = $derived(new Map(cells.map(c => [c.id, c])));

  function condLabel(cond: EdgeCondition): string {
    switch (cond.type) {
      case 'always': return '';
      case 'dice_total_even': return 'even';
      case 'dice_total_odd': return 'odd';
      case 'dice_total_in': return `in[${(cond.values ?? []).join(',')}]`;
      case 'player_resource_at_least': return `${cond.resource}≥${cond.amount ?? 1}`;
      case 'manual_choice': return cond.label ?? 'choice';
      case 'pay_resource': return `pay ${cond.amount ?? 1} ${cond.resource ?? '?'}`;
      default: return cond.type;
    }
  }

  /** Distance from a tile centre to its border along a direction. */
  function borderOffset(dx: number, dy: number, half: number): number {
    // The line leaves a square, not a circle: whichever axis reaches the edge
    // first decides. For an axis-aligned neighbour this is exactly `half`;
    // for a diagonal it is longer, which is what keeps the arrow touching the
    // corner rather than floating away from it.
    const scaleX = dx === 0 ? Infinity : half / Math.abs(dx);
    const scaleY = dy === 0 ? Infinity : half / Math.abs(dy);
    return Math.min(scaleX, scaleY);
  }

  /**
   * Endpoints trimmed to the tiles' borders rather than their centres.
   *
   * Centre-to-centre lines are drawn straight across whatever they connect,
   * so a path looked like it was scribbled over the board instead of joining
   * squares together. This leaves only the gap between two tiles drawn.
   */
  function edgePoints(edge: EdgeDefinition): { x1: number; y1: number; x2: number; y2: number } | null {
    const from = cellMap.get(edge.from);
    const to = cellMap.get(edge.to);
    if (!from || !to) return null;

    // Stop at the drawn tile edge, not the grid slot edge, so the arrow lands
    // in the gutter between neighbours.
    const half = cellSize / 2 - TILE_INSET;
    const slotHalf = cellSize / 2;
    const cx1 = from.x + slotHalf;
    const cy1 = from.y + slotHalf;
    const cx2 = to.x + slotHalf;
    const cy2 = to.y + slotHalf;

    const dx = cx2 - cx1;
    const dy = cy2 - cy1;
    const length = Math.hypot(dx, dy);
    if (length === 0) return null;

    const startScale = borderOffset(dx, dy, half);
    const endScale = borderOffset(dx, dy, half);

    return {
      x1: cx1 + dx * startScale,
      y1: cy1 + dy * startScale,
      x2: cx2 - dx * endScale,
      y2: cy2 - dy * endScale,
    };
  }

  /** Neighbouring tiles leave almost no room, so their labels are dropped. */
  function hasRoomForLabel(points: { x1: number; y1: number; x2: number; y2: number }): boolean {
    return Math.hypot(points.x2 - points.x1, points.y2 - points.y1) > 28;
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
        class="edge-line"
        class:selected={edge.id === selectedEdgeId}
        stroke-width="2"
        marker-end="url(#arrowhead)"
      />
      <!-- Condition label -->
      {#if condLabel(edge.condition) && hasRoomForLabel(pts)}
        <text
          x={(pts.x1 + pts.x2) / 2}
          y={(pts.y1 + pts.y2) / 2 - 6}
          class="edge-label"
          text-anchor="middle"
          font-size="11"
        >
          {condLabel(edge.condition)}
        </text>
      {/if}
    {/if}
  {/each}
  <defs>
    <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
      <polygon class="edge-arrow" points="0 0, 10 3.5, 0 7" />
    </marker>
  </defs>
</svg>

<style>
  .edge-layer {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
    /* Below the tiles. The cells carried no z-index of their own, so this
       layer used to win and every path was drawn across the board. */
    z-index: 1;
  }
  .edge-layer line, .edge-layer path {
    pointer-events: stroke;
  }
  .edge-line {
    stroke: var(--accent);
    stroke-linecap: round;
    pointer-events: none;
  }
  .edge-line.selected {
    stroke: var(--danger);
  }
  .edge-arrow {
    fill: var(--accent);
  }
  .edge-label {
    fill: var(--text-faint);
    font-family: var(--font-mono);
    pointer-events: none;
  }
</style>
