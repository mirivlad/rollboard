<script lang="ts">
  import { api } from '../lib/api';
  import { errorMessage, i18n } from '../lib/i18n.svelte';

  type Props = { roomId: string };
  let { roomId }: Props = $props();

  let token = $state('');
  let error = $state('');
  let copied = $state(false);
  let busy = $state(false);
  let t = $derived(i18n.t);
  // Built from the origin the host is actually browsing, so the link they copy
  // is the one that works for them.
  let link = $derived(token ? `${location.origin}/?invite=${token}` : '');

  $effect(() => {
    const id = roomId;
    if (!id) return;
    void api.getRoomInvite(id).then((result) => (token = result.token)).catch(() => {
      // Only the host can read the invite; everybody else simply sees nothing.
      token = '';
    });
  });

  async function copy() {
    error = '';
    try {
      await navigator.clipboard.writeText(link);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      error = t('invite.copyFailed');
    }
  }

  async function rotate() {
    busy = true; error = '';
    try {
      token = (await api.rotateRoomInvite(roomId)).token;
    } catch (cause) {
      error = errorMessage(t, cause);
    } finally {
      busy = false;
    }
  }
</script>

{#if token}
  <section class="invite-link">
    <h2>{t('invite.shareHeading')}</h2>
    <p class="hint">{t('invite.shareHint')}</p>
    <div class="row">
      <input readonly value={link} aria-label={t('invite.shareHeading')} onfocus={(event) => (event.target as HTMLInputElement).select()} />
      <button class="btn" onclick={copy}>{copied ? t('invite.copied') : t('invite.copy')}</button>
    </div>
    <button class="rotate" onclick={rotate} disabled={busy}>{t('invite.rotate')}</button>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </section>
{/if}

<style>
  .invite-link {
    margin-bottom: var(--space-4);
    padding-bottom: var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }
  h2 {
    margin: 0 0 var(--space-1);
    font-size: var(--text-lg);
  }
  .hint {
    margin: 0 0 var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .row {
    display: flex;
    gap: var(--space-2);
  }
  input {
    flex: 1;
    min-width: 0;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }
  .rotate {
    margin-top: var(--space-2);
    padding: var(--space-1) 0;
    border: 0;
    background: none;
    color: var(--text-faint);
    font: inherit;
    font-size: var(--text-xs);
    text-decoration: underline;
    cursor: pointer;
  }
  .rotate:hover:not(:disabled) {
    color: var(--danger);
  }
  .error {
    margin: var(--space-2) 0 0;
    color: var(--danger);
    font-size: var(--text-sm);
  }
</style>
