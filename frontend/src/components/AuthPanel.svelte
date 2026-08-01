<script lang="ts">
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

<section class="auth-panel" aria-label="Welcome to Rollboard">
  <p class="eyebrow">ROLLBOARD</p>
  <h1>Create and play turn-based board games</h1>
  <p class="intro">Start a private guest session, or create an account to save drafts and publish games.</p>

  <div class="tabs" role="tablist" aria-label="Sign-in method">
    <button class:active={mode === 'guest'} onclick={() => (mode = 'guest')} role="tab" aria-selected={mode === 'guest'}>Guest</button>
    <button class:active={mode === 'register'} onclick={() => (mode = 'register')} role="tab" aria-selected={mode === 'register'}>Create account</button>
    <button class:active={mode === 'login'} onclick={() => (mode = 'login')} role="tab" aria-selected={mode === 'login'}>Sign in</button>
  </div>

  <form onsubmit={(event) => { event.preventDefault(); submit(); }}>
    {#if mode !== 'login'}
      <label>
        Display name
        <input bind:value={displayName} minlength="1" maxlength="64" autocomplete="nickname" required placeholder="How should players see you?" />
      </label>
    {/if}
    {#if mode !== 'guest'}
      <label>
        Email
        <input bind:value={email} type="email" autocomplete="email" required placeholder="you@example.com" />
      </label>
      <label>
        Password
        <input bind:value={password} type="password" minlength="12" autocomplete={mode === 'login' ? 'current-password' : 'new-password'} required />
      </label>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="primary" type="submit" disabled={busy}>
      {busy ? 'Please wait…' : mode === 'guest' ? 'Continue as guest' : mode === 'register' ? 'Create account' : 'Sign in'}
    </button>
  </form>
</section>

<style>
  .auth-panel { width: min(100%, 520px); padding: 2.25rem; border: 1px solid #2a3554; border-radius: 20px; background: #121a2e; box-shadow: 0 22px 60px #05081799; }
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
