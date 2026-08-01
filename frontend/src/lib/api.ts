import type { CatalogGame, GameDefinition, GameSession, GameSummary, GameVersion, PlayerConfig, Principal, PublicUser } from './types';

const BASE = '/api';

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const method = options?.method?.toUpperCase() ?? 'GET';
  const csrf = method === 'GET' ? '' : document.cookie.split('; ').find((value) => value.startsWith('rollboard_csrf='))?.split('=')[1] ?? '';
  const res = await fetch(BASE + url, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}), ...options?.headers },
    ...options,
  });
  if (!res.ok) {
    let errorMsg = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      errorMsg = body.error || errorMsg;
    } catch {
      const text = await res.text().catch(() => '');
      errorMsg = text || res.statusText;
    }
    throw new Error(errorMsg);
  }
  return res.json();
}

export const api = {
  health(): Promise<{ status: string }> {
    return request('/health');
  },

  listGames(): Promise<GameSummary[]> {
    return request('/games');
  },

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

  getGame(id: string): Promise<GameDefinition> {
    return request(`/games/${id}`);
  },

  createGame(g: GameDefinition): Promise<GameDefinition> {
    return request('/games', {
      method: 'POST',
      body: JSON.stringify(g),
    });
  },

  updateGame(id: string, g: GameDefinition): Promise<GameDefinition> {
    return request(`/games/${id}`, {
      method: 'PUT',
      body: JSON.stringify(g),
    });
  },

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

  nextTurn(sessionId: string): Promise<GameSession> {
    return request(`/sessions/${sessionId}/next-turn`, { method: 'POST' });
  },
};
