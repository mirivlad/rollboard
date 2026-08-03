<script lang="ts">
  import type { GameDefinition } from '../lib/types';
  import { i18n } from '../lib/i18n.svelte';

  type Props = {
    game: GameDefinition;
    onContinue: (game: GameDefinition) => void | Promise<void>;
    onAdvanced: (game: GameDefinition) => void | Promise<void>;
  };

  let { game, onContinue, onAdvanced }: Props = $props();
  let setupGameID = $state('');
  let title = $state('');
  let diceCount = $state(1);
  let diceSides = $state(6);
  let t = $derived(i18n.t);

  $effect(() => {
    if (game.id === setupGameID) return;
    setupGameID = game.id;
    title = game.title;
    diceCount = game.rules.dice.count;
    diceSides = game.rules.dice.sides;
  });

  function updatedGame(): GameDefinition {
    return {
      ...game,
      title: title.trim() || t('game.untitled'),
      rules: {
        ...game.rules,
        dice: {
          count: Math.min(10, Math.max(1, Math.floor(diceCount || 1))),
          sides: Math.min(100, Math.max(2, Math.floor(diceSides || 2))),
        },
      },
    };
  }
</script>

<section class="setup" aria-label={t('setup.panelLabel')}>
  <p class="eyebrow">{t('setup.eyebrow')}</p>
  <h1>{t('setup.title')}</h1>
  <p>{t('setup.intro')}</p>

  <form onsubmit={(event) => { event.preventDefault(); void onContinue(updatedGame()); }}>
    <label>{t('setup.gameTitle')}<input aria-label={t('setup.gameTitle')} bind:value={title} maxlength="120" required /></label>
    <div class="dice">
      <label>{t('setup.diceCount')}<input aria-label={t('setup.diceCount')} type="number" bind:value={diceCount} min="1" max="10" /></label>
      <label>{t('setup.diceSides')}<input aria-label={t('setup.diceSides')} type="number" bind:value={diceSides} min="2" max="100" /></label>
    </div>
    <div class="actions">
      <button class="btn btn-primary" type="submit">{t('setup.continue')}</button>
      <button class="btn" type="button" onclick={() => void onAdvanced(updatedGame())}>{t('setup.advanced')}</button>
    </div>
  </form>
</section>

<style>
  .setup{width:min(680px,100%);margin:3rem auto;padding:2rem;border:1px solid var(--border);border-radius:16px;background:var(--surface)}.eyebrow{margin:0;color:var(--accent-strong);font-size:.75rem;font-weight:800;letter-spacing:.12em}.setup h1{margin:.4rem 0}.setup>p:not(.eyebrow){color:var(--text-muted);line-height:1.5}.setup form{display:grid;gap:1rem;margin-top:1.5rem}.setup label{display:grid;gap:.4rem;color:var(--text)}.setup input{box-sizing:border-box;width:100%;padding:.7rem;border:1px solid var(--border-strong);border-radius:8px;background:var(--surface-sunken);color:var(--text);font:inherit}.dice{display:grid;grid-template-columns:1fr 1fr;gap:1rem}.actions{display:flex;flex-wrap:wrap;gap:.7rem;margin-top:.5rem}@media(max-width:560px){.dice{grid-template-columns:1fr}}
</style>
