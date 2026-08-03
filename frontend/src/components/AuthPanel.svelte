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
    <button class="primary" type="submit" disabled={busy}>
      {busy ? t('auth.submit.busy') : mode === 'guest' ? t('auth.submit.guest') : mode === 'register' ? t('auth.submit.register') : t('auth.submit.login')}
    </button>
  </form>
</section>

<style>
  .auth-panel { width: min(100%, 520px); padding: 2.25rem; border: 1px solid #2a3554; border-radius: 20px; background: #121a2e; box-shadow: 0 22px 60px #05081799; }
  .top { display: flex; justify-content: space-between; align-items: center; gap: 1rem; }
  .eyebrow { margin: 0; color: #75d4ff; font-weight: 800; letter-spacing: .12em; font-size: .75rem; }
  h1 { margin: .45rem 0 .75rem; font-size: clamp(1.75rem, 5vw, 2.45rem); line-height: 1.1; color: #f5f7ff; }
  .intro { color: #aab6d3; line-height: 1.55; }
  .tabs { display: grid; grid-template-columns: repeat(3, 1fr); gap: .4rem; margin: 1.6rem 0 1rem; }
  .tabs button { border: 1px solid #2a3554; border-radius: 8px; padding: .6rem .35rem; background: #0b1224; color: #b8c4e0; cursor: pointer; }
  .tabs button.active { border-color: #6dd3ff; background: #153451; color: #fff; }
  form { display: grid; gap: .9rem; }
  label { display: grid; gap: .35rem; color: #dce5fb; font-size: .9rem; }
  input { box-sizing: border-box; width: 100%; border: 1px solid #354568; border-radius: 8px; padding: .75rem; background: #0b1224; color: #f5f7ff; font: inherit; }
  .primary { margin-top: .3rem; border: 0; border-radius: 8px; padding: .82rem 1rem; background: #52c4f2; color: #061120; cursor: pointer; font-weight: 800; font: inherit; }
  .primary:disabled { opacity: .6; cursor: wait; }
  .error { margin: 0; color: #ff8b9d; }
</style>
