<script lang="ts">
  import { onMount } from 'svelte';
  import type { Room, RoomMessage } from '../lib/types';
  import { api } from '../lib/api';
  import { errorMessage, i18n } from '../lib/i18n.svelte';
  import { confirmedRollPlayback, type RollPlayback } from '../lib/room-events';
  import { roomCommand } from '../lib/room-command';
  import { acceptsRoomSequence } from '../lib/room-sequence';
  import BoardView from './BoardView.svelte';
  import InviteLink from './InviteLink.svelte';

  type Actor = { kind: 'user' | 'guest'; id: string };
  type Props = { room: Room; canStart: boolean; canModerate: boolean; actor: Actor; onRoom?: (room: Room) => void };
  let { room, canStart, canModerate, actor, onRoom = () => {} }: Props = $props();
  let currentRoom = $state<Room>({ id: '', gameVersionId: '', hostUserId: '', hostMemberId: '', title: '', status: 'lobby', maxPlayers: 2, members: [], sequence: 0 });
  let messages = $state<RoomMessage[]>([]);
  let message = $state('');
  let error = $state('');
  let socket = $state<WebSocket | null>(null);
  let connected = $state(false);
  let pendingRoom = $state<Room | null>(null);
  let activePlayback = $state<RollPlayback | null>(null);
  let playbackStep = $state<number | null>(null);
  let lastRoll = $state<RollPlayback | null>(null);
  let rolling = $state(false);
  let playbackTimer: ReturnType<typeof setTimeout> | undefined;
  let latestRoomSequence = $state(0);
  let t = $derived(i18n.t);

  $effect(() => {
    currentRoom = room;
    latestRoomSequence = Math.max(latestRoomSequence, room.sequence);
  });
  let ownMember = $derived(currentRoom.members.find((member) => member.actorKind === actor.kind && member.actorId === actor.id));
  let currentPlayer = $derived(currentRoom.session?.state.players[currentRoom.session.state.currentPlayerIndex]);
  let pendingAction = $derived(currentRoom.session?.state.pendingAction);
  let displaySession = $derived.by(() => {
    const session = pendingRoom?.session ?? currentRoom.session;
    if (!session || !activePlayback || playbackStep === null) return session;
    const position = activePlayback.positions[Math.min(playbackStep, activePlayback.positions.length - 1)];
    return {
      ...session,
      state: {
        ...session.state,
        players: session.state.players.map((player) => player.id === activePlayback?.playerId ? { ...player, positionCellId: position } : player),
      },
    };
  });
  let canRoll = $derived(Boolean(
    currentRoom.session?.state.status === 'active' &&
    ownMember?.playerId &&
    ownMember.playerId === currentPlayer?.id &&
    !pendingAction &&
    !rolling &&
    playbackStep === null
  ));
  let canResolveAction = $derived(Boolean(
    pendingAction && ownMember?.playerId === pendingAction.playerId
  ));
  function clearPlaybackTimer() {
    if (playbackTimer) clearTimeout(playbackTimer);
    playbackTimer = undefined;
  }
  function finishPlayback() {
    clearPlaybackTimer();
    const nextRoom = pendingRoom;
    pendingRoom = null;
    activePlayback = null;
    playbackStep = null;
    if (nextRoom) {
      currentRoom = nextRoom;
      onRoom(currentRoom);
    }
  }
  function advancePlayback() {
    if (!activePlayback || playbackStep === null) return;
    if (playbackStep < activePlayback.positions.length - 1) {
      playbackStep += 1;
      playbackTimer = setTimeout(advancePlayback, 250);
      return;
    }
    finishPlayback();
  }
  function startPlayback(nextRoom: Room, playback: RollPlayback) {
    clearPlaybackTimer();
    pendingRoom = nextRoom;
    activePlayback = playback;
    lastRoll = playback;
    playbackStep = 0;
    rolling = false;
    playbackTimer = setTimeout(advancePlayback, 450);
  }
  function send(value: object) {
    if (!socket || socket.readyState !== WebSocket.OPEN) { error = t('room.connectionUnavailable'); return false; }
    socket.send(JSON.stringify(value));
    return true;
  }
  function rollDice() { error = ''; rolling = send(roomCommand('roll')); }
  function acceptsIncomingRoom(envelope: { sequence?: unknown }) {
    if (!acceptsRoomSequence(latestRoomSequence, envelope.sequence)) return false;
    latestRoomSequence = envelope.sequence;
    return true;
  }
  function submitMessage() { const body = message.trim(); if (!body) return; send(roomCommand('chat', { body })); message = ''; }
  async function refreshRoom() { currentRoom = await api.getRoom(currentRoom.id); onRoom(currentRoom); }
  async function muteMember(memberID: string, muted: boolean) { error = ''; try { await api.muteRoomMember(currentRoom.id, memberID, muted); await refreshRoom(); } catch (cause) { error = errorMessage(t, cause); } }
  async function removeMember(memberID: string) { error = ''; try { await api.removeRoomMember(currentRoom.id, memberID); await refreshRoom(); } catch (cause) { error = errorMessage(t, cause); } }

  onMount(() => {
    void api.listRoomMessages(room.id).then((value) => messages = value).catch(() => {});
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${location.host}/api/rooms/${encodeURIComponent(room.id)}/ws?since=${room.sequence}`);
    socket = ws;
    ws.onopen = () => connected = true;
    ws.onclose = () => connected = false;
    ws.onerror = () => error = t('room.connectionInterrupted');
    ws.onmessage = (event) => {
      const envelope = JSON.parse(event.data);
      if (envelope.type === 'room_state' && acceptsIncomingRoom(envelope)) { currentRoom = envelope.payload; onRoom(currentRoom); }
      if (envelope.type === 'room_event' && envelope.payload?.session && acceptsIncomingRoom(envelope)) {
        const nextRoom = { ...currentRoom, sequence: envelope.sequence, status: envelope.payload.session.state.status, session: envelope.payload.session };
        const playback = confirmedRollPlayback(envelope.payload.events ?? []);
        if (playback && currentRoom.session) startPlayback(nextRoom, playback);
        else { rolling = false; currentRoom = nextRoom; onRoom(currentRoom); }
      }
      if (envelope.type === 'chat_message') messages = [...messages, envelope.payload];
      if (envelope.type === 'room_error') { rolling = false; error = envelope.details; }
    };
    return () => { clearPlaybackTimer(); ws.close(); };
  });
</script>

<section class="room-play">
  <div class="top"><div><p class="eyebrow">{connected ? t('room.live') : t('room.reconnecting')}</p><h1>{currentRoom.title}</h1><p>{t('room.occupancy', { count: currentRoom.members.length, max: currentRoom.maxPlayers, status: t(`room.status.${currentRoom.status}`) })}</p></div>{#if currentRoom.status === 'lobby' && canStart}<button onclick={() => send(roomCommand('start'))}>{t('room.startGame')}</button>{/if}</div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="layout">
    <div class="game"><h2>{currentRoom.session ? t('room.turn', { number: currentRoom.session.state.turnNumber }) : t('room.waitingLobby')}</h2>{#if currentRoom.session}<p>{t('room.currentPlayer', { name: currentPlayer?.name ?? '' })}</p>{#if rolling}<p class="dice-result" aria-live="polite">{t('room.rolling')}</p>{:else if lastRoll}<p class="dice-result" aria-live="polite">{t('room.rolled', { rolls: lastRoll.rolls.join(' + '), total: lastRoll.total })}</p>{/if}{#if pendingAction}<section class="pending-action"><h3>{pendingAction.title || t('room.chooseAction')}</h3>{#if canResolveAction}{#each pendingAction.options ?? [] as option (option.id)}<button onclick={() => send(roomCommand('action', { actionId: option.id }))}>{option.title}</button>{/each}{:else}<p>{t('room.waitingChoice')}</p>{/if}</section>{:else if canRoll}<button onclick={rollDice}>{t('room.roll')}</button>{:else}<p>{t('room.waitingRoll')}</p>{/if}{#if displaySession}<BoardView board={displaySession.definition.board} players={displaySession.state.players} cellStates={displaySession.state.cellStates} />{/if}{/if}</div>
    <aside>{#if canModerate && currentRoom.status === 'lobby'}<InviteLink roomId={currentRoom.id} />{/if}<section class="roster"><h2>{t('room.players')}</h2>{#each currentRoom.members as member (member.id)}<div class="member"><span>{member.displayName}{member.mutedAt ? ` · ${t('room.muted')}` : ''}</span>{#if canModerate && member.id !== currentRoom.hostMemberId}<div class="member-actions"><button aria-label={member.mutedAt ? t('room.unmuteMember', { name: member.displayName }) : t('room.muteMember', { name: member.displayName })} onclick={() => muteMember(member.id, !member.mutedAt)}>{member.mutedAt ? t('room.unmute') : t('room.mute')}</button><button aria-label={t('room.removeMember', { name: member.displayName })} onclick={() => removeMember(member.id)}>{t('room.remove')}</button></div>{/if}</div>{/each}</section><section class="chat"><h2>{t('room.chat')}</h2><div class="messages">{#each messages as item (item.id)}<p><strong>{item.displayName}</strong><span>{item.body}</span></p>{/each}</div><form onsubmit={(event) => { event.preventDefault(); submitMessage(); }}><label>{t('room.message')}<input bind:value={message} maxlength="1000" placeholder={t('room.messagePlaceholder')} /></label><button disabled={!message.trim()}>{t('room.send')}</button></form></section></aside>
  </div>
</section>

<style>
  .room-play {
    width: min(var(--page-full), 100%);
    margin: 0 auto;
  }

  .top {
    display: flex;
    justify-content: space-between;
    gap: var(--space-4);
    align-items: start;
    margin-bottom: var(--space-4);
  }
  .top h1 {
    margin: var(--space-1) 0;
    font-size: var(--text-2xl);
  }
  .top p {
    margin: 0;
    color: var(--text-muted);
  }

  .eyebrow {
    margin: 0;
    color: var(--accent-strong);
    letter-spacing: .12em;
    font-size: var(--text-xs);
    font-weight: var(--weight-black);
  }

  .layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 320px;
    gap: var(--space-4);
    align-items: start;
  }

  .game,
  aside {
    padding: var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
  }
  .game h2 {
    margin-top: 0;
    font-size: var(--text-xl);
  }

  .dice-result {
    margin: var(--space-3) 0;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--accent);
    border-radius: var(--radius-sm);
    background: var(--accent-surface);
    color: var(--text);
    font-weight: var(--weight-bold);
  }

  .roster {
    display: grid;
    gap: var(--space-2);
    margin-bottom: var(--space-4);
    padding-bottom: var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }
  .roster h2,
  .chat h2 {
    margin-top: 0;
    font-size: var(--text-lg);
  }

  .member {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
    align-items: center;
    color: var(--text-muted);
  }
  .member-actions {
    display: flex;
    gap: var(--space-1);
  }
  .member-actions button {
    padding: var(--space-1) var(--space-2);
    font-size: var(--text-xs);
  }

  .pending-action {
    display: grid;
    gap: var(--space-2);
    margin: var(--space-3) 0;
    padding: var(--space-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    background: var(--surface-sunken);
  }
  .pending-action h3 {
    margin: 0;
  }

  .messages {
    min-height: 240px;
    max-height: 460px;
    overflow: auto;
  }
  .messages p {
    display: grid;
    gap: var(--space-1);
    padding: var(--space-2) 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  .messages span {
    color: var(--text-muted);
  }

  form {
    display: grid;
    gap: var(--space-2);
  }
  label {
    display: grid;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  input {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
  }

  .room-play button {
    padding: var(--space-2) var(--space-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-raised);
    color: var(--text);
    font: inherit;
    font-weight: var(--weight-medium);
    cursor: pointer;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }
  .room-play button:hover:not(:disabled) {
    border-color: var(--accent);
    background: var(--accent-surface);
  }
  .room-play button:disabled {
    opacity: .55;
    cursor: not-allowed;
  }

  .error {
    margin-bottom: var(--space-4);
    padding: var(--space-3);
    border: 1px solid var(--danger);
    border-left-width: 3px;
    border-radius: var(--radius-md);
    background: var(--danger-surface);
    color: var(--text);
  }

  @media (max-width: 900px) {
    .layout {
      grid-template-columns: 1fr;
    }
    .top {
      display: grid;
    }
  }
</style>
