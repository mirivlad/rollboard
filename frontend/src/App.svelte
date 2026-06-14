<script lang="ts">
  import { mount } from 'svelte';
  import type { GameDefinition, GameSummary, GameSession } from './lib/types';
  import { api } from './lib/api';
  import { createDefaultGame, createMiniMonopolyDemo, createDungeonRaceDemo, createBranchingDemo, createManualBranchDemo } from './lib/defaults';
  import BoardEditor from './components/BoardEditor.svelte';
  import PlaytestPanel from './components/PlaytestPanel.svelte';

  let view = $state<'list' | 'editor' | 'playtest'>('list');
  let games = $state<GameSummary[]>([]);
  let currentGame = $state<GameDefinition | null>(null);
  let currentSession = $state<GameSession | null>(null);
  let message = $state('');
  let messageType = $state<'error' | 'success' | 'info'>('error');
  let loading = $state(false);
  let serverReady = $state(false);
  let serverChecked = $state(false);

  async function checkServer() {
    serverChecked = false;
    serverReady = false;
    message = 'Connecting to server...';
    messageType = 'info';
    try {
      await api.health();
      serverReady = true;
      message = '';
      await loadGames();
    } catch (e: any) {
      message = 'Cannot reach server: ' + (e.message || 'connection failed');
      messageType = 'error';
    } finally {
      serverChecked = true;
    }
  }

  async function loadGames() {
    loading = true;
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
    loading = true;
    message = '';
    try {
      const g = createMiniMonopolyDemo();
      const saved = await api.createGame(g);
      currentGame = saved;
    } catch (e: any) {
      if (e.message.includes('already exists') || e.message.includes('409')) {
        try {
          currentGame = await api.getGame('mini-monopoly-demo');
        } catch (e2: any) {
          message = 'Demo load failed: ' + e2.message;
          messageType = 'error';
          return;
        }
      } else {
        message = e.message;
        messageType = 'error';
        return;
      }
    } finally {
      loading = false;
    }
    currentSession = null;
    view = 'editor';
  }

  async function createDungeonRace() {
    loading = true;
    message = '';
    try {
      const g = createDungeonRaceDemo();
      const saved = await api.createGame(g);
      currentGame = saved;
    } catch (e: any) {
      if (e.message.includes('already exists') || e.message.includes('409')) {
        try {
          currentGame = await api.getGame('dungeon-race-demo');
        } catch (e2: any) {
          message = 'Demo load failed: ' + e2.message;
          messageType = 'error';
          return;
        }
      } else {
        message = e.message;
        messageType = 'error';
        return;
      }
    } finally {
      loading = false;
    }
    currentSession = null;
    view = 'editor';
  }

  async function createBranching() {
    loading = true;
    message = '';
    try {
      const g = createBranchingDemo();
      const saved = await api.createGame(g);
      currentGame = saved;
    } catch (e: any) {
      if (e.message.includes('already exists') || e.message.includes('409')) {
        try {
          currentGame = await api.getGame('branching-demo');
        } catch (e2: any) {
          message = 'Demo load failed: ' + e2.message;
          messageType = 'error';
          return;
        }
      } else {
        message = e.message;
        messageType = 'error';
        return;
      }
    } finally {
      loading = false;
    }
    currentSession = null;
    view = 'editor';
  }

  async function createManualBranch() {
    loading = true;
    message = '';
    try {
      const g = createManualBranchDemo();
      const saved = await api.createGame(g);
      currentGame = saved;
    } catch (e: any) {
      if (e.message.includes('already exists') || e.message.includes('409')) {
        try {
          currentGame = await api.getGame('manual-branch-demo');
        } catch (e2: any) {
          message = 'Demo load failed: ' + e2.message;
          messageType = 'error';
          return;
        }
      } else {
        message = e.message;
        messageType = 'error';
        return;
      }
    } finally {
      loading = false;
    }
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
    // Normalize board dimensions before save
    if (currentGame.board.cellSize > 0) {
      const computedCols = Math.max(1, Math.floor(currentGame.board.width / currentGame.board.cellSize));
      const computedRows = Math.max(1, Math.floor(currentGame.board.height / currentGame.board.cellSize));
      currentGame.board.width = computedCols * currentGame.board.cellSize;
      currentGame.board.height = computedRows * currentGame.board.cellSize;
    }
    try {
      if (currentGame.id) {
        currentGame = await api.updateGame(currentGame.id, currentGame);
      } else {
        currentGame = await api.createGame(currentGame);
      }
      await loadGames();
      message = 'Game saved';
      messageType = 'success';
    } catch (e: any) {
      message = 'Save failed: ' + (e.message || 'unknown error');
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
      message = 'Validate failed: ' + (e.message || 'unknown error');
      messageType = 'error';
    }
  }

  function startPlaytest() {
    if (!currentGame) return;
    view = 'playtest';
  }

  async function backToList() {
    view = 'list';
    currentGame = null;
    currentSession = null;
    if (serverReady) {
      await loadGames();
    }
  }

  function handleSessionCreated(session: GameSession) {
    currentSession = session;
  }

  $effect(() => {
    if (view === 'list' && !serverChecked) {
      checkServer();
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
      {#if view === 'list' && serverReady}
        <button onclick={newGame}>+ New Game</button>
        <button onclick={createMiniMonopoly}>Demo Mini-Monopoly</button>
        <button onclick={createDungeonRace}>Demo Dungeon Race</button>
        <button onclick={createBranching}>Demo Branching</button>
        <button onclick={createManualBranch}>Demo Manual Branch</button>
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
      {#if !serverChecked}
        <p class="status-msg">Connecting to server...</p>
      {:else if !serverReady}
        <p class="status-msg error">Cannot reach backend server. Make sure the server is running on port 8080.</p>
        <button onclick={checkServer}>Retry Connection</button>
      {:else}
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
      {/if}
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
  .status-msg { text-align: center; padding: 40px; color: #888; }
  .status-msg.error { color: #e94560; }
  .game-list { display: grid; gap: 12px; max-width: 600px; margin: 0 auto; }
  .game-card { padding: 16px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; cursor: pointer; transition: border-color 0.2s; }
  .game-card:hover { border-color: #e94560; }
  .game-card .meta { display: block; font-size: 12px; color: #888; margin-top: 4px; }
  .empty { text-align: center; color: #888; padding: 40px; }
</style>
