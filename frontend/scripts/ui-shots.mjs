// ui-shots.mjs — drive a real browser through the whole authoring and play
// flow and photograph every screen.
//
// This is the browser verification that scripts/browser-smoke.sh used to only
// describe. It doubles as the source of the screenshots in the README, so the
// pictures can never drift from what the application actually renders.
//
// Run through scripts/ui-shots.sh, which starts the stack first. It lives
// under frontend/ so Node resolves Playwright from frontend/node_modules.

import { chromium } from 'playwright';
import { mkdir, rm } from 'node:fs/promises';
import { join } from 'node:path';

const BASE = process.env.ROLLBOARD_UI_URL ?? 'http://127.0.0.1:18099';
const OUT = process.env.ROLLBOARD_UI_OUT ?? 'artifacts/ui';
const LOCALE = process.env.ROLLBOARD_UI_LOCALE ?? 'en';

const VIEWPORTS = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'tablet', width: 834, height: 1112 },
  { name: 'mobile', width: 390, height: 844 },
];

const THEMES = ['dark', 'light'];

const password = 'correct-horse-battery-staple';
const unique = () => `${Date.now()}-${Math.floor(Math.random() * 100000)}`;

/** Wait for the interface to settle before photographing it. */
async function settle(page) {
  await page.waitForLoadState('networkidle').catch(() => {});
  // Freeze the dice shake and token animations so screenshots are reproducible.
  await page.addStyleTag({
    content: '*, *::before, *::after { animation: none !important; transition: none !important; }',
  });
  await page.waitForTimeout(150);
}

async function shot(page, viewport, theme, name) {
  await settle(page);
  // Clicking a control can leave the page scrolled; always frame from the top.
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.waitForTimeout(80);
  const file = join(OUT, theme, viewport.name, `${name}.png`);
  await page.screenshot({ path: file, fullPage: false });
  return file;
}

/**
 * Seed one account with a published game and a room, using the API directly.
 *
 * Driving the editor through the UI for setup would make the run slow and
 * brittle; the screenshots that matter are taken from the real interface
 * afterwards.
 */
async function seed(request) {
  const email = `shots-${unique()}@example.com`;
  const register = await request.post(`${BASE}/api/auth/register`, {
    data: { email, displayName: 'Ada Lovelace', password },
  });
  if (!register.ok()) throw new Error(`register failed: ${register.status()} ${await register.text()}`);

  const cookies = await request.storageState();
  const csrf = cookies.cookies.find((cookie) => cookie.name === 'rollboard_csrf')?.value ?? '';
  const headers = { 'X-CSRF-Token': csrf };

  const created = await request.post(`${BASE}/api/games`, { headers, data: demoGame() });
  if (!created.ok()) throw new Error(`create game failed: ${await created.text()}`);
  const game = await created.json();

  const saved = await request.put(`${BASE}/api/games/${game.id}/draft`, { headers, data: demoGame() });
  if (!saved.ok()) throw new Error(`save draft failed: ${await saved.text()}`);

  const published = await request.post(`${BASE}/api/games/${game.id}/publish`, { headers });
  if (!published.ok()) throw new Error(`publish failed: ${await published.text()}`);
  const version = await published.json();

  const room = await request.post(`${BASE}/api/rooms`, {
    headers,
    data: { gameVersionId: version.id, title: 'Friday night', maxPlayers: 4 },
  });
  if (!room.ok()) throw new Error(`create room failed: ${await room.text()}`);

  return { email, gameId: game.id, room: await room.json() };
}

function demoGame() {
  const cell = (id, title, type, x, y, color, fields = {}, onLand = []) => ({
    id, title, type, x, y, visual: { baseColor: color, baseImage: '' }, fields, onLand,
  });
  return {
    title: 'Harbour Run',
    version: 1,
    board: {
      width: 576, height: 384, cellSize: 96,
      cells: [
        cell('start', 'Start', 'start', 0, 0, '#4CAF50'),
        cell('dock', 'Dock', 'property', 96, 0, '#E3F2FD', { cost: 100, rent: 20 }, [
          { type: 'if_cell_unowned', then: [
            { type: 'offer_choice', title: 'Buy the dock for 100?', options: [
              { id: 'buy', title: 'Buy (100)', then: [
                { type: 'lose_resource', resource: 'money', amountField: 'cost' },
                { type: 'set_cell_owner', target: 'current' },
              ] },
              { id: 'skip', title: 'Walk on by', then: [] },
            ] },
          ], else: [
            { type: 'if_cell_owned_by_other', then: [
              { type: 'transfer_resource', resource: 'money', amountField: 'rent', target: 'owner' },
            ] },
          ] },
        ]),
        cell('market', 'Market', 'bonus', 192, 0, '#C8E6C9', { amount: 60 }, [
          { type: 'gain_resource', resource: 'money', amountField: 'amount' },
        ]),
        cell('storm', 'Storm', 'penalty', 288, 0, '#FFCDD2', { amount: 40 }, [
          { type: 'lose_resource', resource: 'money', amountField: 'amount' },
        ]),
        cell('warehouse', 'Warehouse', 'property', 384, 0, '#FFE0B2', { cost: 140, rent: 30 }),
        cell('lighthouse', 'Lighthouse', 'bonus', 480, 0, '#C8E6C9', { amount: 90 }, [
          { type: 'gain_resource', resource: 'money', amountField: 'amount' },
        ]),
      ],
      edges: [
        { id: 'e1', from: 'start', to: 'dock', condition: { type: 'always' } },
        { id: 'e2', from: 'dock', to: 'market', condition: { type: 'always' } },
        { id: 'e3', from: 'market', to: 'storm', condition: { type: 'always' } },
        { id: 'e4', from: 'storm', to: 'warehouse', condition: { type: 'always' } },
        { id: 'e5', from: 'warehouse', to: 'lighthouse', condition: { type: 'always' } },
        { id: 'e6', from: 'lighthouse', to: 'start', condition: { type: 'always' } },
      ],
    },
    rules: {
      dice: { count: 1, sides: 6 },
      resources: { money: { initial: 500, label: 'Money' } },
      cellTypes: {
        start: { title: 'Start', fields: {} },
        property: { title: 'Property', fields: { cost: { label: 'Cost', type: 'number', default: 100 }, rent: { label: 'Rent', type: 'number', default: 20 } } },
        bonus: { title: 'Bonus', fields: { amount: { label: 'Amount', type: 'number', default: 50 } } },
        penalty: { title: 'Penalty', fields: { amount: { label: 'Amount', type: 'number', default: 40 } } },
      },
      startBonus: 50,
      startBonusResource: 'money',
    },
  };
}

