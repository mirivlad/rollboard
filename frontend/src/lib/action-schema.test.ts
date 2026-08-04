import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { ACTION_GROUPS, ACTION_SCHEMAS, blankAction, nextOptionId, schemaFor } from './action-schema';
import en from '../../../locales/en.json';
import ru from '../../../locales/ru.json';

const ENGINE_DIR = join(import.meta.dirname, '../../../backend/internal/game');

/**
 * The action types the engine actually executes, read from its own source.
 *
 * Hard-coding the list here would let it drift, which is exactly the failure
 * this test exists to prevent: the editor previously knew seven types while
 * the engine ran twenty-seven, so most of the language was unreachable without
 * writing a definition by hand.
 */
function engineActionTypes(): string[] {
  const types = new Set<string>();
  for (const file of readdirSync(ENGINE_DIR)) {
    if (!file.endsWith('.go') || file.endsWith('_test.go')) continue;
    const source = readFileSync(join(ENGINE_DIR, file), 'utf8');
    // Only the dispatch switch inside executeOneAction declares action types.
    const dispatch = source.slice(source.indexOf('func (s *GameSession) executeOneAction('));
    if (!dispatch) continue;
    const end = dispatch.indexOf('\nfunc ', 1);
    const body = end === -1 ? dispatch : dispatch.slice(0, end);
    for (const match of body.matchAll(/^\tcase ("(?:[a-z_]+)"(?:, "(?:[a-z_]+)")*):/gm)) {
      for (const quoted of match[1].split(',')) {
        types.add(quoted.trim().replace(/"/g, ''));
      }
    }
  }
  return [...types].sort();
}

describe('the action schema', () => {
  const engineTypes = engineActionTypes();

  it('finds the engine dispatch table at all', () => {
    // A refactor that moves the switch should fail loudly here rather than
    // quietly making this whole file pass on an empty list.
    expect(engineTypes.length).toBeGreaterThan(20);
    expect(engineTypes).toContain('gain_resource');
  });

  it('covers every action the engine can execute', () => {
    const known = new Set(ACTION_SCHEMAS.map((schema) => schema.type));
    const missing = engineTypes.filter((type) => !known.has(type) && type !== 'launch_minigame');
    expect(missing, `these actions cannot be reached from the editor: ${missing.join(', ')}`).toEqual([]);
  });

  it('declares no action the engine does not implement', () => {
    const engine = new Set(engineTypes);
    const extra = ACTION_SCHEMAS.map((s) => s.type).filter((type) => !engine.has(type));
    expect(extra, `the editor offers actions the engine ignores: ${extra.join(', ')}`).toEqual([]);
  });

  it('gives every action a group the picker knows', () => {
    for (const schema of ACTION_SCHEMAS) {
      expect(ACTION_GROUPS, schema.type).toContain(schema.group);
    }
  });

  it('has no duplicate types', () => {
    const types = ACTION_SCHEMAS.map((s) => s.type);
    expect(new Set(types).size).toBe(types.length);
  });
});

describe('schema labels', () => {
  const catalogs: Record<string, Record<string, string>> = { en, ru } as never;

  it('are translated in every language', () => {
    const keys = new Set<string>();
    for (const schema of ACTION_SCHEMAS) {
      keys.add(schema.labelKey);
      for (const field of schema.fields) keys.add(field.labelKey);
    }
    for (const group of ACTION_GROUPS) keys.add(`actionGroup.${group}`);

    for (const [language, catalog] of Object.entries(catalogs)) {
      const missing = [...keys].filter((key) => !(key in catalog));
      expect(missing, `${language} is missing: ${missing.join(', ')}`).toEqual([]);
    }
  });
});

describe('blankAction', () => {
  it('prepares nested lists so the editor can append to them', () => {
    const branch = blankAction('if_resource_ge');
    expect(branch.then).toEqual([]);
    expect(branch.else).toEqual([]);
    expect(blankAction('offer_choice').options).toEqual([]);
  });

  it('starts a cell query as "every cell", which is a working filter', () => {
    // Validation refuses an action with no query at all, so a blank one has to
    // be an object the author narrows down rather than nothing.
    expect(blankAction('if_cells_ge').query).toEqual({});
    expect(blankAction('for_each_cell').query).toEqual({});
  });

  it('gives an auction both of its branches', () => {
    const auction = blankAction('start_auction');
    expect(auction.then).toEqual([]);
    expect(auction.else).toEqual([]);
  });

  it('carries nothing an action type has no use for', () => {
    // Switching an action's type must not leave a stale amount behind on a
    // type that ignores amounts.
    expect(blankAction('finish_game')).toEqual({ type: 'finish_game' });
    expect(Object.keys(blankAction('eliminate_player'))).toEqual(['type']);
  });
});

describe('schemaFor', () => {
  it('resolves a known type and returns nothing for an unknown one', () => {
    expect(schemaFor('grant_item')?.group).toBe('items');
    expect(schemaFor('not_a_real_action')).toBeUndefined();
  });
});

/**
 * Which action types the engine resolves an amount for.
 *
 * Read from the engine's own source, because "the editor covers everything the
 * engine can do" was only ever checked at the level of action names. Underneath
 * that, five actions accepted a computed amount that the editor never offered,
 * and one offered a computed amount the backend refused to publish.
 */
function enginePayload(): { resolvesAmount: Set<string> } {
  const sources = new Map<string, string>();
  for (const file of readdirSync(ENGINE_DIR)) {
    if (!file.endsWith('.go') || file.endsWith('_test.go')) continue;
    sources.set(file, readFileSync(join(ENGINE_DIR, file), 'utf8'));
  }
  const all = [...sources.values()].join('\n');

  // Helpers that resolve an amount themselves, so a case that delegates to one
  // still counts: reveal_cells and start_auction never mention amountFor in the
  // dispatch table, but both accept a formula.
  const resolvers = new Set<string>();
  for (const match of all.matchAll(/func \(s \*GameSession\) (\w+)\([^)]*\)[^{]*\{([\s\S]*?)\n\}/g)) {
    if (/s\.amountFor(OrOne)?\(/.test(match[2])) resolvers.add(match[1]);
  }

  const engine = sources.get('engine.go') ?? '';
  const dispatch = engine.slice(engine.indexOf('func (s *GameSession) executeOneAction('));
  const body = dispatch.slice(0, dispatch.indexOf('\nfunc ', 1));

  const resolvesAmount = new Set<string>();
  const cases = body.split(/^\tcase /m).slice(1);
  for (const block of cases) {
    const header = block.slice(0, block.indexOf(':'));
    const types = [...header.matchAll(/"([a-z_]+)"/g)].map((m) => m[1]);
    const code = block.slice(block.indexOf(':') + 1);
    const usesAmount =
      /s\.amountFor(OrOne)?\(/.test(code) ||
      [...resolvers].some((name) => new RegExp(`s\\.${name}\\(`).test(code));
    if (usesAmount) for (const type of types) resolvesAmount.add(type);
  }
  return { resolvesAmount };
}

/** The fields ActionDefinition actually carries, from its JSON tags. */
function engineActionFields(): Set<string> {
  const source = readFileSync(join(ENGINE_DIR, 'definition.go'), 'utf8');
  const struct = source.slice(source.indexOf('type ActionDefinition struct {'));
  const body = struct.slice(0, struct.indexOf('\n}'));
  return new Set([...body.matchAll(/json:"([a-zA-Z]+)/g)].map((m) => m[1]));
}

describe('the editor offers what the engine reads', () => {
  const { resolvesAmount } = enginePayload();

  it('finds the amount-resolving actions at all', () => {
    expect(resolvesAmount.size).toBeGreaterThan(8);
    expect(resolvesAmount).toContain('gain_resource');
  });

  it('offers a computed amount wherever the engine resolves one', () => {
    const missing = [...resolvesAmount].filter((type) => {
      const schema = schemaFor(type);
      return schema && !schema.fields.some((field) => field.kind === 'formula');
    });
    expect(missing, `the engine computes an amount for these but the editor has no formula: ${missing.join(', ')}`).toEqual([]);
  });

  it('offers a computed amount nowhere else', () => {
    // A control the engine ignores is worse than a missing one: the author
    // fills it in and the game quietly does something else.
    const extra = ACTION_SCHEMAS
      .filter((schema) => schema.fields.some((field) => field.kind === 'formula'))
      .map((schema) => schema.type)
      .filter((type) => !resolvesAmount.has(type));
    expect(extra, `the editor offers a formula the engine never reads: ${extra.join(', ')}`).toEqual([]);
  });

  it('writes only fields ActionDefinition carries', () => {
    const known = engineActionFields();
    expect(known.size).toBeGreaterThan(10);
    for (const schema of ACTION_SCHEMAS) {
      for (const field of schema.fields) {
        expect(known, `${schema.type}.${field.name} is not a field of ActionDefinition`).toContain(field.name);
      }
    }
  });
});

describe('nextOptionId', () => {
  it('fills the first free slot, so add-remove-add cannot collide', () => {
    // The exact sequence that used to produce two option_3s: three options,
    // delete the middle one, add another.
    let options = [{ id: 'option_1' }, { id: 'option_2' }, { id: 'option_3' }];
    options = options.filter((option) => option.id !== 'option_2');
    const added = nextOptionId(options);
    expect(added).toBe('option_2');
    expect(options.some((option) => option.id === added)).toBe(false);
  });

  it('keeps going past a gap-free list', () => {
    expect(nextOptionId([{ id: 'option_1' }, { id: 'option_2' }])).toBe('option_3');
    expect(nextOptionId([])).toBe('option_1');
  });

  it('ignores ids the author renamed', () => {
    expect(nextOptionId([{ id: 'buy' }, { id: 'walk_away' }])).toBe('option_1');
  });
});
