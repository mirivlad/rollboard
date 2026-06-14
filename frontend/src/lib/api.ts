import type { GameDefinition, GameSession, GameSummary, PlayerConfig } from './types';

const BASE = '/api';

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    headers: { 'Content-Type': 'application/json' },
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
