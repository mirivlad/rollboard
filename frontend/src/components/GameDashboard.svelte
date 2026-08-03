<script lang="ts">
  import type { CatalogGame } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';

  export type Template = 'blank' | 'mini-monopoly' | 'dungeon-race';

  type Props = {
    displayName: string;
    onCreate: (template: Template, advanced: boolean) => void | Promise<void>;
    games: CatalogGame[];
    onOpen: (gameId: string) => void | Promise<void>;
    busy?: boolean;
  };

  let { displayName, onCreate, games, onOpen, busy = false }: Props = $props();
  let advanced = $state(false);
  let t = $derived(i18n.t);
</script>

<section class="dashboard">
  <header>
    <div>
      <p class="eyebrow">{t('dashboard.eyebrow')}</p>
      <h1>{t('dashboard.title')}</h1>
      <p>{t('dashboard.welcome', { name: displayName })}</p>
    </div>
    <label class="advanced"><input type="checkbox" bind:checked={advanced} /> {t('dashboard.advanced')}</label>
  </header>

  <section class="create" aria-label={t('dashboard.createLabel')}>
    <h2>{t('dashboard.start')}</h2>
    <div class="templates">
      <button onclick={() => onCreate('blank', advanced)} disabled={busy}>
        <strong>{t('dashboard.template.blank')}</strong><span>{t('dashboard.template.blankHint')}</span>
      </button>
      <button onclick={() => onCreate('mini-monopoly', advanced)} disabled={busy}>
        <strong>{t('dashboard.template.miniMonopoly')}</strong><span>{t('dashboard.template.miniMonopolyHint')}</span>
      </button>
      <button onclick={() => onCreate('dungeon-race', advanced)} disabled={busy}>
        <strong>{t('dashboard.template.dungeonRace')}</strong><span>{t('dashboard.template.dungeonRaceHint')}</span>
      </button>
    </div>
  </section>

  <section class="empty" aria-label={t('dashboard.draftListLabel')}>
    <h2>{t('dashboard.drafts')}</h2>
    {#if games.length === 0}
      <p>{t('dashboard.noDrafts')}</p>
    {:else}
      <div class="drafts">
        {#each games as game (game.id)}
          <button class="draft" onclick={() => onOpen(game.id)} disabled={busy} aria-label={t('dashboard.openGame', { title: game.title })}>
            <strong>{game.title}</strong><span>{t('dashboard.updated', { date: new Date(game.updatedAt).toLocaleDateString(i18n.locale) })}</span>
          </button>
        {/each}
      </div>
    {/if}
  </section>
</section>

<style>
  .dashboard { width: min(1120px, 100%); margin: 0 auto; color: #eef3ff; }
  header { display: flex; justify-content: space-between; gap: 1rem; align-items: start; margin-bottom: 2rem; }
  .eyebrow { margin: 0; color: #75d4ff; letter-spacing: .12em; font-size: .72rem; font-weight: 800; }
  h1 { margin: .35rem 0; font-size: clamp(2rem, 5vw, 3rem); }
  header p:not(.eyebrow) { margin: 0; color: #aab6d3; }
  .advanced { display: flex; gap: .5rem; align-items: center; padding: .65rem .8rem; border: 1px solid #30415f; border-radius: 8px; color: #cad5ed; white-space: nowrap; }
  h2 { margin: 0 0 1rem; font-size: 1.1rem; }
  .templates { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1rem; }
  .templates button { min-height: 142px; display: grid; align-content: center; gap: .55rem; padding: 1.1rem; border: 1px solid #2d4165; border-radius: 14px; background: linear-gradient(140deg, #172a49, #101a30); color: #f1f6ff; text-align: left; cursor: pointer; }
  .templates button:hover { border-color: #68cdf5; transform: translateY(-1px); }
  .templates span { color: #a9b9d7; line-height: 1.4; }
  .empty { margin-top: 2rem; padding: 1.5rem; border: 1px dashed #34476b; border-radius: 14px; color: #aebbd4; }.drafts{display:grid;gap:.7rem}.draft{display:flex;justify-content:space-between;gap:1rem;align-items:center;padding:.8rem;border:1px solid #30415f;border-radius:9px;background:#101a30;color:#eef3ff;text-align:left;cursor:pointer;font:inherit}.draft span{color:#aebbd4;font-size:.85rem}
  .empty p { margin-bottom: 0; }
  @media (max-width: 720px) { header { display: grid; } .templates { grid-template-columns: 1fr; } }
</style>
