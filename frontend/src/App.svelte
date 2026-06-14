<script lang="ts">
  import { mount } from 'svelte';
  import type { GameDefinition, GameSummary, GameSession } from './lib/types';
  import { api } from './lib/api';
  import { createDefaultGame, createMiniMonopolyDemo, createDungeonRaceDemo } from './lib/defaults';
  import BoardEditor from './components/BoardEditor.svelte';
  import PlaytestPanel from './components/PlaytestPanel.svelte';

  let view = $state<'list' | 'editor' | 'playtest'>('list');
  let games = $state<GameSummary[]>([]);
  let currentGame = $state<GameDefinition | null>(null);
  let currentSession = $state<GameSession | null>(null);
  let message = $state('');
  let messageType = $state<'error' | 'success' | 'info'>('error');
  let loading = $state(false);

  async function loadGames() {
    loading = true;
    message = '';
    try {
      games = await api.listGames();
    } catch (e: any) {
      message = e.message;
      messageType = 'error';
    } finally {
      loading = false;
    }
  }

  function newGame() {
    currentGame = createDefaultGame();
    currentSession = null;
    view = 'editor';
  }

  async function createMiniMonopoly() {
    const g = createMiniMonopolyDemo();
    currentGame = g;
    currentSession = null;
    view = 'editor';
  }

  async function createDungeonRace() {
    const g = createDungeonRaceDemo();
    currentGame = g;
    currentSession = null;
    view = 'editor';
  }

  async function openGame(id: string) {
    loading = true;
    message = '';
    try {
      currentGame = await api.getGame(id);
      currentSession = null;
      view = 'editor';
    } catch (e: any) {
      message = e.message;
      messageType = 'error';
    } finally {
      loading = false;
    }
  }

  async function saveGame() {
    if (!currentGame) return;
    loading = true;
    message = '';
    try {
      if (currentGame.id) {
        currentGame = await api.updateGame(currentGame.id, currentGame);
      } else {
        currentGame = await api.createGame(currentGame);
      }
      await loadGames();
    } catch (e: any) {
      message = e.message;
      messageType = 'error';
    } finally {
      loading = false;
    }
  }

  async function validateGame() {
    if (!currentGame) return;
    message = '';
    try {
      if (!currentGame.id) {
        message = 'Save the game first before validating';
        messageType = 'info';
        return;
      }
      const result = await api.validateGame(currentGame.id);
      if (result.valid) {
        message = 'Game is valid!';
        messageType = 'success';
      } else {
        message = 'Validation errors:\n' + (result.errors || []).join('\n');
        messageType = 'error';
      }
    } catch (e: any) {
      message = e.message;
      messageType = 'error';
    }
  }

  function startPlaytest() {
    if (!currentGame) return;
    view = 'playtest';
  }

  function backToList() {
    view = 'list';
    currentGame = null;
    currentSession = null;
  }

  function handleSessionCreated(session: GameSession) {
    currentSession = session;
  }

  $effect(() => {
    if (view === 'list') {
      loadGames();
    }
  });
</script>

<div class="app">
  <header>
    <h1>Rollboard</h1>
    <nav>
      {#if view !== 'list'}
        <button onclick={backToList}>← Games</button>
      {/if}
      {#if view === 'list'}
        <button onclick={newGame}>+ New Game</button>
        <button onclick={createMiniMonopoly}>Demo Mini-Monopoly</button>
        <button onclick={createDungeonRace}>Demo Dungeon Race</button>
      {/if}
      {#if view === 'editor' && currentGame}
        <button onclick={saveGame}>Save</button>
        <button onclick={validateGame}>Validate</button>
        <button onclick={startPlaytest} disabled={!currentGame.id}>Playtest</button>
      {/if}
    </nav>
  </header>

  <main>
    {#if message}
      <div class="message msg-{messageType}">{message}</div>
    {/if}

    {#if view === 'list'}
      <div class="game-list">
        {#if loading}
          <p>Loading...</p>
        {:else if games.length === 0}
          <p class="empty">No games yet. Create a new game or generate a demo.</p>
        {:else}
          {#each games as game}
            <div class="game-card" onclick={() => openGame(game.id)} onkeydown={(e) => e.key === 'Enter' && openGame(game.id)} role="button" tabindex="0">
              <strong>{game.title}</strong>
              <span class="meta">v{game.version} · {game.updatedAt}</span>
            </div>
          {/each}
        {/if}
      </div>
    {:else if view === 'editor' && currentGame}
      <BoardEditor game={currentGame} onsave={saveGame} />
    {:else if view === 'playtest' && currentGame}
      <PlaytestPanel
        {currentGame}
        session={currentSession}
        onSessionCreated={handleSessionCreated}
        onBack={() => { view = 'editor'; }}
      />
    {/if}
  </main>
</div>

<style>
  .app { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
  header { display: flex; align-items: center; gap: 16px; padding: 12px 20px; background: #16213e; border-bottom: 1px solid #0f3460; }
  header h1 { margin: 0; font-size: 20px; color: #e94560; }
  header nav { display: flex; gap: 8px; margin-left: auto; }
  button { padding: 8px 16px; border: 1px solid #0f3460; background: #16213e; color: #e0e0e0; border-radius: 6px; cursor: pointer; font-size: 14px; transition: background 0.2s; }
  button:hover { background: #0f3460; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  main { padding: 20px; }
  .message { padding: 12px; border-radius: 6px; margin-bottom: 12px; white-space: pre-wrap; }
  .msg-error { background: #3e1a1a; border: 1px solid #e94560; color: #e94560; }
  .msg-success { background: #1a3e1a; border: 1px solid #4CAF50; color: #4CAF50; }
  .msg-info { background: #1a2a3e; border: 1px solid #4fc3f7; color: #4fc3f7; }
  .game-list { display: grid; gap: 12px; max-width: 600px; margin: 0 auto; }
  .game-card { padding: 16px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; cursor: pointer; transition: border-color 0.2s; }
  .game-card:hover { border-color: #e94560; }
  .game-card .meta { display: block; font-size: 12px; color: #888; margin-top: 4px; }
  .empty { text-align: center; color: #888; padding: 40px; }
</style>
