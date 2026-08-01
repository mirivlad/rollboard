import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import RoomPlay from './RoomPlay.svelte';

describe('RoomPlay', () => {
  it('shows room controls and a room-only chat composer', () => {
    render(RoomPlay, { room: { id: 'room-id', title: 'Friday', status: 'lobby', maxPlayers: 4, members: [], sequence: 0 } as any, canStart: true, actor: { kind: 'user', id: 'host-id' } });
    expect(screen.getByRole('button', { name: /start game/i })).toBeTruthy();
    expect(screen.getByLabelText(/message/i)).toBeTruthy();
  });

  it('offers the current player the server-provided pending action options', () => {
    render(RoomPlay, {
      room: {
        id: 'room-id', title: 'Friday', status: 'active', maxPlayers: 2, sequence: 2,
        members: [{ id: 'host-member', actorKind: 'user', actorId: 'host-id', playerId: 'player_1', displayName: 'Host' }],
        session: {
          id: 'session-id', definition: { board: { width: 100, height: 100, cellSize: 100, cells: [], edges: [] } },
          state: {
            status: 'active', turnNumber: 1, currentPlayerIndex: 0,
            players: [{ id: 'player_1', name: 'Host', color: '#fff', positionCellId: '', resources: {}, bankrupt: false }],
            cellStates: {}, pendingAction: { playerId: 'player_1', title: 'Choose a route', options: [{ id: 'left', title: 'Left path' }] },
          },
        },
      } as any,
      canStart: false,
      actor: { kind: 'user', id: 'host-id' },
    });

    expect(screen.getByRole('button', { name: /left path/i })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /roll dice/i })).toBeNull();
  });
});
