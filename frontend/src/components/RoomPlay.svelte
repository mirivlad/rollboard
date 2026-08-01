<script lang="ts">
  import { onMount } from 'svelte';
  import type { Room, RoomMessage } from '../lib/types';
  import { api } from '../lib/api';
  import BoardView from './BoardView.svelte';

  type Props = { room: Room; canStart: boolean; onRoom?: (room: Room) => void };
  let { room, canStart, onRoom = () => {} }: Props = $props();
  let currentRoom = $state<Room>({ id: '', gameVersionId: '', hostUserId: '', hostMemberId: '', title: '', status: 'lobby', maxPlayers: 2, members: [], sequence: 0 });
  let messages = $state<RoomMessage[]>([]);
  let message = $state('');
  let error = $state('');
  let socket = $state<WebSocket | null>(null);
  let connected = $state(false);

  $effect(() => { currentRoom = room; });
  function send(value: object) { if (!socket || socket.readyState !== WebSocket.OPEN) { error = 'Realtime connection is unavailable.'; return; } socket.send(JSON.stringify(value)); }
  function submitMessage() { const body = message.trim(); if (!body) return; send({ type: 'chat', body }); message = ''; }

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
    <div class="game"><h2>{currentRoom.session ? `Turn ${currentRoom.session.state.turnNumber}` : 'Waiting in lobby'}</h2>{#if currentRoom.session}<p>Current player: {currentRoom.session.state.players[currentRoom.session.state.currentPlayerIndex]?.name}</p><button onclick={() => send({ type: 'roll' })} disabled={currentRoom.session.state.status !== 'active'}>Roll dice</button><BoardView board={currentRoom.session.definition.board} players={currentRoom.session.state.players} cellStates={currentRoom.session.state.cellStates} />{/if}</div>
    <aside><h2>Room chat</h2><div class="messages">{#each messages as item (item.id)}<p><strong>{item.displayName}</strong><span>{item.body}</span></p>{/each}</div><form onsubmit={(event) => { event.preventDefault(); submitMessage(); }}><label>Message<input bind:value={message} maxlength="1000" placeholder="Say hello" /></label><button disabled={!message.trim()}>Send</button></form></aside>
  </div>
</section>

<style>
  .room-play{width:min(1400px,100%);margin:0 auto}.top{display:flex;justify-content:space-between;gap:1rem;align-items:start}.eyebrow{color:#75d4ff;letter-spacing:.12em;font-size:.72rem;font-weight:800}.top p{color:#aab6d3}.layout{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:1rem}.game,aside{border:1px solid #30415f;border-radius:14px;background:#101a30;padding:1rem}.messages{min-height:260px;max-height:460px;overflow:auto}.messages p{display:grid;gap:.2rem;padding:.55rem 0;border-bottom:1px solid #243652}.messages span{color:#cbd7ef}form{display:grid;gap:.6rem}label{display:grid;gap:.3rem}input{padding:.6rem;border:1px solid #385071;border-radius:8px;background:#0b1224;color:#f1f6ff;font:inherit}.error{padding:.7rem;background:#402034;color:#ffd5df;border-radius:8px}@media(max-width:900px){.layout{grid-template-columns:1fr}.top{display:grid}}
</style>
