import { afterEach, describe, expect, it, vi } from 'vitest';

import { api } from './api';

describe('api', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('accepts a successful no-content moderation response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    await expect(api.muteRoomMember('room-id', 'member-id', true)).resolves.toBeUndefined();
  });
});
