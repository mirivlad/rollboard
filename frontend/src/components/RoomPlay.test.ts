import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import RoomPlay from './RoomPlay.svelte';

describe('RoomPlay', () => {
  it('shows room controls and a room-only chat composer', () => {
    render(RoomPlay, { room: { id: 'room-id', title: 'Friday', status: 'lobby', maxPlayers: 4, members: [], sequence: 0 } as any, canStart: true, canModerate: false, actor: { kind: 'user', id: 'host-id' } });
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
      canModerate: false,
      actor: { kind: 'user', id: 'host-id' },
    });

    expect(screen.getByRole('button', { name: /left path/i })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /roll dice/i })).toBeNull();
  });

  it('gives the host mute and remove controls for other room members', () => {
    render(RoomPlay, {
      room: {
        id: 'room-id', title: 'Friday', status: 'lobby', maxPlayers: 2, sequence: 1,
        members: [
          { id: 'host-member', actorKind: 'user', actorId: 'host-id', displayName: 'Host' },
          { id: 'guest-member', actorKind: 'guest', actorId: 'guest-id', displayName: 'Guest' },
        ],
      } as any,
      canStart: true,
      canModerate: true,
      actor: { kind: 'user', id: 'host-id' },
    });

    expect(screen.getByRole('button', { name: 'Mute Guest' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Remove Guest' })).toBeTruthy();
  });
});

describe('RoomPlay inventory and trading', () => {
  const room = (overrides: Record<string, unknown> = {}) => ({
    id: 'room-id', title: 'Friday', status: 'active', maxPlayers: 2, sequence: 2,
    members: [
      { id: 'host-member', actorKind: 'user', actorId: 'host-id', playerId: 'player_1', displayName: 'Host' },
      { id: 'guest-member', actorKind: 'user', actorId: 'guest-id', playerId: 'player_2', displayName: 'Guest' },
    ],
    session: {
      id: 'session-id',
      definition: {
        board: { width: 100, height: 100, cellSize: 100, cells: [], edges: [] },
        rules: {
          resources: { money: { initial: 100 } },
          equipmentSlots: ['weapon'],
          items: { sword: { id: 'sword', title: 'Sword', slot: 'weapon' } },
        },
      },
      state: {
        status: 'active', turnNumber: 1, currentPlayerIndex: 0, cellStates: {},
        players: [
          { id: 'player_1', name: 'Host', color: '#fff', positionCellId: '', resources: { money: 100 }, bankrupt: false, inventory: { sword: 1 }, equipped: {} },
          { id: 'player_2', name: 'Guest', color: '#000', positionCellId: '', resources: { money: 100 }, bankrupt: false, inventory: {}, equipped: {} },
        ],
      },
    },
    ...overrides,
  });

  // Queries are scoped to each render: these tests share a file with others
  // that leave their own markup behind.
  it('lets the player whose turn it is equip what they carry and offer a trade', () => {
    // The room protocol carried no inventory command at all, so an online
    // player could be handed a sword by a cell and never put it on.
    const { container } = render(RoomPlay, { room: room() as any, canStart: false, canModerate: false, actor: { kind: 'user', id: 'host-id' } });
    expect(within(container).getByRole('button', { name: /equip/i })).toBeTruthy();
    expect(within(container).getByRole('button', { name: /propose a trade/i })).toBeTruthy();
  });

  it('shows another member their own pack, without the controls', () => {
    const { container } = render(RoomPlay, { room: room() as any, canStart: false, canModerate: false, actor: { kind: 'user', id: 'guest-id' } });
    // It is not their turn: the pack is visible, equipping is not offered, and
    // neither is a trade.
    expect(within(container).getByRole('heading', { name: /pack|рюкзак/i })).toBeTruthy();
    expect(within(container).queryByRole('button', { name: /equip/i })).toBeNull();
    expect(within(container).queryByRole('button', { name: /propose a trade/i })).toBeNull();
  });

  it('leaves the panels out of a game with no items', () => {
    const plain = room();
    (plain.session.definition.rules as Record<string, unknown>).items = {};
    const { container } = render(RoomPlay, { room: plain as any, canStart: false, canModerate: false, actor: { kind: 'user', id: 'host-id' } });
    expect(within(container).queryByRole('button', { name: /equip/i })).toBeNull();
  });
});
