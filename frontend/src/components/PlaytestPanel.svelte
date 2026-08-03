<script lang="ts">
  import type { GameDefinition, GameSession, PlayerState } from '../lib/types';
  import { api } from '../lib/api';
  import { errorMessage, i18n } from '../lib/i18n.svelte';
  import BoardView from './BoardView.svelte';

  let { currentGame, session, onSessionCreated, onBack }: {
    currentGame: GameDefinition;
    session: GameSession | null;
    onSessionCreated: (s: GameSession) => void;
    onBack: () => void;
  } = $props();

  // --- Setup form state ---
  let error = $state('');
  let loading = $state(false);
  let t = $derived(i18n.t);
  let playerConfigs = $state<{ name: string; color: string }[]>([
    { name: '', color: '#e74c3c' },
    { name: '', color: '#3498db' },
  ]);

  let playerColors = ['#e74c3c', '#3498db', '#2ecc71', '#f39c12', '#9b59b6', '#1abc9c'];

  function updatePlayerCount(count: number) {
    while (playerConfigs.length < count) {
      const i = playerConfigs.length;
      playerConfigs = [...playerConfigs, { name: '', color: playerColors[i % playerColors.length] }];
    }
    if (playerConfigs.length > count) {
      playerConfigs = playerConfigs.slice(0, count);
    }
  }

  // --- Session state ---
  let currentSession = $state<GameSession | null>(null);

  // Sync when prop session changes (e.g. re-opening an existing session)
  let lastSessionId = $state<string | null>(null);
  $effect(() => {
    const s = session;
    if (s && s.id !== lastSessionId) {
      currentSession = s;
      lastSessionId = s.id;
      if (s.state.status === 'active') {
        phase = 'turn_intro';
      }
    }
  });

  // --- Dice ---
  let diceState = $state<'idle' | 'rolling' | 'result'>('idle');
  let lastRolls = $state<number[]>([]);
  let lastTotal = $state(0);
  let diceTimerId = $state<ReturnType<typeof setTimeout> | null>(null);

  // --- Token animation ---
  let animState = $state<{
    playerId: string;
    path: string[];
    step: number;
    session: GameSession;
  } | null>(null);
  let animTimerId = $state<ReturnType<typeof setTimeout> | null>(null);

  // --- Hotseat phases ---
  let phase = $state<'idle' | 'turn_intro' | 'playing' | 'turn_done'>('idle');

  let lastLogLength = $state(0);
  let currentPlayer = $derived<PlayerState | undefined>(
    currentSession ? currentSession.state.players[currentSession.state.currentPlayerIndex] : undefined
  );

  let diceCount = $derived(currentSession?.definition.rules.dice.count ?? 1);
  let diceSides = $derived(currentSession?.definition.rules.dice.sides ?? 6);
  let diceLabel = $derived(`${diceCount}d${diceSides}`);

  // Pip representation for d6, number for any other die
  function dieFace(val: number): string {
    if (diceSides === 6) {
      return ['', '\u2680', '\u2681', '\u2682', '\u2683', '\u2684', '\u2685'][val] || String(val);
    }
    return String(val);
  }

  let resourceKeys = $derived(
    currentSession ? Object.keys(currentSession.state.players[0]?.resources || {}) : []
  );

  let displayPlayers = $derived.by((): PlayerState[] => {
    const s = currentSession;
    if (!s) return [];
    const players = s.state.players;
    const a = animState;
    if (!a) return players;
    return players.map(p => {
      if (p.id !== a.playerId) return p;
      const animCellId = a.path[Math.min(a.step, a.path.length - 1)];
      return { ...p, positionCellId: animCellId };
    });
  });

  let isAnimating = $derived(animState !== null && animState.step < animState.path.length);

  // Cleanup timers when session changes (e.g. HMR reload, game switch)
  let prevSessionId = $state<string | null>(null);
  $effect(() => {
    const sid = currentSession?.id || null;
    if (sid !== prevSessionId) {
      cancelDice();
      prevSessionId = sid;
    }
  });

  // --- Phase transitions ---
  function showTurnIntro() {
    phase = 'turn_intro';
    diceState = 'idle';
    lastRolls = [];
    lastTotal = 0;
  }

  function startTurn() {
    phase = 'playing';
  }

  async function handleRollStart() {
    const sess = currentSession;
    if (!sess) return;
    diceState = 'rolling';
    lastRolls = [];
    lastTotal = 0;
    lastLogLength = sess.state.log.length;

    diceTimerId = setTimeout(async () => {
      diceTimerId = null;
      try {
        const s = await api.rollDice(sess.id);

        // Extract dice roll result from new events
        const newEvents = s.state.log.slice(lastLogLength);
        const diceEvt = newEvents.find(e => e.type === 'dice_roll');
        if (diceEvt?.payload) {
          lastRolls = diceEvt.payload.rolls || [];
          lastTotal = diceEvt.payload.total || 0;
        }

        // Extract move event
        const moveEvt = newEvents.find(e => e.type === 'move');
        const path: string[] = moveEvt?.payload?.path;
        const playerId: string = moveEvt?.payload?.playerId || sess.state.players[sess.state.currentPlayerIndex].id;

        diceState = 'result';

        // Pause briefly so user sees dice result, then animate
        diceTimerId = setTimeout(() => {
          diceTimerId = null;
          if (path && path.length > 0) {
            animState = { playerId, path, step: 0, session: s };
            startAnimStep();
          } else {
            currentSession = s;
            checkPhaseAfterRoll(s);
          }
        }, 600);
      } catch (e: unknown) {
        error = errorMessage(t, e);
        diceState = 'idle';
        const refreshed = await api.getSession(sess.id);
        currentSession = refreshed;
        checkPhaseAfterRoll(refreshed);
      }
    }, 600);
  }

  function startAnimStep() {
    animTimerId = setTimeout(() => {
      animTimerId = null;
      if (!animState) return;
      const nextStep = animState.step + 1;
      if (nextStep >= animState.path.length) {
        // Animation done
        const s = animState.session;
        animState = null;
        currentSession = s;
        checkPhaseAfterRoll(s);
      } else {
        animState = { ...animState, step: nextStep };
        startAnimStep();
      }
    }, 300);
  }

  function checkPhaseAfterRoll(s: GameSession) {
    if (s.state.pendingAction) {
      phase = 'playing';
    } else {
      phase = 'turn_done';
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
      phase = 'turn_done';
    } catch (e: unknown) {
      error = errorMessage(t, e);
    } finally {
      loading = false;
    }
  }

  async function handleAdvanceTurn() {
    if (!currentSession) return;
    loading = true;
    error = '';
    try {
      const s = await api.nextTurn(currentSession.id);
      currentSession = s;
      showTurnIntro();
    } catch (e: unknown) {
      error = errorMessage(t, e);
    } finally {
      loading = false;
    }
  }

  function cancelDice() {
    if (diceTimerId) {
      clearTimeout(diceTimerId);
      diceTimerId = null;
    }
    if (animTimerId) {
      clearTimeout(animTimerId);
      animTimerId = null;
    }
    animState = null;
    diceState = 'idle';
  }

  // --- Session lifecycle ---
  async function startSession() {
    loading = true;
    error = '';
    try {
      // Names are left blank in the form so the placeholder shows the
      // localised default; fill them in only when the session is created.
      const named = playerConfigs.map((cfg, i) => ({
        ...cfg,
        name: cfg.name.trim() || t('playtest.playerName', { number: i + 1 }),
      }));
      const s = await api.startPlaytest(currentGame.id, named, 'hotseat');
      currentSession = s;
      lastLogLength = s.state.log.length;
      onSessionCreated(s);
      phase = 'turn_intro';
    } catch (e: unknown) {
      error = errorMessage(t, e);
    } finally {
      loading = false;
    }
  }

  // --- Cleanup ---
  $effect(() => {
    return () => {
      if (diceTimerId) clearTimeout(diceTimerId);
      if (animTimerId) clearTimeout(animTimerId);
    };
  });
