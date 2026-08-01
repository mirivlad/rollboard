<script lang="ts">
  import { onMount } from 'svelte';
  import type { Room, RoomMessage } from '../lib/types';
  import { api } from '../lib/api';
  import BoardView from './BoardView.svelte';

  type Actor = { kind: 'user' | 'guest'; id: string };
  type Props = { room: Room; canStart: boolean; canModerate: boolean; actor: Actor; onRoom?: (room: Room) => void };
  let { room, canStart, canModerate, actor, onRoom = () => {} }: Props = $props();
  let currentRoom = $state<Room>({ id: '', gameVersionId: '', hostUserId: '', hostMemberId: '', title: '', status: 'lobby', maxPlayers: 2, members: [], sequence: 0 });
  let messages = $state<RoomMessage[]>([]);
  let message = $state('');
  let error = $state('');
  let socket = $state<WebSocket | null>(null);
  let connected = $state(false);

  $effect(() => { currentRoom = room; });
  let ownMember = $derived(currentRoom.members.find((member) => member.actorKind === actor.kind && member.actorId === actor.id));
  let currentPlayer = $derived(currentRoom.session?.state.players[currentRoom.session.state.currentPlayerIndex]);
  let pendingAction = $derived(currentRoom.session?.state.pendingAction);
  let canRoll = $derived(Boolean(
    currentRoom.session?.state.status === 'active' &&
    ownMember?.playerId &&
    ownMember.playerId === currentPlayer?.id &&
    !pendingAction
  ));
  let canResolveAction = $derived(Boolean(
    pendingAction && ownMember?.playerId === pendingAction.playerId
  ));
  function send(value: object) { if (!socket || socket.readyState !== WebSocket.OPEN) { error = 'Realtime connection is unavailable.'; return; } socket.send(JSON.stringify(value)); }
  function submitMessage() { const body = message.trim(); if (!body) return; send({ type: 'chat', body }); message = ''; }
  async function refreshRoom() { currentRoom = await api.getRoom(currentRoom.id); onRoom(currentRoom); }
  async function muteMember(memberID: string, muted: boolean) { error = ''; try { await api.muteRoomMember(currentRoom.id, memberID, muted); await refreshRoom(); } catch (cause) { error = cause instanceof Error ? `Moderation failed: ${cause.message}` : 'Moderation failed.'; } }
  async function removeMember(memberID: string) { error = ''; try { await api.removeRoomMember(currentRoom.id, memberID); await refreshRoom(); } catch (cause) { error = cause instanceof Error ? `Moderation failed: ${cause.message}` : 'Moderation failed.'; } }

  onMount(() => {
    void api.listRoomMessages(room.id).then((value) => messages = value).catch(() => {});
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${location.host}/api/rooms/${encodeURIComponent(room.id)}/ws?since=${room.sequence}`);
    socket = ws;
    ws.onopen = () => connected = true;
    ws.onclose = () => connected = false;
    ws.onerror = () => error = 'Realtime connection was interrupted.';
    ws.onmessage = (event) => {
      const envelope = JSON.parse(event.data);
      if (envelope.type === 'room_state') { currentRoom = envelope.payload; onRoom(currentRoom); }
      if (envelope.type === 'room_event' && envelope.payload?.session) { currentRoom = { ...currentRoom, sequence: envelope.sequence, status: envelope.payload.session.state.status, session: envelope.payload.session }; onRoom(currentRoom); }
      if (envelope.type === 'chat_message') messages = [...messages, envelope.payload];
      if (envelope.type === 'room_error') error = envelope.details;
    };
    return () => ws.close();
  });
</script>

<section class="room-play">
  <div class="top"><div><p class="eyebrow">{connected ? 'LIVE ROOM' : 'RECONNECTING'}</p><h1>{currentRoom.title}</h1><p>{currentRoom.members.length}/{currentRoom.maxPlayers} players · {currentRoom.status}</p></div>{#if currentRoom.status === 'lobby' && canStart}<button onclick={() => send({ type: 'start' })}>Start game</button>{/if}</div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="layout">
    <div class="game"><h2>{currentRoom.session ? `Turn ${currentRoom.session.state.turnNumber}` : 'Waiting in lobby'}</h2>{#if currentRoom.session}<p>Current player: {currentPlayer?.name}</p>{#if pendingAction}<section class="pending-action"><h3>{pendingAction.title || 'Choose an action'}</h3>{#if canResolveAction}{#each pendingAction.options ?? [] as option (option.id)}<button onclick={() => send({ type: 'action', actionId: option.id })}>{option.title}</button>{/each}{:else}<p>Waiting for the current player to choose.</p>{/if}</section>{:else if canRoll}<button onclick={() => send({ type: 'roll' })}>Roll dice</button>{:else}<p>Waiting for the current player to roll.</p>{/if}<BoardView board={currentRoom.session.definition.board} players={currentRoom.session.state.players} cellStates={currentRoom.session.state.cellStates} />{/if}</div>
    <aside><section class="roster"><h2>Players</h2>{#each currentRoom.members as member (member.id)}<div class="member"><span>{member.displayName}{member.mutedAt ? ' · muted' : ''}</span>{#if canModerate && member.id !== currentRoom.hostMemberId}<div class="member-actions"><button aria-label={`${member.mutedAt ? 'Unmute' : 'Mute'} ${member.displayName}`} onclick={() => muteMember(member.id, !member.mutedAt)}>{member.mutedAt ? 'Unmute' : 'Mute'}</button><button aria-label={`Remove ${member.displayName}`} onclick={() => removeMember(member.id)}>Remove</button></div>{/if}</div>{/each}</section><section class="chat"><h2>Room chat</h2><div class="messages">{#each messages as item (item.id)}<p><strong>{item.displayName}</strong><span>{item.body}</span></p>{/each}</div><form onsubmit={(event) => { event.preventDefault(); submitMessage(); }}><label>Message<input bind:value={message} maxlength="1000" placeholder="Say hello" /></label><button disabled={!message.trim()}>Send</button></form></section></aside>
  </div>
</section>

<style>
  .room-play{width:min(1400px,100%);margin:0 auto}.top{display:flex;justify-content:space-between;gap:1rem;align-items:start}.eyebrow{color:#75d4ff;letter-spacing:.12em;font-size:.72rem;font-weight:800}.top p{color:#aab6d3}.layout{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:1rem}.game,aside{border:1px solid #30415f;border-radius:14px;background:#101a30;padding:1rem}.roster{display:grid;gap:.45rem;margin-bottom:1rem;padding-bottom:1rem;border-bottom:1px solid #243652}.roster h2,.chat h2{margin-top:0}.member{display:flex;justify-content:space-between;gap:.5rem;align-items:center;color:#cbd7ef}.member-actions{display:flex;gap:.35rem}.member-actions button{padding:.35rem .5rem;font-size:.8rem}.pending-action{display:grid;gap:.6rem;margin:.8rem 0;padding:.8rem;border:1px solid #385071;border-radius:10px;background:#0b1224}.pending-action h3{margin:0}.messages{min-height:260px;max-height:460px;overflow:auto}.messages p{display:grid;gap:.2rem;padding:.55rem 0;border-bottom:1px solid #243652}.messages span{color:#cbd7ef}form{display:grid;gap:.6rem}label{display:grid;gap:.3rem}input{padding:.6rem;border:1px solid #385071;border-radius:8px;background:#0b1224;color:#f1f6ff;font:inherit}.room-play button{border:1px solid #385071;border-radius:8px;padding:.55rem .8rem;background:#14233d;color:#eff5ff;cursor:pointer;font:inherit}.room-play button:disabled{opacity:.55;cursor:not-allowed}.error{padding:.7rem;background:#402034;color:#ffd5df;border-radius:8px}@media(max-width:900px){.layout{grid-template-columns:1fr}.top{display:grid}}
</style>
