import { describe, expect, it } from 'vitest';
import { errorMessage, negotiateLocale, translate } from './i18n.svelte';
import { ApiError } from './api';
import en from '../../../locales/en.json';
import ru from '../../../locales/ru.json';

const enCatalog = en as Record<string, string>;
const ruCatalog = ru as Record<string, string>;

describe('translate', () => {
  it('returns the message for a known key', () => {
    expect(translate(enCatalog, 'en', 'app.rooms')).toBe('Rooms');
  });

  it('substitutes named parameters', () => {
    expect(translate(enCatalog, 'en', 'dashboard.welcome', { name: 'Ada' })).toContain('Ada');
  });

  it('leaves an unknown placeholder untouched rather than printing undefined', () => {
    expect(translate({ 'x.y': 'Hello {missing}' }, 'en', 'x.y', { other: 1 })).toBe('Hello {missing}');
  });

  it('falls back to the bundled English catalog for a key the active language lacks', () => {
    expect(translate({}, 'ru', 'app.rooms')).toBe('Rooms');
  });

  it('returns the key itself when nothing defines it, so gaps are visible', () => {
    expect(translate({}, 'en', 'totally.unknown.key')).toBe('totally.unknown.key');
  });
});

describe('plural forms', () => {
  it('selects singular and plural in English', () => {
    expect(translate(enCatalog, 'en', 'playtest.playerCount', { count: 1 })).toBe('1 player');
    expect(translate(enCatalog, 'en', 'playtest.playerCount', { count: 5 })).toBe('5 players');
  });

  // Russian needs three forms, which a naive singular/plural pair cannot produce.
  it('selects one, few and many in Russian', () => {
    expect(translate(ruCatalog, 'ru', 'playtest.playerCount', { count: 1 })).toBe('1 игрок');
    expect(translate(ruCatalog, 'ru', 'playtest.playerCount', { count: 2 })).toBe('2 игрока');
    expect(translate(ruCatalog, 'ru', 'playtest.playerCount', { count: 5 })).toBe('5 игроков');
    expect(translate(ruCatalog, 'ru', 'playtest.playerCount', { count: 21 })).toBe('21 игрок');
    expect(translate(ruCatalog, 'ru', 'playtest.playerCount', { count: 22 })).toBe('22 игрока');
    expect(translate(ruCatalog, 'ru', 'playtest.playerCount', { count: 11 })).toBe('11 игроков');
  });
});

describe('negotiateLocale', () => {
  it('prefers an exact match', () => {
    expect(negotiateLocale(['ru', 'en'], ['en', 'ru'])).toBe('ru');
  });

  it('matches a regional tag against its base language', () => {
    expect(negotiateLocale(['ru-RU'], ['en', 'ru'])).toBe('ru');
  });

  it('walks the preference list in order', () => {
    expect(negotiateLocale(['de', 'ru', 'en'], ['en', 'ru'])).toBe('ru');
  });

  it('falls back to English when nothing matches', () => {
    expect(negotiateLocale(['ja'], ['en', 'ru'])).toBe('en');
  });

  it('falls back to the first supported locale when English is absent', () => {
    expect(negotiateLocale(['ja'], ['ru'])).toBe('ru');
  });
});

describe('errorMessage', () => {
  const t = (key: string) => translate(ruCatalog, 'ru', key);

  it('translates a known server error code', () => {
    const message = errorMessage(t, new ApiError('ACCOUNT_REQUIRED', 'account required', '', 403));
    expect(message).toBe(ruCatalog['errors.ACCOUNT_REQUIRED']);
  });

  it('falls back to the server prose for an unrecognised code', () => {
    const message = errorMessage(t, new ApiError('SOME_NEW_CODE', 'a brand new failure', '', 400));
    expect(message).toBe('a brand new failure');
  });

  it('reports a generic message for a non-API failure', () => {
    expect(errorMessage(t, new TypeError('boom'))).toBe(ruCatalog['app.genericError']);
  });
});

describe('catalog completeness', () => {
  // A translation that silently loses keys degrades to English at runtime with
  // no warning, so the mismatch is asserted here instead.
  it('translates every English key into Russian', () => {
    const missing = Object.keys(enCatalog).filter((key) => !(key in ruCatalog));
    expect(missing).toEqual([]);
  });

  it('defines no Russian key that English does not have', () => {
    // Plural categories English does not need are legitimately extra.
    const pluralExtras = /\.(one|two|few|many|other)$/;
    const extra = Object.keys(ruCatalog).filter((key) => !(key in enCatalog) && !pluralExtras.test(key));
    expect(extra).toEqual([]);
  });

  it('keeps the same placeholders in both languages', () => {
    const placeholders = (value: string) => (value.match(/\{(\w+)\}/g) ?? []).sort();
    const mismatched = Object.keys(enCatalog)
      .filter((key) => key in ruCatalog)
      .filter((key) => placeholders(enCatalog[key]).join() !== placeholders(ruCatalog[key]).join());
    expect(mismatched).toEqual([]);
  });
});
