import type { ActionDefinition, CellDefinition, EdgeDefinition, GameDefinition } from './types';

/**
 * A full 40-square property game, expressed entirely as data.
 *
 * This template exists as much to prove a point as to be played: everything
 * below — buying, tiered rent, building, mortgaging, jail, chance cards,
 * bankruptcy — is a `GameDefinition`. The engine contains no notion of a
 * "property", a "house" or a "jail"; it only executes actions.
 *
 * What it deliberately does not do, because the engine cannot express it yet:
 *
 *   - Colour-group monopolies. Rent cannot depend on whether the same player
 *     owns every square of a group, because no action can query another cell's
 *     state. Rent here scales with buildings alone.
 *   - Trading between players, and auctions. Both need a negotiation between
 *     two players, and a pending action is addressed to exactly one.
 *   - Station rent scaling with how many stations you own. Same missing
 *     cross-cell query as monopolies.
 *
 * Those are documented in docs/ARCHITECTURE.md as the next engine primitives.
 */

const GO_SALARY = 200;
const JAIL_TURNS = 2;
const BAIL = 50;

function action(type: string, extra: Partial<ActionDefinition> = {}): ActionDefinition {
  return { type, ...extra };
}

/**
 * Rent that grows with the buildings on the square.
 *
 * Written as a descending chain of level checks because the engine has no
 * table lookup: the first level that matches wins, so the order matters.
 */
function rentByLevel(): ActionDefinition {
  // A player who cannot cover the rent is out. Without this the engine floors
  // money at zero and the game never ends.
  const payRent = (field: string) =>
    action('if_resource_ge', {
      resource: 'money',
      amountField: field,
      then: [action('transfer_resource', { resource: 'money', amountField: field, target: 'owner' })],
      else: [
        action('transfer_resource', { resource: 'money', amountField: field, target: 'owner' }),
        action('log_message', { title: 'Cannot cover the rent.' }),
        action('eliminate_player'),
      ],
    });

  return action('if_cell_mortgaged', {
    then: [action('log_message', { title: 'The square is mortgaged, so no rent is due.' })],
    else: [
      action('if_cell_level_ge', {
        amount: 5,
        then: [payRent('rentHotel')],
        else: [
          action('if_cell_level_ge', {
            amount: 3,
            then: [payRent('rent3')],
            else: [
              action('if_cell_level_ge', {
                amount: 1,
                then: [payRent('rent1')],
                else: [payRent('rent')],
              }),
            ],
          }),
        ],
      }),
    ],
  });
}

/** Landing on a square somebody may own, or may want to buy. */
function propertyActions(): ActionDefinition[] {
  return [
    action('if_cell_unowned', {
      then: [
        action('if_resource_ge', {
          resource: 'money',
          amountField: 'cost',
          then: [
            action('offer_choice', {
              title: 'Buy this square?',
              options: [
                {
                  id: 'buy',
                  title: 'Buy it',
                  then: [
                    action('lose_resource', { resource: 'money', amountField: 'cost' }),
                    action('set_cell_owner', { target: 'current' }),
                  ],
                },
                { id: 'pass', title: 'Leave it', then: [] },
              ],
            }),
          ],
          else: [action('log_message', { title: 'Not enough money to buy this square.' })],
        }),
      ],
      else: [
        action('if_cell_owned_by_other', {
          then: [rentByLevel()],
          // Your own square: build, mortgage, or do nothing.
          else: [
            action('offer_choice', {
              title: 'Your square. Develop it?',
              options: [
                {
                  id: 'build',
                  title: 'Build (cost: buildCost)',
                  then: [
                    action('if_cell_level_ge', {
                      amount: 5,
                      then: [action('log_message', { title: 'Already fully built.' })],
                      else: [
                        action('if_resource_ge', {
                          resource: 'money',
                          amountField: 'buildCost',
                          then: [
                            action('lose_resource', { resource: 'money', amountField: 'buildCost' }),
                            // Levels step one at a time; the chain below reads
                            // the current level and sets the next one.
                            action('if_cell_level_ge', {
                              amount: 3,
                              then: [action('set_cell_level', { amount: 5 })],
                              else: [
                                action('if_cell_level_ge', {
                                  amount: 1,
                                  then: [action('set_cell_level', { amount: 3 })],
                                  else: [action('set_cell_level', { amount: 1 })],
                                }),
                              ],
                            }),
                          ],
                          else: [action('log_message', { title: 'Not enough money to build.' })],
                        }),
                      ],
                    }),
                  ],
                },
                {
                  id: 'mortgage',
                  title: 'Mortgage for half the cost',
                  then: [
                    action('if_cell_mortgaged', {
                      then: [action('log_message', { title: 'Already mortgaged.' })],
                      else: [
                        action('set_cell_mortgaged', { target: 'true' }),
                        action('gain_resource', { resource: 'money', amountField: 'mortgageValue' }),
                      ],
                    }),
                  ],
                },
                {
                  id: 'redeem',
                  title: 'Redeem the mortgage',
                  then: [
                    action('if_cell_mortgaged', {
                      then: [
                        action('if_resource_ge', {
                          resource: 'money',
                          amountField: 'cost',
                          then: [
                            action('lose_resource', { resource: 'money', amountField: 'cost' }),
                            action('set_cell_mortgaged', { target: 'false' }),
                          ],
                          else: [action('log_message', { title: 'Not enough money to redeem.' })],
                        }),
                      ],
                      else: [action('log_message', { title: 'This square is not mortgaged.' })],
                    }),
                  ],
                },
                { id: 'nothing', title: 'Do nothing', then: [] },
              ],
            }),
          ],
        }),
      ],
    }),
  ];
}

