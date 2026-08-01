<script lang="ts">
  import type { GameVersion } from '../lib/types';

  type Props = {
    versions: GameVersion[];
    onCreate: (versionId: string, title: string, maxPlayers: number) => void | Promise<void>;
    onJoin: (roomId: string) => void | Promise<void>;
    busy?: boolean;
  };
  let { versions, onCreate, onJoin, busy = false }: Props = $props();
  let selectedVersion = $state('');
  let title = $state('');
  let maxPlayers = $state(4);
  let roomId = $state('');

  $effect(() => { if (!selectedVersion && versions[0]) selectedVersion = versions[0].id; });
  function create() { if (selectedVersion) void onCreate(selectedVersion, title, maxPlayers); }
  function join() { if (roomId.trim()) void onJoin(roomId.trim()); }
</script>

<section class="lobby">
  <div><p class="eyebrow">MULTIPLAYER</p><h1>Rooms</h1><p>Rooms are pinned to a published game version. Guests can join with a room ID.</p></div>
  <div class="cards">
    <form onsubmit={(event) => { event.preventDefault(); create(); }}>
      <h2>Create a room</h2>
      <label>Published version
        <select bind:value={selectedVersion} disabled={busy || versions.length === 0}>
          {#each versions as version}<option value={version.id}>{version.definition.title} · v{version.versionNumber}</option>{/each}
        </select>
      </label>
      <label>Room title<input bind:value={title} maxlength="120" placeholder="Friday game" /></label>
      <label>Players<input type="number" bind:value={maxPlayers} min="2" max="8" /></label>
      <button disabled={busy || !selectedVersion}>Create room</button>
      {#if versions.length === 0}<p class="hint">Publish a game version first.</p>{/if}
    </form>
    <form onsubmit={(event) => { event.preventDefault(); join(); }}>
      <h2>Join a room</h2>
      <label>Room ID<input bind:value={roomId} placeholder="Paste room ID" /></label>
      <button disabled={busy || !roomId.trim()}>Join room</button>
    </form>
  </div>
</section>

<style>
  .lobby{width:min(900px,100%);margin:0 auto}.eyebrow{color:#75d4ff;letter-spacing:.12em;font-size:.72rem;font-weight:800}.lobby>div>p:not(.eyebrow){color:#aab6d3}.cards{display:grid;grid-template-columns:1fr 1fr;gap:1rem;margin-top:2rem}.cards form{display:grid;gap:.8rem;padding:1.25rem;border:1px solid #30415f;border-radius:14px;background:#101a30}.cards label{display:grid;gap:.35rem;color:#cbd7ef}.cards input,.cards select{padding:.6rem;border:1px solid #385071;border-radius:8px;background:#0b1224;color:#f1f6ff;font:inherit}.hint{color:#aab6d3;margin:0}@media(max-width:720px){.cards{grid-template-columns:1fr}}
</style>
