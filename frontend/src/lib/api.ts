import type { CatalogGame, GameDefinition, GameSession, GameVersion, PlayerConfig, Principal, PublicUser, Room, RoomInvite, RoomMember, RoomMessage } from './types';

const BASE = '/api';

/**
 * An API failure that keeps the server's stable machine-readable code.
 *
 * The code is what makes errors translatable: `body.error` is English prose
 * written for developers, whereas `ACCOUNT_REQUIRED` can be looked up in any
 * locale. The English message is retained only as a last-resort fallback for a
 * code the interface does not know about yet.
 */
export class ApiError extends Error {
  readonly code: string;
  readonly details: string;
  readonly status: number;

  constructor(code: string, message: string, details: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.details = details;
    this.status = status;
  }
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const method = options?.method?.toUpperCase() ?? 'GET';
  const csrf = method === 'GET' ? '' : document.cookie.split('; ').find((value) => value.startsWith('rollboard_csrf='))?.split('=')[1] ?? '';
  let res: Response;
  try {
    res = await fetch(BASE + url, {
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}), ...options?.headers },
      ...options,
    });
  } catch {
    throw new ApiError('NETWORK', 'network request failed', '', 0);
  }
  if (!res.ok) {
    try {
      const body = await res.json();
      throw new ApiError(body.code || 'INTERNAL_ERROR', body.error || `HTTP ${res.status}`, body.details || '', res.status);
    } catch (cause) {
      if (cause instanceof ApiError) throw cause;
      throw new ApiError('INTERNAL_ERROR', res.statusText || `HTTP ${res.status}`, '', res.status);
    }
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

export const api = {
  health(): Promise<{ status: string }> {
    return request('/health');
  },

  listOwnedGames(): Promise<CatalogGame[]> { return request('/games'); },
  listPublishedVersions(): Promise<GameVersion[]> { return request('/games/versions'); },

  me(): Promise<Principal> { return request('/auth/me'); },
  enterGuest(displayName: string): Promise<Principal> { return request('/auth/guest', { method: 'POST', body: JSON.stringify({ displayName }) }); },
  register(email: string, displayName: string, password: string): Promise<PublicUser> { return request('/auth/register', { method: 'POST', body: JSON.stringify({ email, displayName, password }) }); },
  login(email: string, password: string): Promise<PublicUser> { return request('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }); },
  logout(): Promise<void> { return request('/auth/logout', { method: 'POST' }); },

  createDraft(g: GameDefinition): Promise<CatalogGame> { return request('/games', { method: 'POST', body: JSON.stringify(g) }); },
  getDraft(id: string): Promise<GameDefinition> { return request(`/games/${id}/draft`); },
  saveDraft(id: string, g: GameDefinition): Promise<GameDefinition> { return request(`/games/${id}/draft`, { method: 'PUT', body: JSON.stringify(g) }); },
  publishDraft(id: string): Promise<GameVersion> { return request(`/games/${id}/publish`, { method: 'POST' }); },
  getVersion(id: string, number: number): Promise<GameVersion> { return request(`/games/${id}/versions/${number}`); },

  createRoom(gameVersionId: string, title: string, maxPlayers: number): Promise<Room> {
    return request('/rooms', { method: 'POST', body: JSON.stringify({ gameVersionId, title, maxPlayers }) });
  },
  getRoom(id: string): Promise<Room> { return request(`/rooms/${id}`); },
  joinRoom(id: string): Promise<RoomMember> { return request(`/rooms/${id}/join`, { method: 'POST' }); },
  listRoomMessages(id: string): Promise<RoomMessage[]> { return request(`/rooms/${id}/messages`); },

  resolveInvite(token: string): Promise<RoomInvite> { return request(`/rooms/invite/${encodeURIComponent(token)}`); },
  joinByInvite(token: string): Promise<{ roomId: string }> { return request(`/rooms/invite/${encodeURIComponent(token)}/join`, { method: 'POST' }); },
  getRoomInvite(roomID: string): Promise<{ token: string }> { return request(`/rooms/${roomID}/invite`); },
  rotateRoomInvite(roomID: string): Promise<{ token: string }> { return request(`/rooms/${roomID}/invite`, { method: 'POST' }); },
  muteRoomMember(roomID: string, memberID: string, muted: boolean): Promise<void> { return request(`/rooms/${roomID}/members/${memberID}/mute`, { method: 'POST', body: JSON.stringify({ muted }) }); },
  removeRoomMember(roomID: string, memberID: string): Promise<void> { return request(`/rooms/${roomID}/members/${memberID}`, { method: 'DELETE' }); },

  validateGame(id: string): Promise<{ valid: boolean; errors?: string[] }> {
    return request(`/games/${id}/validate`, { method: 'POST' });
  },

  startPlaytest(gameId: string, players: PlayerConfig[], mode = 'hotseat'): Promise<GameSession> {
    return request(`/games/${gameId}/playtest`, {
      method: 'POST',
      body: JSON.stringify({ mode, players }),
    });
  },

  getSession(sessionId: string): Promise<GameSession> {
    return request(`/sessions/${sessionId}`);
  },

  rollDice(sessionId: string): Promise<GameSession> {
    return request(`/sessions/${sessionId}/roll`, { method: 'POST' });
  },

  performAction(sessionId: string, actionId: string): Promise<GameSession> {
    return request(`/sessions/${sessionId}/actions`, {
      method: 'POST',
      body: JSON.stringify({ actionId }),
    });
  },

  /** Multipart, so this one bypasses the JSON request helper. */
  async uploadImage(file: File): Promise<{ url: string; contentType: string }> {
    const csrf = document.cookie.split('; ').find((v) => v.startsWith('rollboard_csrf='))?.split('=')[1] ?? '';
    const body = new FormData();
    body.append('file', file);
    let res: Response;
    try {
      res = await fetch(`${BASE}/uploads`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {},
        body,
      });
    } catch {
      throw new ApiError('NETWORK', 'network request failed', '', 0);
    }
    if (!res.ok) {
      const problem = await res.json().catch(() => ({}));
      throw new ApiError(problem.code || 'INTERNAL_ERROR', problem.error || `HTTP ${res.status}`, problem.details || '', res.status);
    }
    return res.json();
  },

  proposeTrade(sessionId: string, offer: {
    toPlayerId: string;
    offerItems?: Record<string, number>;
    offerResources?: Record<string, number>;
    requestItems?: Record<string, number>;
    requestResources?: Record<string, number>;
  }): Promise<GameSession> {
    return request(`/sessions/${sessionId}/trade`, { method: 'POST', body: JSON.stringify(offer) });
  },

  manageInventory(sessionId: string, operation: 'equip' | 'unequip' | 'use', target: string): Promise<GameSession> {
    return request(`/sessions/${sessionId}/inventory`, {
      method: 'POST',
      body: JSON.stringify({ operation, target }),
    });
  },

  nextTurn(sessionId: string): Promise<GameSession> {
    return request(`/sessions/${sessionId}/next-turn`, { method: 'POST' });
  },
};
