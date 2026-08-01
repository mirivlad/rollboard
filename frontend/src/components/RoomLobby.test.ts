import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import RoomLobby from './RoomLobby.svelte';

describe('RoomLobby', () => {
  it('lets an account holder create a room from a published version', () => {
    render(RoomLobby, {
      versions: [{ id: 'version-id', gameId: 'game-id', versionNumber: 1, definition: {} as any, publishedAt: '' }],
      onCreate: () => {},
      onJoin: () => {},
    });

    expect(screen.getByLabelText(/published version/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /create room/i })).toBeTruthy();
    expect(screen.getByRole('button', { name: /join room/i })).toBeTruthy();
  });
});
