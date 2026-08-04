<script lang="ts">
  import type { GameSession, PlayerState } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';

  type Props = {
    session: GameSession;
    player: PlayerState;
    onPropose: (offer: {
      toPlayerId: string;
      offerItems?: Record<string, number>;
      offerResources?: Record<string, number>;
      requestItems?: Record<string, number>;
      requestResources?: Record<string, number>;
    }) => void | Promise<void>;
  };

  let { session, player, onPropose }: Props = $props();
  let t = $derived(i18n.t);
  let open = $state(false);
  let partnerId = $state('');
  let giveItem = $state('');
  let giveGold = $state(0);
  let askItem = $state('');
  let askGold = $state(0);

  let items = $derived(session.definition.rules.items ?? {});
  // The first declared resource stands in for currency; a game with no
  // resources at all can still trade items.
  let currency = $derived(Object.keys(session.definition.rules.resources ?? {})[0] ?? '');
  let partners = $derived(session.state.players.filter((p) => p.id !== player.id && !p.bankrupt));
  let carried = $derived(Object.entries(player.inventory ?? {}).filter(([, count]) => count > 0));
  let partner = $derived(partners.find((p) => p.id === partnerId));
  let partnerItems = $derived(Object.entries(partner?.inventory ?? {}).filter(([, count]) => count > 0));

  let canSend = $derived(Boolean(partnerId) && Boolean(giveItem || giveGold > 0 || askItem || askGold > 0));

  function send() {
    if (!canSend) return;
    void onPropose({
      toPlayerId: partnerId,
      offerItems: giveItem ? { [giveItem]: 1 } : undefined,
      offerResources: giveGold > 0 && currency ? { [currency]: giveGold } : undefined,
      requestItems: askItem ? { [askItem]: 1 } : undefined,
      requestResources: askGold > 0 && currency ? { [currency]: askGold } : undefined,
    });
    open = false;
    giveItem = ''; askItem = ''; giveGold = 0; askGold = 0;
  }
</script>

{#if partners.length > 0}
  <section class="trade">
    {#if !open}
      <button class="open" onclick={() => (open = true)}>{t('trade.propose')}</button>
    {:else}
      <h3>{t('trade.propose')}</h3>
      <label>
        {t('trade.with')}
        <select bind:value={partnerId}>
          <option value="">{t('inspector.choose')}</option>
          {#each partners as candidate (candidate.id)}<option value={candidate.id}>{candidate.name}</option>{/each}
        </select>
      </label>

      <fieldset>
        <legend>{t('trade.youGive')}</legend>
        <select bind:value={giveItem}>
          <option value="">—</option>
          {#each carried as [id, count] (id)}<option value={id}>{items[id]?.title ?? id} ({count})</option>{/each}
        </select>
        {#if currency}
          <input type="number" min="0" bind:value={giveGold} aria-label={currency} />
        {/if}
      </fieldset>

      <fieldset>
        <legend>{t('trade.youAsk')}</legend>
        <select bind:value={askItem} disabled={!partnerId}>
          <option value="">—</option>
          {#each partnerItems as [id, count] (id)}<option value={id}>{items[id]?.title ?? id} ({count})</option>{/each}
        </select>
        {#if currency}
          <input type="number" min="0" bind:value={askGold} aria-label={currency} />
        {/if}
      </fieldset>

      <div class="actions">
        <button class="send" onclick={send} disabled={!canSend}>{t('trade.send')}</button>
        <button onclick={() => (open = false)}>{t('trade.cancel')}</button>
      </div>
    {/if}
  </section>
{/if}

<style>
  .trade {
    display: grid;
    gap: var(--space-2);
  }
  h3 {
    margin: 0;
    color: var(--accent-strong);
    font-size: var(--text-sm);
    letter-spacing: .08em;
    text-transform: uppercase;
  }
  label, fieldset {
    display: grid;
    gap: var(--space-1);
    margin: 0;
    padding: 0;
    border: 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  fieldset {
    padding: var(--space-2);
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
  }
  legend {
    padding: 0 var(--space-1);
    color: var(--text-faint);
  }
  select, input {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
  }
  button {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-raised);
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    cursor: pointer;
  }
  button:hover:not(:disabled) {
    border-color: var(--accent);
    background: var(--accent-surface);
  }
  button:disabled {
    opacity: .55;
    cursor: not-allowed;
  }
  .send {
    border-color: var(--accent);
    background: var(--accent);
    color: var(--accent-contrast);
    font-weight: var(--weight-bold);
  }
</style>