</script>

<div class="playtest">
  {#if !currentSession}
    <div class="setup">
      <h2>{t('playtest.title')}</h2>
      <p>{t('playtest.game', { title: currentGame.title })}</p>

      <div class="setup-players">
        <label>
          {t('playtest.playersLabel')}
          <select bind:value={playerConfigs.length} onchange={(e) => updatePlayerCount(Number((e.target as HTMLSelectElement).value))}>
            {#each [2, 3, 4, 5, 6] as n}
              <option value={n}>{t('playtest.playerCount', { count: n })}</option>
            {/each}
          </select>
        </label>

        {#each playerConfigs as cfg, i}
          <div class="player-config">
            <span class="dot" style="background: {cfg.color}"></span>
            <input
              bind:value={cfg.name}
              placeholder={t('playtest.playerName', { number: i + 1 })}
            />
            <input
              type="color"
              bind:value={cfg.color}
            />
          </div>
        {/each}
      </div>

      <button onclick={startSession} disabled={loading}>
        {loading ? t('playtest.starting') : t('playtest.start')}
      </button>
    </div>

  {:else if currentSession.state.status === 'finished'}
    <div class="game-over">
      <h2>{t('playtest.gameOver')}</h2>
      <div class="winner-banner">
        {t('playtest.wins', { name: currentSession!.state.players.find(p => p.id === currentSession!.state.winnerPlayerId)?.name ?? '' })}
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
      <button onclick={onBack}>{t('playtest.backToEditor')}</button>
    </div>

  {:else}
    {#if phase === 'turn_intro'}
      <div class="turn-intro">
        <div class="current-player-card" style="border-color: {currentPlayer?.color}">
          <div class="big-dot" style="background: {currentPlayer?.color}"></div>
          <h2>{t('playtest.turnOf', { name: currentPlayer?.name ?? '' })}</h2>
          <p>{t('playtest.roundTurn', { round: currentSession.state.roundNumber, turn: currentSession.state.turnNumber })}</p>
          <p class="pass-msg">{t('playtest.passControl', { name: currentPlayer?.name ?? '' })}</p>
          <button class="primary-btn" onclick={startTurn}>
            {t('playtest.startTurn')}
          </button>
        </div>

        <div class="quick-resources">
          <h3>{t('playtest.playersLabel')}</h3>
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
                <span class="badge bankrupt">{t('playtest.bankrupt')}</span>
              {/if}
            </div>
          {/each}
        </div>
      </div>

    {:else}
      <div class="game-ui">
        <div class="sidebar">
          <div class="panel">
            <div class="dice-rule">{t('playtest.diceRule', { dice: diceLabel })}</div>
            <h3>{t('playtest.playersLabel')}</h3>
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
                  <span class="badge bankrupt">{t('playtest.dead')}</span>
                {/if}
              </div>
            {/each}
          </div>

          <div class="panel actions">
            {#if diceState === 'idle' && !isAnimating}
              {#if currentSession.state.pendingAction}
                {#if currentSession.state.pendingAction.type === 'route_choice'}
                  <h3>{t('playtest.choosePath')}</h3>
                {:else}
                  <h3>{currentSession.state.pendingAction.title || t('playtest.actionRequired')}</h3>
                {/if}
                {#each currentSession.state.pendingAction.options || [] as opt}
                  <button
                    class="action-btn"
                    onclick={() => handleAction(opt.id)}
                    disabled={loading}
                  >
                    {opt.title}
                  </button>
                {/each}
              {:else}
                <button class="roll-btn" onclick={handleRollStart} disabled={loading}>
                  {t('playtest.roll')}
                </button>
              {/if}
            {:else if diceState === 'rolling'}
              <div class="dice-rolling">
                <div class="dice-animation">
                  {#each Array(diceCount) as _}
                    <div class="dice-face rolling">?</div>
                  {/each}
                </div>
                <p>{t('playtest.rolling')}</p>
              </div>
            {:else if diceState === 'result' && !isAnimating}
              <div class="dice-result">
                <div class="dice-row">
                  {#each lastRolls as roll, i}
                    <div class="dice-face result">{dieFace(roll)}</div>
                  {/each}
                </div>
                <p class="dice-total">{t('playtest.total')} <strong>{lastTotal}</strong></p>
              </div>
              {#if currentSession.state.pendingAction}
                {#if currentSession.state.pendingAction.type === 'route_choice'}
                  <h3>{t('playtest.choosePath')}</h3>
                {:else}
                  <h3>{currentSession.state.pendingAction.title || t('playtest.actionRequired')}</h3>
                {/if}
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
            {/if}
            {#if isAnimating}
              <div class="animating">
                <p>{t('playtest.moving')}</p>
              </div>
            {/if}
          </div>

          <div class="panel log-panel">
            <h3>{t('playtest.eventLog')}</h3>
            <div class="log">
              {#each [...currentSession.state.log].reverse() as event}
                <div class="log-entry log-{event.type}">{event.message}</div>
              {/each}
            </div>
          </div>
        </div>

        <div class="board-area">
          <BoardView
            board={currentSession.definition.board}
            players={displayPlayers}
            cellStates={currentSession.state.cellStates}
          />
        </div>
      </div>

      {#if phase === 'turn_done' && !isAnimating}
        <div class="turn-done-bar">
          <button class="primary-btn" onclick={handleAdvanceTurn} disabled={loading}>
            {loading ? t('playtest.advancing') : t('playtest.passNext')}
          </button>
        </div>
      {/if}
    {/if}
  {/if}
</div>

<style>
  .playtest { height: calc(100vh - 120px); display: flex; flex-direction: column; }

  /* Setup screen */
  .setup { max-width: 480px; margin: 20px auto; padding: 24px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; }
  .setup h2 { color: #e94560; margin: 0 0 16px; text-align: center; }
  .setup p { text-align: center; margin: 0 0 16px; }
  .setup-players label { display: block; text-align: center; margin-bottom: 16px; color: #aaa; }
  .setup-players select { margin-left: 8px; padding: 6px 12px; background: #0d1b2a; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; }
  .player-config { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
  .player-config input { flex: 1; padding: 6px 8px; background: #0d1b2a; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; }
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

  /* Player row */
  .player-row { display: flex; align-items: center; gap: 8px; padding: 8px; border-radius: 6px; margin-bottom: 4px; font-size: 13px; }
  .player-row.active { background: #1a2a4a; border: 1px solid #e94560; }
  .player-row.bankrupt { opacity: 0.5; }
  .dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
  .name { flex: 1; }
  .res-values { font-size: 11px; color: #aaa; }
  .res-values strong { color: #4CAF50; }
  .badge { font-size: 10px; padding: 2px 6px; border-radius: 3px; font-weight: bold; }
  .badge.bankrupt { background: #5c1a1a; color: #e94560; }

  /* Game UI layout */
  .game-ui { display: flex; gap: 12px; flex: 1; min-height: 0; }
  .sidebar { width: 280px; display: flex; flex-direction: column; gap: 8px; overflow-y: auto; flex-shrink: 0; }
  .panel { padding: 12px; background: #16213e; border: 1px solid #0f3460; border-radius: 8px; }
  .panel h3 { margin: 0 0 8px; color: #e94560; font-size: 14px; }
  .dice-rule { font-size: 12px; color: #f39c12; margin-bottom: 8px; font-family: monospace; }

  .actions { text-align: center; }
  .action-btn { display: block; width: 100%; padding: 12px; margin-bottom: 8px; background: #4CAF50; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 16px; }
  .action-btn:hover { opacity: 0.9; }
  .action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .roll-btn { width: 100%; padding: 16px; background: #f39c12; color: #111; border: none; border-radius: 8px; cursor: pointer; font-size: 18px; font-weight: bold; }
  .roll-btn:hover { background: #e67e22; }
  .roll-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  /* Dice */
  .dice-rolling, .dice-result, .animating { padding: 12px 0; }
  .dice-animation { display: flex; justify-content: center; gap: 8px; }
  .dice-row { display: flex; justify-content: center; gap: 8px; }
  .dice-face {
    width: 48px; height: 48px; display: flex; align-items: center; justify-content: center;
    background: white; color: #111; border-radius: 8px; font-size: 22px; font-weight: bold;
    box-shadow: 0 2px 6px rgba(0,0,0,0.3);
  }
  .dice-face.rolling { animation: diceShake 0.15s infinite alternate; background: #f5f5f5; color: #666; }
  .dice-face.result { background: #f39c12; color: white; }
  @keyframes diceShake {
    from { transform: rotate(-8deg) scale(0.95); }
    to { transform: rotate(8deg) scale(1.05); }
  }
  .dice-total { font-size: 16px; margin-top: 8px; }
  .dice-total strong { color: #f39c12; font-size: 20px; }

  .animating p { color: #4fc3f7; font-style: italic; }

  .log-panel { flex: 1; overflow: hidden; display: flex; flex-direction: column; }
  .log { flex: 1; overflow-y: auto; font-size: 11px; }
  .log-entry { padding: 4px 0; border-bottom: 1px solid #0f3460; color: #ccc; }
  .log-dice_roll { color: #f39c12; }
  .log-gain_resource, .log-game_start { color: #4CAF50; }
  .log-lose_resource, .log-transfer_resource { color: #e74c3c; }
  .log-game_over { color: #e94560; font-weight: bold; }

  .board-area { flex: 1; overflow: auto; border: 1px solid #0f3460; border-radius: 8px; background: #0d1b2a; }

  /* Turn done bar */
  .turn-done-bar { text-align: center; padding: 12px; border-top: 1px solid #0f3460; background: #16213e; }

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
