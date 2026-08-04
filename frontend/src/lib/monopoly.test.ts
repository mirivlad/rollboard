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

describe('colour groups', () => {
  const properties = game.board.cells.filter((c) => c.type === 'property');

  it('give every street a group other streets share', () => {
    const sizes = new Map<string, number>();
    for (const cell of properties) {
      const group = cell.fields.group;
      expect(group, `${cell.id} has no colour group`).toBeTypeOf('string');
      sizes.set(group, (sizes.get(group) ?? 0) + 1);
    }
    // A group of one would be a monopoly the moment it was bought.
    for (const [group, size] of sizes) expect(size, group).toBeGreaterThan(1);
  });

  it('detects a complete group by comparing two queries, not a fixed number', () => {
    // Hard-coding "3 squares" would quietly break the moment somebody edited
    // the board, which is exactly what a template must not teach.
    const monopolyCheck = allActions().find((a) => a.type === 'if_cells_ge');
    expect(monopolyCheck).toBeDefined();
    expect(monopolyCheck!.query).toMatchObject({ field: 'group', sameAsCell: true, owner: 'cellOwner' });
    expect(monopolyCheck!.formula?.base).toMatchObject({ kind: 'cells' });
    expect(monopolyCheck!.amount).toBeUndefined();
  });
});

describe('stations and utilities', () => {
  it('charge rent per holding rather than by tier', () => {
    for (const type of ['station', 'utility']) {
      const cells = game.board.cells.filter((c) => c.type === type);
      expect(cells.length, type).toBeGreaterThan(1);

      const rents: ActionDefinition[] = [];
      for (const cell of cells) walk(cell.onLand, (a) => { if (a.type === 'transfer_resource') rents.push(a); });
      expect(rents.length, `${type} charges no rent`).toBeGreaterThan(0);
      for (const rent of rents) {
        // The multiplier counts the landlord's holdings; counting the
        // visitor's would charge a tenant for their own stations.
        expect(rent.formula?.times).toMatchObject({ kind: 'cells', query: { type, owner: 'cellOwner' } });
      }
    }
  });
});

describe('auctions', () => {
  it('put a declined square in front of the whole table', () => {
    const auctions = allActions().filter((a) => a.type === 'start_auction');
    expect(auctions.length).toBeGreaterThan(0);
    for (const auction of auctions) {
      expect(auction.resource).toBe('money');
      // An auction that awards nothing takes the winning bid and gives back
      // nothing; the backend refuses to publish one.
      expect(auction.then?.[0]?.type).toBe('set_cell_owner');
      expect(auction.else?.length ?? 0).toBeGreaterThan(0);
    }
  });

  it('is reachable both by declining and by being unable to pay', () => {
    const unowned = game.board.cells
      .find((c) => c.type === 'property')!
      .onLand!.find((a) => a.type === 'if_cell_unowned')!;
    const offer = unowned.then![0];
    const declined = offer.then![0].options!.find((o) => o.id === 'pass');
    expect(declined!.then![0].type).toBe('start_auction');
    expect(offer.else!.some((a) => a.type === 'start_auction')).toBe(true);
  });
});

describe('board-wide effects', () => {
  it('charges street repairs per built square instead of a flat fee', () => {
    const repairs = allActions().find((a) => a.type === 'for_each_cell');
    expect(repairs).toBeDefined();
    expect(repairs!.query).toMatchObject({ type: 'property', owner: 'current', minLevel: 1 });
    expect(repairs!.then?.[0]?.type).toBe('lose_resource');
  });
});
