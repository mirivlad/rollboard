import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';

import GameSetup from './GameSetup.svelte';

describe('GameSetup', () => {
  it('passes the edited basics to the guided flow without changing the template board', async () => {
    const onContinue = vi.fn();
    render(GameSetup, {
      game: {
        id: 'game-id', title: 'Mini-Monopoly Demo', version: 1,
        board: { width: 96, height: 96, cellSize: 96, cells: [{ id: 'start' }], edges: [] },
        rules: { dice: { count: 1, sides: 6 }, resources: {}, cellTypes: {}, startBonus: 0 },
      } as any,
      onContinue,
      onAdvanced: () => {},
    });

    await fireEvent.input(screen.getByLabelText('Game title'), { target: { value: 'Friday race' } });
    await fireEvent.input(screen.getByLabelText('Dice sides'), { target: { value: '8' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Continue to board' }));

    expect(onContinue).toHaveBeenCalledWith(expect.objectContaining({
      title: 'Friday race',
      board: expect.objectContaining({ cells: [{ id: 'start' }] }),
      rules: expect.objectContaining({ dice: { count: 1, sides: 8 } }),
    }));
  });
});
