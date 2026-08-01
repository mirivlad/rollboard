import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import AuthPanel from './AuthPanel.svelte';

describe('AuthPanel', () => {
  it('offers guest entry before account registration', () => {
    render(AuthPanel, { onGuest: () => {} });

    expect(screen.getByRole('button', { name: /continue as guest/i })).toBeTruthy();
    expect(screen.getByLabelText(/display name/i)).toBeTruthy();
  });
});