/** Chance and community chest, resolved by the server's own RNG. */
function chanceActions(): ActionDefinition[] {
  return [
    action('random_branch', {
      options: [
        {
          id: 'bank_dividend',
          title: 'Bank pays a dividend of 100',
          then: [action('gain_resource', { resource: 'money', amount: 100 })],
        },
        {
          id: 'doctor_fee',
          title: "Doctor's fee: pay 60",
          then: [action('lose_resource', { resource: 'money', amount: 60 })],
        },
        {
          id: 'advance_to_go',
          title: 'Advance to GO and collect your salary',
          then: [action('move_player_to', { to: 'go' })],
        },
        {
          id: 'go_to_jail',
          title: 'Go directly to jail',
          then: [action('move_player_to', { to: 'jail' })],
        },
        {
          id: 'street_repairs',
          title: 'Street repairs: pay 120',
          then: [action('lose_resource', { resource: 'money', amount: 120 })],
        },
        {
          id: 'beauty_contest',
          title: 'Second prize in a beauty contest: collect 40',
          then: [action('gain_resource', { resource: 'money', amount: 40 })],
        },
      ],
    }),
  ];
}

type Street = {
  id: string;
  title: string;
  colour: string;
  cost: number;
  rent: number;
};

/** The board, walking clockwise from GO. */
const STREETS: (Street | { id: string; title: string; kind: string })[] = [
  { id: 'go', title: 'GO', kind: 'go' },
  { id: 'old_kent', title: 'Old Kent Road', colour: '#8B4513', cost: 60, rent: 4 },
  { id: 'chance_1', title: 'Chance', kind: 'chance' },
  { id: 'whitechapel', title: 'Whitechapel Road', colour: '#8B4513', cost: 60, rent: 8 },
  { id: 'income_tax', title: 'Income Tax', kind: 'tax' },
  { id: 'kings_cross', title: "King's Cross Station", colour: '#2F4F4F', cost: 200, rent: 25 },
  { id: 'angel', title: 'The Angel, Islington', colour: '#87CEEB', cost: 100, rent: 12 },
  { id: 'chance_2', title: 'Chance', kind: 'chance' },
  { id: 'euston', title: 'Euston Road', colour: '#87CEEB', cost: 100, rent: 12 },
  { id: 'pentonville', title: 'Pentonville Road', colour: '#87CEEB', cost: 120, rent: 16 },

  { id: 'jail', title: 'Jail', kind: 'jail' },
  { id: 'pall_mall', title: 'Pall Mall', colour: '#FF69B4', cost: 140, rent: 20 },
  { id: 'whitehall', title: 'Whitehall', colour: '#FF69B4', cost: 140, rent: 20 },
  { id: 'northumberland', title: 'Northumberland Avenue', colour: '#FF69B4', cost: 160, rent: 24 },
  { id: 'marylebone', title: 'Marylebone Station', colour: '#2F4F4F', cost: 200, rent: 25 },
  { id: 'bow', title: 'Bow Street', colour: '#FFA500', cost: 180, rent: 28 },
  { id: 'chance_3', title: 'Chance', kind: 'chance' },
  { id: 'marlborough', title: 'Marlborough Street', colour: '#FFA500', cost: 180, rent: 28 },
  { id: 'vine', title: 'Vine Street', colour: '#FFA500', cost: 200, rent: 32 },
  { id: 'free_parking', title: 'Free Parking', kind: 'free' },

  { id: 'strand', title: 'Strand', colour: '#DC143C', cost: 220, rent: 36 },
  { id: 'chance_4', title: 'Chance', kind: 'chance' },
  { id: 'fleet', title: 'Fleet Street', colour: '#DC143C', cost: 220, rent: 36 },
  { id: 'trafalgar', title: 'Trafalgar Square', colour: '#DC143C', cost: 240, rent: 40 },
  { id: 'fenchurch', title: 'Fenchurch St Station', colour: '#2F4F4F', cost: 200, rent: 25 },
  { id: 'leicester', title: 'Leicester Square', colour: '#FFFF00', cost: 260, rent: 44 },
  { id: 'coventry', title: 'Coventry Street', colour: '#FFFF00', cost: 260, rent: 44 },
  { id: 'piccadilly', title: 'Piccadilly', colour: '#FFFF00', cost: 280, rent: 48 },
  { id: 'go_to_jail', title: 'Go To Jail', kind: 'gotojail' },
  { id: 'regent', title: 'Regent Street', colour: '#008000', cost: 300, rent: 52 },

  { id: 'oxford', title: 'Oxford Street', colour: '#008000', cost: 300, rent: 52 },
  { id: 'chance_5', title: 'Chance', kind: 'chance' },
  { id: 'bond', title: 'Bond Street', colour: '#008000', cost: 320, rent: 56 },
  { id: 'liverpool', title: 'Liverpool St Station', colour: '#2F4F4F', cost: 200, rent: 25 },
  { id: 'chance_6', title: 'Chance', kind: 'chance' },
  { id: 'park_lane', title: 'Park Lane', colour: '#00008B', cost: 350, rent: 70 },
  { id: 'super_tax', title: 'Super Tax', kind: 'tax' },
  { id: 'mayfair', title: 'Mayfair', colour: '#00008B', cost: 400, rent: 100 },
  { id: 'water_works', title: 'Water Works', colour: '#B0C4DE', cost: 150, rent: 20 },
  { id: 'electric', title: 'Electric Company', colour: '#B0C4DE', cost: 150, rent: 20 },
];

