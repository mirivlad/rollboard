<script lang="ts">
  import type { CatalogGame, GameDefinition, GameSession, GameVersion, Principal, PublicUser, Room, RoomInvite } from './lib/types';
  import { ApiError, api } from './lib/api';
  import { errorMessage, i18n } from './lib/i18n.svelte';
  import { createDefaultGame, createDungeonRaceDemo, createMiniMonopolyDemo } from './lib/defaults';
  import { createMonopolyDemo } from './lib/monopoly';
  import { createRpgDemo } from './lib/rpg';
  import AuthPanel from './components/AuthPanel.svelte';
  import InviteCard from './components/InviteCard.svelte';
  import LanguagePicker from './components/LanguagePicker.svelte';
  import ThemeToggle from './components/ThemeToggle.svelte';
  import BoardEditor from './components/BoardEditor.svelte';
  import GameDashboard, { type Template } from './components/GameDashboard.svelte';
  import GameSetup from './components/GameSetup.svelte';
  import PlaytestPanel from './components/PlaytestPanel.svelte';
  import RoomLobby from './components/RoomLobby.svelte';
  import RoomPlay from './components/RoomPlay.svelte';

  let view = $state<'loading' | 'auth' | 'invite' | 'dashboard' | 'setup' | 'editor' | 'playtest' | 'rooms' | 'room'>('loading');
  let pendingInvite = $state<RoomInvite | null>(null);
  let inviteToken = $state('');
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

  let t = $derived(i18n.t);

  function showError(error: unknown) { message = errorMessage(t, error); }
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
    // Language first, so a failure below is reported in the player's language
    // rather than in English.
    await i18n.init();

    inviteToken = new URLSearchParams(location.search).get('invite') ?? '';

    let signedIn = false;
    try {
      await api.health();
      if (document.cookie.includes('rollboard_csrf=')) {
        principal = await api.me();
        await refreshCatalog();
        signedIn = true;
      }
    } catch {
      signedIn = false;
    }

    if (inviteToken) {
      try {
        pendingInvite = await api.resolveInvite(inviteToken);
        view = 'invite';
        return;
      } catch {
        // A stale or withdrawn link should not trap somebody on an error
        // screen; drop it and carry on to the normal entry point.
        clearInviteFromURL();
      }
    }
    view = signedIn ? 'dashboard' : 'auth';
  }

  /** Take the token out of the address bar so a reload or share does not
      re-trigger the invite, and so it stays out of the browser history. */
  function clearInviteFromURL() {
    inviteToken = '';
    pendingInvite = null;
    const url = new URL(location.href);
    url.searchParams.delete('invite');
    history.replaceState(null, '', url.pathname + url.search);
  }

  async function acceptInvite(displayName: string) {
    busy = true; message = '';
    try {
      if (!principal) {
        principal = await api.enterGuest(displayName);
      }
      const { roomId } = await api.joinByInvite(inviteToken);
      currentRoom = await api.getRoom(roomId);
      clearInviteFromURL();
      view = 'room';
    } catch (error) { showError(error); } finally { busy = false; }
  }

  function dismissInvite() {
    const wasSignedIn = principal !== null;
    clearInviteFromURL();
    view = wasSignedIn ? 'dashboard' : 'auth';
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
    if (principal?.kind !== 'user') { message = t('dashboard.accountRequired'); return; }
    busy = true; message = '';
    const definition = template === 'mini-monopoly' ? createMiniMonopolyDemo()
      : template === 'monopoly' ? createMonopolyDemo()
      : template === 'rpg' ? createRpgDemo()
      : template === 'dungeon-race' ? createDungeonRaceDemo()
      : createDefaultGame();
    try { const game = await api.createDraft(definition); currentGame = await api.getDraft(game.id); view = advanced ? 'editor' : 'setup'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function finishSetup(game: GameDefinition) {
    busy = true; message = '';
    try { currentGame = await api.saveDraft(game.id, game); await refreshCatalog(); view = 'editor'; } catch (error) { showError(error); } finally { busy = false; }
  }
  async function saveGame() {
    if (!currentGame) return;
    busy = true; message = '';
    try { currentGame = await api.saveDraft(currentGame.id, currentGame); await refreshCatalog(); message = t('dashboard.draftSaved'); } catch (error) { showError(error); } finally { busy = false; }
  }
  async function publishGame() {
    if (!currentGame) return;
    busy = true; message = '';
    try { const version = await api.publishDraft(currentGame.id); publishedVersions = [version, ...publishedVersions.filter((item) => item.id !== version.id)]; await refreshCatalog(); message = t('dashboard.published', { version: version.versionNumber }); } catch (error) { showError(error); } finally { busy = false; }
  }
  async function openGame(gameID: string) { busy = true; message = ''; try { currentGame = await api.getDraft(gameID); view = 'editor'; } catch (error) { showError(error); } finally { busy = false; } }
  async function createRoom(versionId: string, title: string, maxPlayers: number) { busy = true; message = ''; try { currentRoom = await api.createRoom(versionId, title || t('lobby.newRoom'), maxPlayers); view = 'room'; } catch (error) { showError(error); } finally { busy = false; } }
  async function joinRoom(roomId: string) {
    busy = true; message = '';
    try {
      try {
        await api.joinRoom(roomId);
      } catch (error) {
        // The host is already a member of the room they created, and so is
        // anyone returning to a room they left open. Joining again is refused,
        // but they are still entitled to enter, so fall through to the read
        // and let its membership check decide.
        if (!(error instanceof ApiError) || error.code !== 'ROOM_CONFLICT') throw error;
      }
      currentRoom = await api.getRoom(roomId);
      view = 'room';
    } catch (error) { showError(error); } finally { busy = false; }
  }
  async function logout() { await api.logout(); principal = null; currentGame = null; currentSession = null; view = 'auth'; }

  $effect(() => { restoreSession(); });
</script>

<div class="app">
  {#if view !== 'auth' && view !== 'loading' && view !== 'invite'}
    <header>
      <strong class="brand">Rollboard</strong>
      <span class="who">{displayName()}</span>
      <div class="header-actions">
        <LanguagePicker />
        <ThemeToggle />
        <button class="btn" onclick={() => (view = 'rooms')}>{t('app.rooms')}</button>
        <button class="btn" onclick={logout}>{t('app.signOut')}</button>
      </div>
    </header>
  {/if}
  <main class:centred={view === 'auth' || view === 'loading' || view === 'invite'}>
    <!-- The auth panel renders the message inline next to the form, so the
         banner would be a duplicate there. -->
    {#if message && view !== 'auth' && view !== 'invite'}<p class="message" role="alert">{message}</p>{/if}
    {#if view === 'loading'}<p class="loading">{t('app.connecting')}</p>
    {:else if view === 'auth'}<AuthPanel onGuest={enterGuest} onRegister={register} onLogin={login} {busy} error={message} />
    {:else if view === 'invite' && pendingInvite}
      <InviteCard invite={pendingInvite} needsIdentity={principal === null} onJoin={acceptInvite} onDismiss={dismissInvite} {busy} error={message} />
    {:else if view === 'dashboard'}
      {#if principal?.kind === 'user'}<GameDashboard displayName={displayName()} onCreate={create} games={ownedGames} onOpen={openGame} {busy} />
      {:else}<RoomLobby versions={[]} onCreate={createRoom} onJoin={joinRoom} {busy} />{/if}
    {:else if view === 'setup' && currentGame}
      <GameSetup game={currentGame} onContinue={finishSetup} onAdvanced={finishSetup} />
    {:else if view === 'editor' && currentGame}
      <nav class="page-actions">
        <button class="btn" onclick={() => (view = 'dashboard')}>{t('editor.back')}</button>
        <button class="btn" onclick={saveGame} disabled={busy}>{t('editor.save')}</button>
        <button class="btn btn-primary" onclick={publishGame} disabled={busy}>{t('editor.publish')}</button>
        <button class="btn" onclick={() => (view = 'playtest')}>{t('editor.playtest')}</button>
      </nav>
      <BoardEditor game={currentGame} onsave={saveGame} />
    {:else if view === 'playtest' && currentGame}
      <PlaytestPanel {currentGame} session={currentSession} onSessionCreated={(session) => (currentSession = session)} onBack={() => (view = 'editor')} />
    {:else if view === 'rooms'}
      <RoomLobby versions={publishedVersions} onCreate={createRoom} onJoin={joinRoom} {busy} />
    {:else if view === 'room' && currentRoom}
      <nav class="page-actions">
        <button class="btn" onclick={() => (view = 'rooms')}>{t('room.back')}</button>
        <code class="room-id">{currentRoom.id}</code>
      </nav>
      <RoomPlay room={currentRoom} canStart={principal?.kind === 'user' && principal.user.id === currentRoom.hostUserId} canModerate={principal?.kind === 'user' && principal.user.id === currentRoom.hostUserId} actor={roomActor} onRoom={(room) => (currentRoom = room)} />
    {/if}
  </main>
</div>

<style>
  .app {
    min-height: 100vh;
    background: var(--bg-gradient);
    background-attachment: fixed;
    color: var(--text);
  }

  header {
    display: flex;
    gap: var(--space-4);
    align-items: center;
    min-height: var(--header-height);
    padding: var(--space-3) var(--space-5);
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-overlay);
    backdrop-filter: blur(12px);
    position: sticky;
    top: 0;
    z-index: 20;
    flex-wrap: wrap;
  }

  .brand {
    color: var(--accent-strong);
    font-size: var(--text-lg);
    font-weight: var(--weight-black);
    letter-spacing: -0.01em;
  }

  .who {
    margin-left: auto;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }

  .header-actions {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }

  main {
    padding: clamp(var(--space-4), 4vw, var(--space-7));
  }

  /* The sign-in card used to sit against the left edge because main only had
     padding. Centre it instead. */
  main.centred {
    display: grid;
    place-items: center;
    min-height: 100vh;
  }

  .loading {
    padding: var(--space-7);
    color: var(--text-muted);
    text-align: center;
  }

  .message {
    width: min(var(--page-wide), 100%);
    margin: 0 auto var(--space-4);
    padding: var(--space-3) var(--space-4);
    border: 1px solid var(--danger);
    border-left-width: 3px;
    border-radius: var(--radius-md);
    background: var(--danger-surface);
    color: var(--text);
  }

  .page-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
    width: min(var(--page-full), 100%);
    margin: 0 auto var(--space-4);
  }

  .room-id {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    overflow-x: auto;
  }

  /* Shared button, so every screen gets the same control instead of each
     component inventing its own. */
  :global(.btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: var(--surface-raised);
    color: var(--text);
    font: inherit;
    font-weight: var(--weight-medium);
    cursor: pointer;
    white-space: nowrap;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }
  :global(.btn:hover:not(:disabled)) {
    border-color: var(--accent);
    background: var(--accent-surface);
  }
  :global(.btn:disabled) {
    opacity: 0.55;
    cursor: not-allowed;
  }
  :global(.btn-primary) {
    border-color: var(--accent);
    background: var(--accent);
    color: var(--accent-contrast);
    font-weight: var(--weight-bold);
  }
  :global(.btn-primary:hover:not(:disabled)) {
    background: var(--accent-strong);
    border-color: var(--accent-strong);
  }

  @media (max-width: 640px) {
    header {
      gap: var(--space-2);
      padding: var(--space-3) var(--space-4);
    }
    .who {
      width: 100%;
      margin-left: 0;
      order: 3;
    }
    .header-actions {
      margin-left: auto;
    }
  }
</style>
