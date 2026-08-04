<script lang="ts">
  import type { RoomInvite } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';
  import LanguagePicker from './LanguagePicker.svelte';
  import ThemeToggle from './ThemeToggle.svelte';

  type Props = {
    invite: RoomInvite;
    /** Somebody signed out has to pick a name before they can be a player. */
    needsIdentity: boolean;
    onJoin: (displayName: string) => void | Promise<void>;
    onDismiss: () => void;
    busy?: boolean;
    error?: string;
  };

  let { invite, needsIdentity, onJoin, onDismiss, busy = false, error = '' }: Props = $props();
  let displayName = $state('');
  let t = $derived(i18n.t);
</script>

<section class="invite-card" aria-label={t('invite.heading')}>
  <div class="top">
    <p class="eyebrow">{t('invite.eyebrow')}</p>
    <div class="controls">
      <LanguagePicker />
      <ThemeToggle />
    </div>
  </div>

  <h1>{invite.title}</h1>
  <p class="game">{t('invite.game', { title: invite.gameTitle })}</p>
  <p class="seats">{t('invite.seats', { count: invite.memberCount, max: invite.maxPlayers })}</p>

  {#if !invite.joinable}
    <p class="closed" role="status">
      {invite.status === 'lobby' ? t('invite.full') : t('invite.alreadyStarted')}
    </p>
    <button class="btn" onclick={onDismiss}>{t('invite.browseInstead')}</button>
  {:else}
    <form onsubmit={(event) => { event.preventDefault(); onJoin(displayName); }}>
      {#if needsIdentity}
        <label>
          {t('auth.displayName')}
          <input bind:value={displayName} minlength="1" maxlength="64" autocomplete="nickname" required placeholder={t('auth.displayNamePlaceholder')} />
        </label>
      {/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button class="btn btn-primary" type="submit" disabled={busy}>
        {busy ? t('auth.submit.busy') : t('invite.join')}
      </button>
    </form>
    <button class="link" onclick={onDismiss}>{t('invite.notNow')}</button>
  {/if}
</section>

<style>
  .invite-card {
    width: min(100%, 480px);
    padding: var(--space-6);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-lg);
  }
  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-4);
  }
  .controls {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .eyebrow {
    margin: 0;
    color: var(--accent-strong);
    font-weight: var(--weight-black);
    letter-spacing: .12em;
    font-size: var(--text-xs);
  }
  h1 {
    margin: var(--space-2) 0 var(--space-1);
    font-size: var(--text-2xl);
  }
  .game {
    margin: 0;
    color: var(--text-muted);
  }
  .seats {
    margin: var(--space-1) 0 var(--space-5);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .closed {
    margin: 0 0 var(--space-4);
    padding: var(--space-3);
    border: 1px solid var(--warning);
    border-left-width: 3px;
    border-radius: var(--radius-md);
    background: var(--warning-surface);
    color: var(--text);
  }
  form {
    display: grid;
    gap: var(--space-3);
  }
  label {
    display: grid;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  input {
    width: 100%;
    padding: var(--space-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .link {
    display: block;
    margin: var(--space-4) auto 0;
    padding: var(--space-1);
    border: 0;
    background: none;
    color: var(--text-faint);
    font: inherit;
    font-size: var(--text-sm);
    text-decoration: underline;
    cursor: pointer;
  }
  .link:hover {
    color: var(--text-muted);
  }
</style>
