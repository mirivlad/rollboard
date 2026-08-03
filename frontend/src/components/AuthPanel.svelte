<script lang="ts">
  import { i18n } from '../lib/i18n.svelte';
  import LanguagePicker from './LanguagePicker.svelte';

  type Props = {
    onGuest: (displayName: string) => void | Promise<void>;
    onRegister?: (email: string, displayName: string, password: string) => void | Promise<void>;
    onLogin?: (email: string, password: string) => void | Promise<void>;
    busy?: boolean;
    error?: string;
  };

  let { onGuest, onRegister, onLogin, busy = false, error = '' }: Props = $props();
  let displayName = $state('');
  let email = $state('');
  let password = $state('');
  let mode = $state<'guest' | 'register' | 'login'>('guest');
  let t = $derived(i18n.t);

  async function submit() {
    if (mode === 'guest') {
      await onGuest(displayName);
      return;
    }
    if (mode === 'register' && onRegister) {
      await onRegister(email, displayName, password);
      return;
    }
    if (mode === 'login' && onLogin) {
      await onLogin(email, password);
    }
  }
</script>

<section class="auth-panel" aria-label={t('auth.panelLabel')}>
  <div class="top">
    <p class="eyebrow">{t('auth.eyebrow')}</p>
    <LanguagePicker />
  </div>
  <h1>{t('auth.title')}</h1>
  <p class="intro">{t('auth.intro')}</p>

  <div class="tabs" role="tablist" aria-label={t('auth.methodLabel')}>
    <button class:active={mode === 'guest'} onclick={() => (mode = 'guest')} role="tab" aria-selected={mode === 'guest'}>{t('auth.tab.guest')}</button>
    <button class:active={mode === 'register'} onclick={() => (mode = 'register')} role="tab" aria-selected={mode === 'register'}>{t('auth.tab.register')}</button>
    <button class:active={mode === 'login'} onclick={() => (mode = 'login')} role="tab" aria-selected={mode === 'login'}>{t('auth.tab.login')}</button>
  </div>

  <form onsubmit={(event) => { event.preventDefault(); submit(); }}>
    {#if mode !== 'login'}
      <label>
        {t('auth.displayName')}
        <input bind:value={displayName} minlength="1" maxlength="64" autocomplete="nickname" required placeholder={t('auth.displayNamePlaceholder')} />
      </label>
    {/if}
    {#if mode !== 'guest'}
      <label>
        {t('auth.email')}
        <input bind:value={email} type="email" autocomplete="email" required placeholder={t('auth.emailPlaceholder')} />
      </label>
      <label>
        {t('auth.password')}
        <input bind:value={password} type="password" minlength="12" autocomplete={mode === 'login' ? 'current-password' : 'new-password'} required />
      </label>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn btn-primary primary" type="submit" disabled={busy}>
      {busy ? t('auth.submit.busy') : mode === 'guest' ? t('auth.submit.guest') : mode === 'register' ? t('auth.submit.register') : t('auth.submit.login')}
    </button>
  </form>
</section>

<style>
  .auth-panel { width: min(100%, 520px); padding: 2.25rem; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface); box-shadow: var(--shadow-lg); }
  .top { display: flex; justify-content: space-between; align-items: center; gap: 1rem; }
  .eyebrow { margin: 0; color: var(--accent-strong); font-weight: 800; letter-spacing: .12em; font-size: .75rem; }
  h1 { margin: .45rem 0 .75rem; font-size: clamp(1.75rem, 5vw, 2.45rem); line-height: 1.1; color: var(--text); }
  .intro { color: var(--text-muted); line-height: 1.55; }
  .tabs { display: grid; grid-template-columns: repeat(3, 1fr); gap: .4rem; margin: 1.6rem 0 1rem; }
  .tabs button { border: 1px solid var(--border); border-radius: 8px; padding: .6rem .35rem; background: var(--surface-sunken); color: var(--text-muted); cursor: pointer; }
  .tabs button.active { border-color: var(--accent-strong); background: var(--accent-surface); color: var(--text); }
  form { display: grid; gap: .9rem; }
  label { display: grid; gap: .35rem; color: var(--text); font-size: .9rem; }
  input { box-sizing: border-box; width: 100%; border: 1px solid var(--border-strong); border-radius: 8px; padding: .75rem; background: var(--surface-sunken); color: var(--text); font: inherit; }
  .primary { margin-top: .3rem; border: 0; border-radius: 8px; padding: .82rem 1rem; background: var(--accent); color: var(--accent-contrast); cursor: pointer; font-weight: 800; font: inherit; }
  .primary:disabled { opacity: .6; cursor: wait; }
  .error { margin: 0; color: var(--danger); }
</style>