const CELL = 96;
const PER_SIDE = 10;

/** Lay the 40 squares out as a ring, so the board reads like a real one. */
function positionFor(index: number): { x: number; y: number } {
  const side = Math.floor(index / PER_SIDE);
  const offset = index % PER_SIDE;
  switch (side) {
    case 0:
      return { x: offset * CELL, y: 0 };
    case 1:
      return { x: PER_SIDE * CELL, y: offset * CELL };
    case 2:
      return { x: (PER_SIDE - offset) * CELL, y: PER_SIDE * CELL };
    default:
      return { x: 0, y: (PER_SIDE - offset) * CELL };
  }
}

export function createMonopolyDemo(): GameDefinition {
  const cells: CellDefinition[] = [];
  const edges: EdgeDefinition[] = [];

  STREETS.forEach((square, index) => {
    const { x, y } = positionFor(index);
    const base: Omit<CellDefinition, 'type' | 'fields' | 'onLand'> = {
      id: square.id,
      title: square.title,
      x,
      y,
      visual: { baseColor: '#F5F5F5', baseImage: '' },
    };

    if ('colour' in square) {
      const rent = square.rent;
      cells.push({
        ...base,
        type: 'property',
        visual: { baseColor: square.colour, baseImage: '' },
        fields: {
          cost: square.cost,
          rent,
          // Tiered rent, read by the level chain in rentByLevel().
          rent1: rent * 3,
          rent3: rent * 8,
          rentHotel: rent * 15,
          buildCost: Math.round(square.cost / 2),
          mortgageValue: Math.round(square.cost / 2),
        },
        onLand: propertyActions(),
      });
    } else {
      switch (square.kind) {
        case 'go':
          cells.push({ ...base, type: 'start', visual: { baseColor: '#4CAF50', baseImage: '' }, fields: {}, onLand: [] });
          break;
        case 'chance':
          cells.push({ ...base, type: 'chance', visual: { baseColor: '#FFE082', baseImage: '' }, fields: {}, onLand: chanceActions() });
          break;
        case 'tax':
          cells.push({
            ...base,
            type: 'tax',
            visual: { baseColor: '#EF9A9A', baseImage: '' },
            fields: { amount: square.id === 'super_tax' ? 100 : 200 },
            onLand: [
              action('if_resource_ge', {
                resource: 'money',
                amountField: 'amount',
                then: [action('lose_resource', { resource: 'money', amountField: 'amount' })],
                else: [action('lose_resource', { resource: 'money', amountField: 'amount' }), action('eliminate_player')],
              }),
            ],
          });
          break;
        case 'jail':
          // Simply visiting is free; you only serve time if sent here.
          cells.push({ ...base, type: 'jail', visual: { baseColor: '#BCAAA4', baseImage: '' }, fields: {}, onLand: [] });
          break;
        case 'gotojail':
          cells.push({
            ...base,
            type: 'gotojail',
            visual: { baseColor: '#B71C1C', baseImage: '' },
            fields: {},
            onLand: [
              action('move_player_to', { to: 'jail' }),
              action('offer_choice', {
                title: 'You are in jail. Pay bail?',
                options: [
                  {
                    id: 'bail',
                    title: `Pay ${BAIL} and keep playing`,
                    then: [action('lose_resource', { resource: 'money', amount: BAIL })],
                  },
                  {
                    id: 'serve',
                    title: `Sit out ${JAIL_TURNS} turns`,
                    then: [action('skip_turns', { amount: JAIL_TURNS })],
                  },
                ],
              }),
            ],
          });
          break;
        default:
          cells.push({ ...base, type: 'empty', fields: {}, onLand: [] });
      }
    }

    const next = STREETS[(index + 1) % STREETS.length];
    edges.push({ id: `e_${square.id}`, from: square.id, to: next.id, condition: { type: 'always' } });
  });

  return {
    id: '',
    title: 'Monopoly',
    version: 1,
    board: {
      width: (PER_SIDE + 1) * CELL,
      height: (PER_SIDE + 1) * CELL,
      cellSize: CELL,
      cells,
      edges,
    },
    rules: {
      dice: { count: 2, sides: 6 },
      resources: { money: { initial: 1500 } },
      cellTypes: {
        start: { title: 'GO', fields: {} },
        property: {
          title: 'Property',
          fields: {
            cost: { type: 'number', label: 'Cost', default: 100 },
            rent: { type: 'number', label: 'Rent', default: 10 },
            rent1: { type: 'number', label: 'Rent with 1 house', default: 30 },
            rent3: { type: 'number', label: 'Rent with 3 houses', default: 80 },
            rentHotel: { type: 'number', label: 'Rent with a hotel', default: 150 },
            buildCost: { type: 'number', label: 'Cost to build', default: 50 },
            mortgageValue: { type: 'number', label: 'Mortgage value', default: 50 },
          },
        },
        chance: { title: 'Chance', fields: {} },
        tax: { title: 'Tax', fields: { amount: { type: 'number', label: 'Amount', default: 100 } } },
        jail: { title: 'Jail', fields: {} },
        gotojail: { title: 'Go To Jail', fields: {} },
        free: { title: 'Free Parking', fields: {} },
        empty: { title: 'Empty', fields: {} },
      },
      startBonus: GO_SALARY,
      startBonusResource: 'money',
    },
  };
}
