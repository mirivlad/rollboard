import { render } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import BoardView from './BoardView.svelte';
import { TILE_INSET } from '../lib/board-layout';
import type { Board, PlayerState } from '../lib/types';

const CELL = 96;

const board: Board = {
  width: 288, height: 96, cellSize: CELL,
  cells: [
    { id: 'start', title: 'Start', type: 'start', x: 0, y: 0, visual: { baseColor: '#4CAF50', baseImage: '' }, fields: {} },
    { id: 'dock', title: 'Dock', type: 'property', x: CELL, y: 0, visual: { baseColor: '#E3F2FD', baseImage: '' }, fields: {} },
    { id: 'far', title: 'Far', type: 'property', x: CELL * 2, y: 0, visual: { baseColor: '#FFE0B2', baseImage: '' }, fields: {} },
  ],
  edges: [],
};

const players: PlayerState[] = [
  { id: 'p1', name: 'Ada', color: '#e74c3c', positionCellId: 'far', resources: {}, bankrupt: false },
];

function left(element: Element): number {
  return Number.parseFloat((element as HTMLElement).style.left);
}

describe('BoardView', () => {
  const { container } = render(BoardView, { board, players, cellStates: {} });

  it('draws each tile at its own coordinates', () => {
    // The tile used to sit inside a second positioned wrapper offset by the
    // same coordinates, which put every cell at twice its distance from the
    // origin. The grid, the tokens and the edges were all right; the tiles
    // alone drifted, and the further along the board a cell was, the further
    // out it went.
    const tiles = [...container.querySelectorAll('.cell')];
    expect(tiles).toHaveLength(3);
    expect(tiles.map(left)).toEqual([TILE_INSET, CELL + TILE_INSET, CELL * 2 + TILE_INSET]);

    // Those coordinates are absolute, so they must be measured from the board
    // itself. jsdom does no layout, so the offset a positioned wrapper adds is
    // invisible to a style check — the structure is what has to be asserted.
    for (const tile of tiles) {
      expect(tile.parentElement?.classList.contains('board-area'), 'a tile is nested inside another element').toBe(true);
    }
  });

  it('puts the token on the tile the player is standing on', () => {
    const token = container.querySelector('.token');
    const far = [...container.querySelectorAll('.cell')][2] as HTMLElement;
    const tileCentre = left(far) + Number.parseFloat(far.style.width) / 2;
    // Within the gutter: the token is centred on the slot, the tile is inset
    // inside it, so their centres coincide.
    expect(Math.abs(left(token!) - tileCentre)).toBeLessThanOrEqual(1);
  });

  it('lines the tiles up with the grid slots they belong to', () => {
    const slots = [...container.querySelectorAll('.grid-cell')].map(left);
    for (const tile of container.querySelectorAll('.cell')) {
      expect(slots).toContain(left(tile) - TILE_INSET);
    }
  });
});
