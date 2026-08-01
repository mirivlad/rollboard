import { describe, expect, it } from 'vitest';

import { acceptsRoomSequence } from './room-sequence';

describe('acceptsRoomSequence', () => {
  it('accepts only a newer authoritative room state', () => {
    expect(acceptsRoomSequence(4, 5)).toBe(true);
    expect(acceptsRoomSequence(4, 4)).toBe(false);
    expect(acceptsRoomSequence(4, 3)).toBe(false);
    expect(acceptsRoomSequence(4, '5')).toBe(false);
  });
});
