export type Theme = 'system' | 'light' | 'dark';

const STORAGE_KEY = 'rollboard_theme';

let theme = $state<Theme>('system');

function apply(next: Theme) {
  if (typeof document === 'undefined') return;
  // 'system' removes the attribute entirely so the prefers-color-scheme
  // media query in tokens.css takes over again.
  if (next === 'system') {
    document.documentElement.removeAttribute('data-theme');
  } else {
    document.documentElement.setAttribute('data-theme', next);
  }
}

function stored(): Theme | null {
  try {
    const value = localStorage.getItem(STORAGE_KEY);
    return value === 'light' || value === 'dark' || value === 'system' ? value : null;
  } catch {
    return null;
  }
}

export const themeStore = {
  get value() {
    return theme;
  },

  /** Whichever theme is actually on screen right now. */
  get resolved(): 'light' | 'dark' {
    if (theme !== 'system') return theme;
    if (typeof window === 'undefined') return 'dark';
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  },

  init() {
    theme = stored() ?? 'system';
    apply(theme);
  },

  set(next: Theme) {
    theme = next;
    apply(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Storage can be unavailable in private browsing; the choice then just
      // does not survive a reload.
    }
  },

  /** Cycle through the three states for a single-button toggle. */
  cycle() {
    const order: Theme[] = ['system', 'light', 'dark'];
    this.set(order[(order.indexOf(theme) + 1) % order.length]);
  },
};
