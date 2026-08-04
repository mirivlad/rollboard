<script lang="ts">
  import type { GameSession, PlayerState } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';

  type Props = {
    session: GameSession;
    player: PlayerState;
    /** Only the player whose turn it is may equip or use anything. */
    canAct: boolean;
    onAction: (actionId: string) => void | Promise<void>;
  };

  let { session, player, canAct, onAction }: Props = $props();
  let t = $derived(i18n.t);
  let items = $derived(session.definition.rules.items ?? {});
  let slots = $derived(session.definition.rules.equipmentSlots ?? []);
  let carried = $derived(Object.entries(player.inventory ?? {}).filter(([, count]) => count > 0));

  /**
   * The stored value plus every bonus from what is worn. The server computes
   * the same sum when it resolves a rule; this is only for display.
   */
  function effective(resource: string): number {
    let total = player.resources[resource] ?? 0;
    for (const itemId of Object.values(player.equipped ?? {})) {
      total += items[itemId]?.bonuses?.[resource] ?? 0;
    }
    return total;
  }

  function bonusFor(resource: string): number {
    return effective(resource) - (player.resources[resource] ?? 0);
  }

  function describeBonuses(itemId: string): string {
    const bonuses = items[itemId]?.bonuses ?? {};
    const parts = Object.entries(bonuses).map(([name, value]) => `${value > 0 ? '+' : ''}${value} ${name}`);
    return parts.join(', ');
  }
</script>

<section class="inventory">
  <h3>{t('rpg.stats')}</h3>
  <dl class="stats">
    {#each Object.keys(player.resources) as resource (resource)}
      <div class="stat">
        <dt>{resource}</dt>
        <dd>
          {effective(resource)}
          {#if bonusFor(resource) !== 0}
            <!-- Showing the split makes it obvious which equipment is carrying
                 the character, rather than presenting one opaque number. -->
            <span class="bonus">({player.resources[resource]}{bonusFor(resource) > 0 ? '+' : ''}{bonusFor(resource)})</span>
          {/if}
        </dd>
      </div>
    {/each}
  </dl>

  {#if slots.length > 0}
    <h3>{t('rpg.equipped')}</h3>
    <ul class="slots">
      {#each slots as slot (slot)}
        {@const wornId = player.equipped?.[slot]}
        <li>
          <span class="slot-name">{slot}</span>
          {#if wornId}
            <span class="worn">{items[wornId]?.title ?? wornId}</span>
            {#if canAct}
              <button class="small" onclick={() => onAction(`unequip:${slot}`)}>{t('rpg.unequip')}</button>
            {/if}
          {:else}
            <span class="empty">{t('rpg.slotEmpty')}</span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  <h3>{t('rpg.inventory')}</h3>
  {#if carried.length === 0}
    <p class="empty">{t('rpg.packEmpty')}</p>
  {:else}
    <ul class="items">
      {#each carried as [itemId, count] (itemId)}
        {@const item = items[itemId]}
        <li>
          <div class="item-head">
            <span class="item-title">{item?.title ?? itemId}</span>
            {#if count > 1}<span class="count">×{count}</span>{/if}
          </div>
          {#if describeBonuses(itemId)}<span class="item-bonus">{describeBonuses(itemId)}</span>{/if}
          {#if canAct}
            <div class="item-actions">
              {#if item?.slot}
                <button class="small" onclick={() => onAction(`equip:${itemId}`)}>{t('rpg.equip')}</button>
              {/if}
              {#if item?.consumable}
                <button class="small" onclick={() => onAction(`use:${itemId}`)}>{t('rpg.use')}</button>
              {/if}
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .inventory {
    display: grid;
    gap: var(--space-2);
  }
  h3 {
    margin: var(--space-2) 0 0;
    color: var(--accent-strong);
    font-size: var(--text-sm);
    letter-spacing: .08em;
    text-transform: uppercase;
  }
  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
    gap: var(--space-1);
    margin: 0;
  }
  .stat {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    font-size: var(--text-sm);
  }
  dt {
    color: var(--text-muted);
  }
  dd {
    margin: 0;
    color: var(--text);
    font-weight: var(--weight-bold);
  }
  .bonus {
    color: var(--success);
    font-weight: var(--weight-normal);
    font-size: var(--text-xs);
  }
  .slots, .items {
    display: grid;
    gap: var(--space-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .slots li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }
  .slot-name {
    color: var(--text-faint);
    text-transform: capitalize;
    min-width: 4.5rem;
  }
  .worn {
    color: var(--text);
    font-weight: var(--weight-medium);
  }
  .empty {
    color: var(--text-faint);
    font-size: var(--text-sm);
    font-style: italic;
  }
  .items li {
    display: grid;
    gap: var(--space-1);
    padding: var(--space-2);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
  }
  .item-head {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
  }
  .item-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }
  .count {
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .item-bonus {
    color: var(--success);
    font-size: var(--text-xs);
  }
  .item-actions {
    display: flex;
    gap: var(--space-1);
  }
  .small {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-raised);
    color: var(--text);
    font: inherit;
    font-size: var(--text-xs);
    cursor: pointer;
  }
  .small:hover {
    border-color: var(--accent);
    background: var(--accent-surface);
  }
</style>
