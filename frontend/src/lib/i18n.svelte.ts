import en from '../../../locales/en.json';
import { ApiError } from './api';

export type Catalog = Record<string, string>;
export type Params = Record<string, string | number>;

/**
 * English ships inside the bundle purely as a safety net: if the locale volume
 * is missing or misconfigured the interface still reads correctly instead of
 * rendering raw keys. The catalogs served by the backend are authoritative, so
 * a deployment can correct or extend any language without a rebuild.
 */
const FALLBACK: Catalog = en as Catalog;
const FALLBACK_LOCALE = 'en';
const STORAGE_KEY = 'rollboard_locale';

/**
 * Locales the interface ships with. The list is only used to pick a sensible
 * default and to label the switcher; the backend decides what is really
 * available, and any extra language it reports is offered too.
 */
export const BUILT_IN_LOCALES = ['en', 'ru'] as const;

/** Endonyms, so each language is listed the way its own speakers write it. */
const LOCALE_NAMES: Record<string, string> = {
  en: 'English',
  ru: 'Русский',
};

export function localeName(tag: string): string {
  if (LOCALE_NAMES[tag]) return LOCALE_NAMES[tag];
  try {
    return new Intl.DisplayNames([tag], { type: 'language' }).of(tag) ?? tag;
  } catch {
    return tag;
  }
}

let locale = $state(FALLBACK_LOCALE);
let catalog = $state<Catalog>(FALLBACK);
let available = $state<string[]>([...BUILT_IN_LOCALES]);

/**
 * Resolve one message.
 *
 * Plural forms use Intl.PluralRules, which matters well beyond English: Russian
 * selects between one/few/many depending on the number, so `5 игроков` and
 * `2 игрока` cannot be produced by a simple singular/plural pair. A key with a
 * `count` parameter is looked up as `<key>.<category>` first and falls back to
 * the bare key for languages that need only one form.
 */
export function translate(source: Catalog, activeLocale: string, key: string, params?: Params): string {
  let template = source[key] ?? FALLBACK[key];

  if (params && typeof params.count === 'number') {
    const category = pluralCategory(activeLocale, params.count);
    template =
      source[`${key}.${category}`] ??
      source[key] ??
      FALLBACK[`${key}.${category}`] ??
      FALLBACK[key];
  }

  // Showing the key is deliberate: a visible `room.title` in the interface is a
  // reported bug, whereas an empty string silently hides missing translations.
  if (template === undefined) return key;

  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    params[name] === undefined ? whole : String(params[name]),
  );
}

/**
 * Turn any thrown value into something worth showing a player.
 *
 * A known error code becomes a translated sentence. An unknown one falls back
 * to the server's English prose, which is worse than a translation but far
 * better than swallowing the reason.
 */
export function errorMessage(t: (key: string, params?: Params) => string, error: unknown): string {
  if (error instanceof ApiError) {
    const key = `errors.${error.code}`;
    const translated = t(key);
    return translated === key ? error.message : translated;
  }
  return t('app.genericError');
}

function pluralCategory(activeLocale: string, count: number): string {
  try {
    return new Intl.PluralRules(activeLocale).select(count);
  } catch {
    return new Intl.PluralRules(FALLBACK_LOCALE).select(count);
  }
}

/** Pick the best available language from what the browser asks for. */
export function negotiateLocale(requested: readonly string[], supported: readonly string[]): string {
  for (const candidate of requested) {
    const exact = supported.find((tag) => tag.toLowerCase() === candidate.toLowerCase());
    if (exact) return exact;
    // `ru-RU` from the browser should match a catalog published as `ru`.
    const base = candidate.split('-')[0].toLowerCase();
    const partial = supported.find((tag) => tag.split('-')[0].toLowerCase() === base);
    if (partial) return partial;
  }
  return supported.includes(FALLBACK_LOCALE) ? FALLBACK_LOCALE : (supported[0] ?? FALLBACK_LOCALE);
}

function storedLocale(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

function rememberLocale(tag: string) {
  try {
    localStorage.setItem(STORAGE_KEY, tag);
  } catch {
    // Private browsing modes can refuse storage; the choice simply will not
    // survive a reload.
  }
}

async function fetchCatalog(tag: string): Promise<Catalog | null> {
  try {
    const response = await fetch(`/api/locales/${encodeURIComponent(tag)}`, { credentials: 'same-origin' });
    if (!response.ok) return null;
    return (await response.json()) as Catalog;
  } catch {
    return null;
  }
}

async function fetchAvailable(): Promise<string[]> {
  try {
    const response = await fetch('/api/locales', { credentials: 'same-origin' });
    if (!response.ok) return [];
    const body = (await response.json()) as { locales?: string[] };
    return body.locales ?? [];
  } catch {
    return [];
  }
}

/**
 * The reactive translation handle used throughout the interface.
 *
 * `t` is a getter so that reading `i18n.t` inside a component re-runs when the
 * language changes, which is what makes switching language update the whole
 * page without a reload.
 */
export const i18n = {
  get locale() {
    return locale;
  },
  get available() {
    return available;
  },
  get t() {
    const activeCatalog = catalog;
    const activeLocale = locale;
    return (key: string, params?: Params) => translate(activeCatalog, activeLocale, key, params);
  },

  /** Load the language list and activate the stored, negotiated, or default one. */
  async init() {
    const served = await fetchAvailable();
    available = served.length > 0 ? served : [...BUILT_IN_LOCALES];

    const preferred =
      storedLocale() ??
      negotiateLocale(typeof navigator !== 'undefined' ? navigator.languages ?? [navigator.language] : [], available);

    await this.setLocale(preferred, { remember: false });
  },

  async setLocale(tag: string, options: { remember?: boolean } = {}) {
    const next = available.includes(tag) ? tag : FALLBACK_LOCALE;
    const loaded = await fetchCatalog(next);

    // A language with no catalog on the server still renders through the
    // bundled fallback rather than showing bare keys.
    catalog = loaded ?? (next === FALLBACK_LOCALE ? FALLBACK : { ...FALLBACK, ...(loaded ?? {}) });
    locale = next;

    if (typeof document !== 'undefined') {
      document.documentElement.lang = next;
    }
    if (options.remember !== false) {
      rememberLocale(next);
    }
  },
};
