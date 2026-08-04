import { describe, expect, it } from 'vitest';
import { createMonopolyDemo } from './monopoly';
import type { ActionDefinition } from './types';

const game = createMonopolyDemo();

function walk(actions: ActionDefinition[] | undefined, visit: (a: ActionDefinition) => void) {
  for (const action of actions ?? []) {
    visit(action);
    walk(action.then, visit);
    walk(action.else, visit);
    for (const option of action.options ?? []) walk(option.then, visit);
  }
}

function allActions(): ActionDefinition[] {
  const found: ActionDefinition[] = [];
  for (const cell of game.board.cells) {
    walk(cell.onLand, (a) => found.push(a));
    walk(cell.onPass, (a) => found.push(a));
  }
  return found;
}

describe('the Monopoly board', () => {
  it('has 40 squares', () => {
    expect(game.board.cells).toHaveLength(40);
  });

  it('forms a single closed ring, so a player always has somewhere to go', () => {
    const outgoing = new Map(game.board.edges.map((e) => [e.from, e.to]));
    expect(outgoing.size).toBe(40);

    // Walk the ring and check it returns to the start having visited everything.
    const visited = new Set<string>();
    let current = 'go';
    for (let step = 0; step < 40; step++) {
      expect(visited.has(current)).toBe(false);
      visited.add(current);
      current = outgoing.get(current)!;
      expect(current).toBeDefined();
    }
    expect(current).toBe('go');
    expect(visited.size).toBe(40);
  });

  it('gives every cell a declared type', () => {
    for (const cell of game.board.cells) {
      expect(Object.keys(game.rules.cellTypes)).toContain(cell.type);
    }
  });

  it('pays a salary for passing GO', () => {
    expect(game.rules.startBonus).toBeGreaterThan(0);
    expect(game.rules.startBonusResource).toBe('money');
  });

  it('rolls two dice, so the distribution is not flat', () => {
    expect(game.rules.dice).toEqual({ count: 2, sides: 6 });
  });
});

describe('property squares', () => {
  const properties = game.board.cells.filter((c) => c.type === 'property');

  it('exist in quantity', () => {
    expect(properties.length).toBeGreaterThan(20);
  });

  it('declare every field the rent chain reads', () => {
    // A missing field silently resolves to zero at play time, which would look
    // like free rent rather than an error.
    for (const cell of properties) {
      for (const field of ['cost', 'rent', 'rent1', 'rent3', 'rentHotel', 'buildCost', 'mortgageValue']) {
        expect(cell.fields[field], `${cell.id}.${field}`).toBeTypeOf('number');
      }
    }
  });

  it('charge more as buildings go up', () => {
    for (const cell of properties) {
      const { rent, rent1, rent3, rentHotel } = cell.fields as Record<string, number>;
      expect(rent1).toBeGreaterThan(rent);
      expect(rent3).toBeGreaterThan(rent1);
      expect(rentHotel).toBeGreaterThan(rent3);
    }
  });
});

describe('teleports', () => {
  it('only ever point at squares that exist', () => {
    const ids = new Set(game.board.cells.map((c) => c.id));
    const destinations = allActions()
      .filter((a) => a.type === 'move_player_to')
      .map((a) => a.to);

    expect(destinations.length).toBeGreaterThan(0);
    for (const to of destinations) {
      expect(ids.has(to!), `unknown destination ${to}`).toBe(true);
    }
  });

  it('never send a player back to the square they are standing on', () => {
    // The backend rejects this too; catching it here names the offending cell.
    for (const cell of game.board.cells) {
      walk(cell.onLand, (a) => {
        if (a.type === 'move_player_to') {
          expect(a.to, `${cell.id} teleports onto itself`).not.toBe(cell.id);
        }
      });
    }
  });
});

describe('chance squares', () => {
  it('offer several outcomes so the draw is meaningful', () => {
    const chance = game.board.cells.filter((c) => c.type === 'chance');
    expect(chance.length).toBeGreaterThan(1);
    for (const cell of chance) {
      const branch = cell.onLand?.[0];
      expect(branch?.type).toBe('random_branch');
      expect(branch?.options?.length ?? 0).toBeGreaterThanOrEqual(4);
      for (const option of branch!.options!) {
        expect(option.id).toBeTruthy();
        expect(option.title).toBeTruthy();
      }
    }
  });
});

describe('the engine primitives this template depends on', () => {
  it('uses every one of the new action types', () => {
    const used = new Set(allActions().map((a) => a.type));
    for (const type of ['set_cell_level', 'if_cell_level_ge', 'set_cell_mortgaged', 'if_cell_mortgaged', 'move_player_to', 'skip_turns', 'random_branch']) {
      expect(used.has(type), `${type} is never exercised`).toBe(true);
    }
  });
});
