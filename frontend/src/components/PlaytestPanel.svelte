<script lang="ts">
  import type { GameDefinition, GameSession, PlayerState, ActionOption } from '../lib/types';
  import { api } from '../lib/api';
  import BoardCanvas from './BoardCanvas.svelte';

  let { currentGame, session, onSessionCreated, onBack }: {
    currentGame: GameDefinition;
    session: GameSession | null;
    onSessionCreated: (s: GameSession) => void;
    onBack: () => void;
  } = $props();

  // --- Setup form state ---
  let error = $state('');
  let loading = $state(false);
  let playerConfigs = $state<{ name: string; color: string }[]>([
    { name: 'Player 1', color: '#e74c3c' },
    { name: 'Player 2', color: '#3498db' },
  ]);

  let playerColors = ['#e74c3c', '#3498db', '#2ecc71', '#f39c12', '#9b59b6', '#1abc9c'];

  function updatePlayerCount(count: number) {
    while (playerConfigs.length < count) {
      const i = playerConfigs.length;
      playerConfigs = [...playerConfigs, { name: `Player ${i + 1}`, color: playerColors[i % playerColors.length] }];
    }
    if (playerConfigs.length > count) {
      playerConfigs = playerConfigs.slice(0, count);
    }
  }

  // --- Session state ---
  let currentSession = $state<GameSession | null>(session);

  // Hotseat phases:
  // 'idle' - initial setup
  // 'turn_intro' - show "Pass to player X" screen
  // 'rolling' - dice roll + movement
  // 'action' - waiting for player action (pendingAction)
  // 'turn_done' - show "Turn complete" screen
  let phase = $state<'idle' | 'turn_intro' | 'rolling' | 'action' | 'turn_done'>('idle');

  let lastLogLength = $state(0);
  let currentPlayer = $derived<PlayerState | undefined>(
    currentSession ? currentSession.state.players[currentSession.state.currentPlayerIndex] : undefined
  );

  let resourceKeys = $derived(
    currentSession ? Object.keys(currentSession.state.players[0]?.resources || {}) : []
  );

  // --- Phase transitions ---
  function showTurnIntro() {
    phase = 'turn_intro';
  }

  function startRollPhase() {
    phase = 'rolling';
    handleRoll();
  }

  function showActionPhase() {
    phase = 'action';
  }

  function completeTurn() {
    phase = 'turn_done';
  }

  function advanceToNextTurn() {
    showTurnIntro();
  }

  // --- API calls ---
  async function startSession() {
    loading = true;
    error = '';
    try {
      const s = await api.startPlaytest(currentGame.id, playerConfigs, 'hotseat');
      currentSession = s;
      lastLogLength = s.state.log.length;
      onSessionCreated(s);
      phase = 'turn_intro';
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleRoll() {
    if (!currentSession) return;
    loading = true;
    error = '';
    try {
      lastLogLength = currentSession.state.log.length;
      const s = await api.rollDice(currentSession.id);
      currentSession = s;
      // After roll, check for pending action
      if (s.state.pendingAction) {
        showActionPhase();
      } else {
        completeTurn();
      }
    } catch (e: any) {
      error = e.message;
      // Still show roll result even on error
      completeTurn();
    } finally {
      loading = false;
    }
  }

  async function handleAction(actionId: string) {
    if (!currentSession) return;
    loading = true;
    error = '';
    try {
      lastLogLength = currentSession.state.log.length;
      const s = await api.performAction(currentSession.id, actionId);
      currentSession = s;
      completeTurn();
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  // Auto-refresh for events
  let autoRefresh = $state(false);
  $effect(() => {
    if (autoRefresh && currentSession?.state.status === 'active') {
      const id = setInterval(async () => {
        if (!currentSession) return;
        try {
          const s = await api.getSession(currentSession.id);
          currentSession = s;
        } catch (_) {}
      }, 2000);
      return () => clearInterval(id);
    }
  });
</script>

<div class="playtest">
  {#if !currentSession}
    <div class="setup">
      <h2>Hotseat Playtest</h2>
      <p>Game: <strong>{currentGame.title}</strong></p>

      <div class="setup-players">
        <label>
          Players:
          <select bind:value={playerConfigs.length} onchange={(e) => updatePlayerCount(Number((e.target as HTMLSelectElement).value))}>
            {#each [2, 3, 4, 5, 6] as n}
              <option value={n}>{n} Players</option>
            {/each}
          </select>
        </label>

        {#each playerConfigs as cfg, i}
          <div class="player-config">
            <span class="dot" style="background: {cfg.color}"></span>
            <input
              bind:value={cfg.name}
              placeholder="Player {i + 1}"
            />
            <input
              type="color"
              bind:value={cfg.color}
            />
          </div>
        {/each}
      </div>

      <button onclick={startSession} disabled={loading}>
        {loading ? 'Starting...' : 'Start Playtest'}
      </button>
    </div>

  {:else if currentSession.state.status === 'finished'}
    <div class="game-over">
      <h2>Game Over!</h2>
      <div class="winner-banner">
        {currentSession!.state.players.find(p => p.id === currentSession!.state.winnerPlayerId)?.name} wins!
      </div>
      <div class="player-stats">
        {#each currentSession.state.players as player}
          <div class="final-player" class:winner={currentSession.state.winnerPlayerId === player.id}>
            <span class="dot" style="background: {player.color}"></span>
            <span class="name">{player.name}</span>
            <span class="resources">
              {#each Object.entries(player.resources) as [res, val]}
                {res}: {val}
              {/each}
            </span>
          </div>
        {/each}
      </div>
      <button onclick={onBack}>Back to Editor</button>
    </div>

  {:else}
    {#if phase === 'turn_intro'}
      <div class="turn-intro">
        <div class="current-player-card" style="border-color: {currentPlayer?.color}">
          <div class="big-dot" style="background: {currentPlayer?.color}"></div>
          <h2>{currentPlayer?.name}'s Turn</h2>
          <p>Round {currentSession.state.roundNumber} · Turn {currentSession.state.turnNumber}</p>
          <p class="pass-msg">Pass control to {currentPlayer?.name}</p>
          <button class="primary-btn" onclick={startRollPhase}>
            Start Turn
          </button>
        </div>

        <div class="quick-resources">
          <h3>Players</h3>
          {#each currentSession.state.players as player, i}
            <div class="player-row" class:active={i === currentSession.state.currentPlayerIndex}>
              <span class="dot" style="background: {player.color}"></span>
              <span class="name">{player.name}</span>
              <span class="res-values">
                {#each resourceKeys as res}
                  {res}: <strong>{player.resources[res]}</strong>
                {/each}
              </span>
              {#if player.bankrupt}
                <span class="badge bankrupt">BANKRUPT</span>
              {/if}
            </div>
          {/each}
        </div>
      </div>

    {:else if phase === 'rolling'}
      <div class="rolling-screen">
        <p>Rolling dice...</p>
      </div>

    {:else if phase === 'action'}
      <div class="game-ui">
        <div class="sidebar">
          <div class="panel">
            <h3>Players</h3>
            {#each currentSession.state.players as player, i}
              <div class="player-row" class:active={i === currentSession.state.currentPlayerIndex && !player.bankrupt} class:bankrupt={player.bankrupt}>
                <span class="dot" style="background: {player.color}"></span>
                <span class="name">{player.name}</span>
                <span class="res-values">
                  {#each resourceKeys as res}
                    {res}: <strong>{player.resources[res]}</strong>
                  {/each}
                </span>
                {#if player.bankrupt}
                  <span class="badge bankrupt">DEAD</span>
                {/if}
              </div>
            {/each}
          </div>

          <div class="panel actions">
            {#if currentSession.state.pendingAction}
              <h3>{currentSession.state.pendingAction.title || 'Action Required'}</h3>
              {#each currentSession.state.pendingAction.options || [] as opt}
                <button
                  class="action-btn"
                  onclick={() => handleAction(opt.id)}
                  disabled={loading}
                >
                  {opt.title}
                </button>
              {/each}
            {/if}
          </div>

          <div class="panel log-panel">
            <h3>Event Log</h3>
            <div class="log">
              {#each [...currentSession.state.log].reverse() as event}
                <div class="log-entry log-{event.type}">{event.message}</div>
              {/each}
            </div>
          </div>
        </div>

        <div class="board-area">
          <BoardCanvas
            board={currentGame.board}
            players={currentSession.state.players}
            cellStates={currentSession.state.cellStates}
            mode="select"
          />
        </div>
      </div>

    {:else if phase === 'turn_done'}
      <div class="turn-done">
        <h2>Turn Complete</h2>
        <div class="turn-summary">
          {#each [...currentSession.state.log].reverse().slice(0, 5) as event}
            <div class="log-entry">{event.message}</div>
          {/each}
        </div>
        <button class="primary-btn" onclick={advanceToNextTurn}>
          Pass to Next Player
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .playtest { height: calc(100vh - 120px); }

  /* Setup screen */
  .setup { max-width: 480px; margin: 20px auto; padding: 24px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; }
  .setup h2 { color: #e94560; margin: 0 0 16px; text-align: center; }
  .setup p { text-align: center; margin: 0 0 16px; }
  .setup-players label { display: block; text-align: center; margin-bottom: 16px; color: #aaa; }
  .setup-players select { margin-left: 8px; padding: 6px 12px; background: #0d1b2a; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; }
  .player-config { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
  .player-config input { flex: 1; padding: 6px 8px; background: #0d1b2a; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; }
  .player-config input[type="color"] { width: 40px; height: 32px; padding: 2px; background: transparent; border: 1px solid #0f3460; border-radius: 4px; cursor: pointer; }
  .setup > button { display: block; width: 100%; padding: 12px; background: #e94560; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 16px; margin-top: 12px; }

  /* Turn intro screen */
  .turn-intro { display: flex; gap: 24px; max-width: 700px; margin: 40px auto; align-items: flex-start; }
  .current-player-card { flex: 1; padding: 32px; background: #16213e; border: 3px solid; border-radius: 12px; text-align: center; }
  .big-dot { width: 48px; height: 48px; border-radius: 50%; margin: 0 auto 12px; }
  .current-player-card h2 { margin: 0; font-size: 24px; }
  .pass-msg { color: #f39c12; font-weight: bold; margin: 12px 0; }
  .primary-btn { padding: 14px 32px; background: #e94560; color: white; border: none; border-radius: 8px; cursor: pointer; font-size: 18px; margin-top: 16px; }
  .primary-btn:hover { background: #d63851; }

  /* Quick resources on intro */
  .quick-resources { width: 280px; padding: 16px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; }
  .quick-resources h3 { margin: 0 0 12px; color: #e94560; }

  /* Player row used in multiple places */
  .player-row { display: flex; align-items: center; gap: 8px; padding: 8px; border-radius: 6px; margin-bottom: 4px; font-size: 13px; }
  .player-row.active { background: #1a2a4a; border: 1px solid #e94560; }
  .player-row.bankrupt { opacity: 0.5; }
  .dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
  .name { flex: 1; }
  .res-values { font-size: 11px; color: #aaa; }
  .res-values strong { color: #4CAF50; }
  .badge { font-size: 10px; padding: 2px 6px; border-radius: 3px; font-weight: bold; }
  .badge.bankrupt { background: #5c1a1a; color: #e94560; }

  /* Rolling screen */
  .rolling-screen { display: flex; align-items: center; justify-content: center; height: 200px; font-size: 24px; color: #f39c12; }

  /* Action screen = game UI */
  .game-ui { display: flex; gap: 12px; height: 100%; }
  .sidebar { width: 280px; display: flex; flex-direction: column; gap: 8px; overflow-y: auto; flex-shrink: 0; }
  .panel { padding: 12px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; }
  .panel h3 { margin: 0 0 8px; color: #e94560; font-size: 14px; }

  .actions { text-align: center; }
  .action-btn { display: block; width: 100%; padding: 12px; margin-bottom: 8px; background: #4CAF50; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 16px; }
  .action-btn:hover { opacity: 0.9; }
  .log-panel { flex: 1; overflow: hidden; display: flex; flex-direction: column; }
  .log { flex: 1; overflow-y: auto; font-size: 11px; }
  .log-entry { padding: 4px 0; border-bottom: 1px solid #0f3460; color: #ccc; }
  .log-dice_roll { color: #f39c12; }
  .log-gain_resource, .log-game_start { color: #4CAF50; }
  .log-lose_resource, .log-transfer_resource { color: #e74c3c; }
  .log-game_over { color: #e94560; font-weight: bold; }
  .board-area { flex: 1; overflow: auto; border: 1px solid #0f3460; border-radius: 8px; background: #0d1b2a; }

  /* Turn done screen */
  .turn-done { max-width: 500px; margin: 60px auto; padding: 32px; background: #16213e; border: 1px solid #0f3460; border-radius: 12px; text-align: center; }
  .turn-done h2 { color: #e94560; margin: 0 0 20px; }
  .turn-summary { text-align: left; margin-bottom: 20px; }

  /* Game over */
  .game-over { max-width: 500px; margin: 40px auto; padding: 32px; background: #16213e; border: 1px solid #0f3460; border-radius: 12px; text-align: center; }
  .game-over h2 { color: #e94560; margin: 0 0 16px; }
  .winner-banner { font-size: 28px; color: #FFD700; margin-bottom: 24px; }
  .player-stats { text-align: left; margin-bottom: 24px; }
  .final-player { display: flex; align-items: center; gap: 8px; padding: 8px; border-radius: 6px; margin-bottom: 4px; }
  .final-player.winner { background: #1a5a1a; }
  .final-player .resources { font-size: 12px; color: #aaa; }
  .game-over button { padding: 12px 24px; background: #4CAF50; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 16px; }
</style>