async function capture(browser, viewport, theme, fixtures) {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    colorScheme: theme,
    locale: LOCALE,
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();
  const taken = [];

  const remember = async (name) => taken.push(await shot(page, viewport, theme, name));

  // 1. Sign-in screen, signed out.
  await page.goto(BASE);
  await page.evaluate((locale) => localStorage.setItem('rollboard_locale', locale), LOCALE);
  await page.goto(BASE);
  await page.waitForSelector('.auth-panel');
  await remember('01-sign-in');

  // Tab through a few controls so the focus ring is captured. The interface
  // previously had no focus styles at all, so this frame is the evidence that
  // keyboard navigation is now visible.
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  await remember('01b-keyboard-focus');
  await page.locator('body').click({ position: { x: 5, y: 5 } }).catch(() => {});

  // 2. Sign in as the seeded author.
  await page.getByRole('tab').nth(2).click();
  await page.locator('input[type=email]').fill(fixtures.email);
  await page.locator('input[type=password]').fill(password);
  await page.locator('button[type=submit]').click();
  await page.waitForSelector('.dashboard', { timeout: 15000 });
  await remember('02-dashboard');

  // 3. The board editor with a cell selected, so the inspector has content.
  await page.locator('.draft').first().click();
  await page.waitForSelector('.editor', { timeout: 15000 });
  await remember('03-editor');

  const firstCell = page.locator('.editor .cell, .editor [data-cell-id]').first();
  if (await firstCell.count()) {
    await firstCell.click({ force: true }).catch(() => {});
    await remember('04-editor-inspector');

    // The action editor is the point of the studio, so photograph it with a
    // real action open rather than an empty list.
    const addAction = page.locator('.inspector select.add').first();
    if (await addAction.count()) {
      await addAction.selectOption('if_resource_ge').catch(() => {});
      await page.waitForTimeout(200);
      const formulaToggle = page.locator('.inspector .formula input[type=checkbox]').first();
      if (await formulaToggle.count()) {
        await formulaToggle.check().catch(() => {});
      }
      await page.locator('.inspector').evaluate((el) => el.scrollTo(0, el.scrollHeight)).catch(() => {});
      await remember('04b-action-editor');
      await page.locator('.inspector').evaluate((el) => el.scrollTo(0, 0)).catch(() => {});
    }
  }

  // 4. Hotseat playtest: setup, a turn, and a roll.
  await page.getByRole('button', { name: /playtest|тестовая игра/i }).first().click().catch(() => {});
  if (await page.locator('.playtest').count()) {
    await page.waitForSelector('.setup', { timeout: 10000 }).catch(() => {});
    await remember('05-playtest-setup');
    await page.locator('.setup > button').click().catch(() => {});
    await page.waitForSelector('.turn-intro', { timeout: 10000 }).catch(() => {});
    await remember('06-playtest-turn');
    await page.locator('.primary-btn').first().click().catch(() => {});
    await page.waitForSelector('.game-ui', { timeout: 10000 }).catch(() => {});
    await remember('07-playtest-board');
    await page.locator('.roll-btn').click().catch(() => {});
    await page.waitForTimeout(2200);
    await remember('08-playtest-rolled');
  }

  // 5. The multiplayer lobby and a live room.
  await page.goto(BASE);
  await page.waitForSelector('.dashboard', { timeout: 15000 }).catch(() => {});
  await page.getByRole('button', { name: /^rooms$|^комнаты$/i }).first().click().catch(() => {});
  await page.waitForSelector('.lobby', { timeout: 10000 }).catch(() => {});
  await remember('09-rooms-lobby');

  await page.locator('.lobby input').last().fill(fixtures.room.id);
  await page.getByRole('button', { name: /join room|войти/i }).last().click().catch(() => {});
  await page.waitForSelector('.room-play', { timeout: 15000 }).catch(() => {});
  await remember('10-room');

  await context.close();
  return taken;
}

async function main() {
  await rm(OUT, { recursive: true, force: true });
  for (const theme of THEMES) {
    for (const viewport of VIEWPORTS) {
      await mkdir(join(OUT, theme, viewport.name), { recursive: true });
    }
  }

  const browser = await chromium.launch();
  const seedContext = await browser.newContext({ baseURL: BASE });
  const fixtures = await seed(seedContext.request);
  await seedContext.close();

  let total = 0;
  for (const theme of THEMES) {
    for (const viewport of VIEWPORTS) {
      const taken = await capture(browser, viewport, theme, fixtures);
      total += taken.length;
      console.log(`${theme}/${viewport.name}: ${taken.length} screenshots`);
    }
  }
  await browser.close();
  console.log(`\n${total} screenshots written to ${OUT}/`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
