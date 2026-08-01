import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import RoomPlay from './RoomPlay.svelte';

describe('RoomPlay', () => {
  it('shows room controls and a room-only chat composer', () => {
    render(RoomPlay, { room: { id: 'room-id', title: 'Friday', status: 'lobby', maxPlayers: 4, members: [], sequence: 0 } as any, canStart: true });
    expect(screen.getByRole('button', { name: /start game/i })).toBeTruthy();
    expect(screen.getByLabelText(/message/i)).toBeTruthy();
  });
});
