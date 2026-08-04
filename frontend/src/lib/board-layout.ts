/**
 * Tiles are drawn slightly smaller than the grid slot they occupy, leaving a
 * gutter between neighbours.
 *
 * Without it there is nowhere to draw a connection: adjacent tiles touch, so a
 * path either runs across them (which reads as scribbling over the board) or
 * has zero length and disappears. The gutter is where arrows live.
 *
 * This is presentation only. A cell's stored x/y and the board geometry are
 * untouched, so the grid, the engine and saved games are unaffected.
 */
export const TILE_INSET = 6;

/** The drawn rectangle for a cell that occupies a grid slot at (x, y). */
export function tileRect(x: number, y: number, cellSize: number) {
  return {
    left: x + TILE_INSET,
    top: y + TILE_INSET,
    size: Math.max(cellSize - TILE_INSET * 2, 1),
  };
}
