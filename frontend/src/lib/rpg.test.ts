import { describe, expect, it } from 'vitest';
import { createRpgDemo } from './rpg';
import type { ActionDefinition } from './types';

const game = createRpgDemo();

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
  for (const item of Object.values(game.rules.items ?? {})) {
    walk(item.use, (a) => found.push(a));
  }
  return found;
}

describe('the dungeon board', () => {
  it('is a full grid', () => {
    expect(game.board.cells).toHaveLength(36);
  });

  it('connects neighbours in both directions, so a free move can go anywhere', () => {
    const pairs = new Set(game.board.edges.map((e) => `${e.from}->${e.to}`));
    for (const edge of game.board.edges) {
      expect(pairs.has(`${edge.to}->${edge.from}`), `${edge.to}->${edge.from} missing`).toBe(true);
    }
  });

  it('lets a player reach every square from the camp', () => {
    const neighbours = new Map<string, string[]>();
    for (const edge of game.board.edges) {
      neighbours.set(edge.from, [...(neighbours.get(edge.from) ?? []), edge.to]);
    }
    const seen = new Set(['camp']);
    const queue = ['camp'];
    while (queue.length) {
      for (const next of neighbours.get(queue.shift()!) ?? []) {
        if (!seen.has(next)) {
          seen.add(next);
          queue.push(next);
        }
      }
    }
    expect(seen.size).toBe(36);
  });

  it('explores rather than following a track', () => {
    expect(game.rules.movement).toBe('free');
    expect(game.rules.hiddenCells).toBe(true);
  });
});

describe('character progression', () => {
  it('declares stats, a level and spendable points', () => {
    for (const stat of ['health', 'attack', 'defence', 'agility', 'experience', 'level', 'points']) {
      expect(game.rules.resources[stat], stat).toBeDefined();
    }
  });

  it('has rising experience thresholds', () => {
    const thresholds = game.rules.progression!.thresholds;
    expect(thresholds.length).toBeGreaterThan(3);
    for (let i = 1; i < thresholds.length; i++) {
      expect(thresholds[i]).toBeGreaterThan(thresholds[i - 1]);
    }
  });

  it('lets the camp turn a point into a stat', () => {
    const camp = game.board.cells.find((c) => c.id === 'camp')!;
    const spends = new Set<string>();
    walk(camp.onLand, (a) => {
      if (a.type === 'gain_resource' && a.resource && a.resource !== 'health') spends.add(a.resource);
    });
    expect([...spends].sort()).toEqual(['agility', 'attack', 'defence']);
  });
});

describe('equipment', () => {
  it('only puts items in declared slots', () => {
    for (const [id, item] of Object.entries(game.rules.items ?? {})) {
      if (item.slot) {
        expect(game.rules.equipmentSlots, `${id} slot ${item.slot}`).toContain(item.slot);
      }
    }
  });

  it('only grants bonuses to stats that exist', () => {
    for (const [id, item] of Object.entries(game.rules.items ?? {})) {
      for (const stat of Object.keys(item.bonuses ?? {})) {
        expect(game.rules.resources[stat], `${id} bonus ${stat}`).toBeDefined();
      }
    }
  });

  it('offers weapons that are worth upgrading to', () => {
    const weapons = Object.values(game.rules.items ?? {})
      .filter((item) => item.slot === 'weapon')
      .map((item) => item.bonuses?.attack ?? 0)
      .sort((a, b) => a - b);
    expect(weapons.length).toBeGreaterThanOrEqual(3);
    // A later weapon must actually beat an earlier one, or looting is pointless.
    expect(weapons[weapons.length - 1]).toBeGreaterThan(weapons[0]);
  });

  it('every item a cell hands out is defined', () => {
    const granted = allActions().filter((a) => a.type === 'grant_item').map((a) => a.field);
    expect(granted.length).toBeGreaterThan(0);
    for (const id of granted) {
      expect(game.rules.items?.[id!], `granted unknown item ${id}`).toBeDefined();
    }
  });
});

describe('the dungeon itself', () => {
  it('has enemies, traps, loot, shrines and exactly one boss', () => {
    const counts = new Map<string, number>();
    for (const cell of game.board.cells) counts.set(cell.type, (counts.get(cell.type) ?? 0) + 1);
    expect(counts.get('enemy') ?? 0).toBeGreaterThanOrEqual(4);
    expect(counts.get('trap') ?? 0).toBeGreaterThanOrEqual(3);
    expect(counts.get('loot') ?? 0).toBeGreaterThanOrEqual(4);
    expect(counts.get('shrine') ?? 0).toBeGreaterThanOrEqual(1);
    expect(counts.get('boss') ?? 0).toBe(1);
  });

  it('gets harder the further in you go', () => {
    const enemies = game.board.cells.filter((c) => c.type === 'enemy');
    const difficulties = enemies.map((c) => c.fields.difficulty as number);
    expect(Math.max(...difficulties)).toBeGreaterThan(Math.min(...difficulties) * 2);
  });

  it('can actually be won and lost', () => {
    const types = new Set(allActions().map((a) => a.type));
    expect(types.has('finish_game'), 'no victory condition').toBe(true);
    expect(types.has('eliminate_player'), 'death is impossible').toBe(true);
  });

  it('offers a way to scout ahead', () => {
    expect(allActions().some((a) => a.type === 'reveal_cells')).toBe(true);
  });

  it('exercises the whole RPG action set', () => {
    const used = new Set(allActions().map((a) => a.type));
    for (const type of ['grant_item', 'if_stat_ge', 'reveal_cells', 'random_branch', 'eliminate_player', 'finish_game']) {
      expect(used.has(type), `${type} is never used`).toBe(true);
    }
  });
});
