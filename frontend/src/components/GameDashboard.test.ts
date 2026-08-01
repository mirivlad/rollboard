import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import GameDashboard from './GameDashboard.svelte';

describe('GameDashboard', () => {
  it('shows an empty author workspace with template choices', () => {
    render(GameDashboard, { displayName: 'Author', onCreate: () => {} });

    expect(screen.getByRole('heading', { name: /your games/i })).toBeTruthy();
    expect(screen.getByText(/no drafts yet/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /blank board/i })).toBeTruthy();
    expect(screen.getByRole('button', { name: /mini-monopoly/i })).toBeTruthy();
  });
});
