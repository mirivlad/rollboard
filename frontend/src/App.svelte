<script lang="ts">
  import type { CatalogGame, GameDefinition, GameSession, GameVersion, Principal, PublicUser, Room } from './lib/types';
  import { api } from './lib/api';
  import { createDefaultGame, createDungeonRaceDemo, createMiniMonopolyDemo } from './lib/defaults';
  import AuthPanel from './components/AuthPanel.svelte';
  import BoardEditor from './components/BoardEditor.svelte';
  import GameDashboard, { type Template } from './components/GameDashboard.svelte';
  import GameSetup from './components/GameSetup.svelte';
  import PlaytestPanel from './components/PlaytestPanel.svelte';
  import RoomLobby from './components/RoomLobby.svelte';
  import RoomPlay from './components/RoomPlay.svelte';

  let view = $state<'loading' | 'auth' | 'dashboard' | 'setup' | 'editor' | 'playtest' | 'rooms' | 'room'>('loading');
  let principal = $state<Principal | null>(null);
  let currentGame = $state<GameDefinition | null>(null);
  let currentSession = $state<GameSession | null>(null);
  let message = $state('');
  let busy = $state(false);
  let publishedVersions = $state<GameVersion[]>([]);
  let ownedGames = $state<CatalogGame[]>([]);
  let currentRoom = $state<Room | null>(null);
  let roomActor = $derived(principal?.kind === 'user'
    ? { kind: 'user' as const, id: principal.user.id }
    : { kind: 'guest' as const, id: principal?.guest.id ?? '' });

  function showError(error: unknown) { message = error instanceof Error ? error.message : 'Something went wrong'; }
  function displayName() { return principal?.kind === 'user' ? principal.user.displayName : principal?.guest.displayName ?? ''; }

  async function refreshCatalog() {
    if (principal?.kind !== 'user') {
      ownedGames = [];
      publishedVersions = [];
      return;
    }
    const [games, versions] = await Promise.all([api.listOwnedGames(), api.listPublishedVersions()]);
    ownedGames = games;
    publishedVersions = versions;
  }

  async function restoreSession() {
    try {
      await api.health();
      if (!document.cookie.includes('rollboard_csrf=')) {
        view = 'auth';
        return;
      }
      principal = await api.me();
      await refreshCatalog();
      view = 'dashboard';
    } catch {
      view = 'auth';
    }
  }

  async function enterGuest(name: string) {
    busy = true; message = '';
    try { principal = await api.enterGuest(name); view = 'dashboard'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function register(email: string, name: string, password: string) {
    busy = true; message = '';
    try { const user = await api.register(email, name, password); principal = { kind: 'user', user }; await refreshCatalog(); view = 'dashboard'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function login(email: string, password: string) {
    busy = true; message = '';
    try { const user: PublicUser = await api.login(email, password); principal = { kind: 'user', user }; await refreshCatalog(); view = 'dashboard'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function create(template: Template, advanced: boolean) {
    if (principal?.kind !== 'user') { message = 'Create an account to save and publish games.'; return; }
    busy = true; message = '';
    const definition = template === 'mini-monopoly' ? createMiniMonopolyDemo() : template === 'dungeon-race' ? createDungeonRaceDemo() : createDefaultGame();
    try { const game = await api.createDraft(definition); currentGame = await api.getDraft(game.id); view = advanced ? 'editor' : 'setup'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function finishSetup(game: GameDefinition) {
    busy = true; message = '';
    try { currentGame = await api.saveDraft(game.id, game); await refreshCatalog(); view = 'editor'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function saveGame() {
    if (!currentGame) return;
    busy = true; message = '';
    try { currentGame = await api.saveDraft(currentGame.id, currentGame); await refreshCatalog(); message = 'Draft saved'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function publishGame() {
    if (!currentGame) return;
    busy = true; message = '';
    try { const version = await api.publishDraft(currentGame.id); publishedVersions = [version, ...publishedVersions.filter((item) => item.id !== version.id)]; await refreshCatalog(); message = `Published version ${version.versionNumber}. You can now create a multiplayer room.`; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function openGame(gameID: string) { busy = true; message = ''; try { currentGame = await api.getDraft(gameID); view = 'editor'; } catch (error) { showError(error); } finally { busy = false; } }
  async function createRoom(versionId: string, title: string, maxPlayers: number) { busy = true; message = ''; try { currentRoom = await api.createRoom(versionId, title || 'New room', maxPlayers); view = 'room'; } catch (error) { showError(error); } finally { busy = false; } }
  async function joinRoom(roomId: string) { busy = true; message = ''; try { await api.joinRoom(roomId); currentRoom = await api.getRoom(roomId); view = 'room'; } catch (error) { showError(error); } finally { busy = false; } }
  async function logout() { await api.logout(); principal = null; currentGame = null; currentSession = null; view = 'auth'; }

  $effect(() => { restoreSession(); });
</script>

<div class="app">
  {#if view !== 'auth' && view !== 'loading'}
    <header><strong>Rollboard</strong><span>{displayName()}</span><button onclick={() => (view = 'rooms')}>Rooms</button><button onclick={logout}>Sign out</button></header>
  {/if}
  <main>
    {#if message}<p class="message" role="alert">{message}</p>{/if}
    {#if view === 'loading'}<p class="loading">Connecting to Rollboard…</p>
    {:else if view === 'auth'}<AuthPanel onGuest={enterGuest} onRegister={register} onLogin={login} {busy} error={message} />
    {:else if view === 'dashboard'}
      {#if principal?.kind === 'user'}<GameDashboard displayName={displayName()} onCreate={create} games={ownedGames} onOpen={openGame} {busy} />
      {:else}<RoomLobby versions={[]} onCreate={createRoom} onJoin={joinRoom} {busy} />{/if}
    {:else if view === 'setup' && currentGame}
      <GameSetup game={currentGame} onContinue={finishSetup} onAdvanced={finishSetup} />
    {:else if view === 'editor' && currentGame}
      <nav class="editor-actions"><button onclick={() => (view = 'dashboard')}>← Dashboard</button><button onclick={saveGame} disabled={busy}>Save draft</button><button onclick={publishGame} disabled={busy}>Publish</button><button onclick={() => (view = 'playtest')}>Playtest</button></nav>
      <BoardEditor game={currentGame} onsave={saveGame} />
    {:else if view === 'playtest' && currentGame}
      <PlaytestPanel {currentGame} session={currentSession} onSessionCreated={(session) => (currentSession = session)} onBack={() => (view = 'editor')} />
    {:else if view === 'rooms'}
      <RoomLobby versions={publishedVersions} onCreate={createRoom} onJoin={joinRoom} {busy} />
    {:else if view === 'room' && currentRoom}
      <nav class="editor-actions"><button onclick={() => (view = 'rooms')}>← Rooms</button><code>{currentRoom.id}</code></nav>
      <RoomPlay room={currentRoom} canStart={principal?.kind === 'user' && principal.user.id === currentRoom.hostUserId} canModerate={principal?.kind === 'user' && principal.user.id === currentRoom.hostUserId} actor={roomActor} onRoom={(room) => (currentRoom = room)} />
    {/if}
  </main>
</div>

<style>
  .app { min-height: 100vh; font-family: Inter, ui-sans-serif, system-ui, sans-serif; background: radial-gradient(circle at top left, #15345a, #090e1c 52%); color: #eef3ff; }
  header { display: flex; gap: 1rem; align-items: center; padding: .9rem 1.5rem; border-bottom: 1px solid #273858; background: #0b1224cc; } header strong { color: #74d4ff; } header span { margin-left: auto; color: #b5c2dd; } button { border: 1px solid #385071; border-radius: 8px; padding: .55rem .8rem; background: #14233d; color: #eff5ff; cursor: pointer; font: inherit; } main { padding: clamp(1rem, 4vw, 3rem); } .loading { color: #b9c8e5; text-align: center; padding: 4rem; } .message { width: min(1120px, 100%); margin: 0 auto 1rem; padding: .8rem; border: 1px solid #7c4561; border-radius: 8px; color: #ffd5df; background: #402034; } .editor-actions { display: flex; gap: .6rem; margin: 0 auto 1rem; width: min(1400px, 100%); }.editor-actions code{padding:.55rem;background:#080e1c;border-radius:8px;overflow:auto}
</style>
