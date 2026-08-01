import { describe, expect, it } from 'vitest';

import { confirmedRollPlayback } from './room-events';

describe('confirmedRollPlayback', () => {
  it('uses only the authoritative dice and path event payloads', () => {
    const playback = confirmedRollPlayback([
      { id: 'dice', type: 'dice_roll', message: '', createdAt: '', payload: { rolls: [3, 4], total: 7, playerId: 'player_1' } },
      { id: 'move', type: 'move', message: '', createdAt: '', payload: { from: 'start', path: ['a', 'b'], playerId: 'player_1' } },
    ]);

    expect(playback).toEqual({ rolls: [3, 4], total: 7, playerId: 'player_1', positions: ['start', 'a', 'b'] });
  });

  it('does not animate malformed dice payloads', () => {
    const playback = confirmedRollPlayback([
      { id: 'dice', type: 'dice_roll', message: '', createdAt: '', payload: { rolls: [], total: 0, playerId: 'player_1' } },
      { id: 'move', type: 'move', message: '', createdAt: '', payload: { from: 'start', path: [], playerId: 'player_1' } },
    ]);

    expect(playback).toBeNull();
  });
});
