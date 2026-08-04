import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { ACTION_GROUPS, ACTION_SCHEMAS, blankAction, schemaFor } from './action-schema';
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
