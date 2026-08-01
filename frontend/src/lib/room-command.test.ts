import { describe, expect, it } from 'vitest';

import { roomCommand } from './room-command';

describe('roomCommand', () => {
  it('adds one UUID command ID without adding a game result', () => {
    const command = roomCommand('roll');

    expect(command).toMatchObject({ type: 'roll', commandId: expect.any(String) });
    expect(command.commandId).toMatch(/^[0-9a-f-]{36}$/i);
    expect(command).not.toHaveProperty('dice');
    expect(command).not.toHaveProperty('total');
  });
});
