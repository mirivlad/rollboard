import { describe, expect, it } from 'vitest';
import { optionLabel, pendingHeading } from './option-label';
import { translate } from './i18n.svelte';
import type { GameSession } from './types';
import en from '../../../locales/en.json';
import ru from '../../../locales/ru.json';

const t = (catalog: Record<string, string>, locale: string) =>
  (key: string, params?: Record<string, string | number>) => translate(catalog, locale, key, params);

const enT = t(en as Record<string, string>, 'en');
const ruT = t(ru as Record<string, string>, 'ru');

describe('optionLabel', () => {
  it('leaves an author\'s own wording alone', () => {
    expect(optionLabel({ id: 'buy', title: 'Buy the dock for 100' }, enT)).toBe('Buy the dock for 100');
  });

  it('translates the options the engine generates', () => {
    // The server writes these in English because it has no idea what language
    // the player is reading in.
    expect(optionLabel({ id: 'pass', title: 'Pass' }, ruT)).toBe('Пас');
    expect(optionLabel({ id: 'accept', title: 'Accept' }, ruT)).toBe('Принять');
    expect(optionLabel({ id: 'decline', title: 'Decline' }, ruT)).toBe('Отказаться');
  });

  it('rebuilds a bid from its id, with the auction currency', () => {
    const option = { id: 'bid_120', title: 'Bid 120 money' };
    expect(optionLabel(option, enT, { resource: 'money' })).toBe('Bid 120 money');
    expect(optionLabel(option, ruT, { resource: 'золото' })).toBe('Ставка 120 золото');
  });

  it('does not mistake an author option that merely starts with bid', () => {
    expect(optionLabel({ id: 'bidding_war', title: 'Start a bidding war' }, enT)).toBe('Start a bidding war');
  });
});

function auctionSession(highBidderId?: string): GameSession {
  return {
    id: 's1', gameId: 'g1', gameVersion: 1, mode: 'hotseat',
    definition: { id: 'g1', title: 'T', version: 1, board: { width: 0, height: 0, cellSize: 96, cells: [], edges: [] }, rules: { dice: { count: 1, sides: 6 }, resources: {}, cellTypes: {}, startBonus: 0 } },
    state: {
      currentPlayerIndex: 0,
      players: [
        { id: 'player_1', name: 'Ada', color: '#111', positionCellId: 'start', resources: {}, bankrupt: false },
        { id: 'player_2', name: 'Bob', color: '#222', positionCellId: 'start', resources: {}, bankrupt: false },
      ],
      cellStates: {}, turnNumber: 1, roundNumber: 1, status: 'active', log: [],
      pendingAction: { type: 'auction_bid', playerId: 'player_2', title: 'Auction: Blue A — Ada leads with 60 money' },
      pendingAuction: { resource: 'money', increment: 10, minBid: 50, highBid: 60, highBidderId, order: ['player_1', 'player_2'], index: 1 },
    },
  } as GameSession;
}

describe('pendingHeading', () => {
  it('rebuilds the standing bid in the reader\'s language', () => {
    expect(pendingHeading(auctionSession('player_1'), ruT, 'room.chooseAction')).toBe('Аукцион — Лидирует Ada: 60 money');
  });

  it('says so when nobody has bid yet', () => {
    expect(pendingHeading(auctionSession(undefined), enT, 'room.chooseAction')).toBe('Auction — No bids yet');
  });

  it('keeps the author\'s title for every other kind of choice', () => {
    const session = auctionSession('player_1');
    session.state.pendingAction = { type: 'choice', playerId: 'player_1', title: 'Buy the dock?' };
    session.state.pendingAuction = undefined;
    expect(pendingHeading(session, ruT, 'room.chooseAction')).toBe('Buy the dock?');
  });
});
