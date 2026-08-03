<script lang="ts">
  import { i18n } from '../lib/i18n.svelte';
  import { themeStore } from '../lib/theme.svelte';

  let t = $derived(i18n.t);
  let icon = $derived(themeStore.value === 'system' ? '◐' : themeStore.value === 'light' ? '☀' : '☾');
</script>

<button
  class="theme-toggle"
  type="button"
  onclick={() => themeStore.cycle()}
  aria-label={t(`theme.${themeStore.value}`)}
  title={t(`theme.${themeStore.value}`)}
>
  <span aria-hidden="true">{icon}</span>
</button>

<style>
  .theme-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2.25rem;
    height: 2.25rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text-muted);
    font-size: var(--text-lg);
    line-height: 1;
    cursor: pointer;
    transition: color var(--transition-fast), border-color var(--transition-fast);
  }
  .theme-toggle:hover {
    color: var(--accent-strong);
    border-color: var(--border-strong);
  }
</style>
