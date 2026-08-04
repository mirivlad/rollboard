import type {
  ActionDefinition,
  AmountFormula,
  CellDefinition,
  CellQuery,
  EdgeDefinition,
  GameDefinition,
} from './types';

/**
 * A full 40-square property game, expressed entirely as data.
 *
 * This template exists as much to prove a point as to be played: everything
 * below — buying, tiered rent, building, mortgaging, jail, chance cards,
 * bankruptcy — is a `GameDefinition`. The engine contains no notion of a
 * "property", a "house" or a "jail"; it only executes actions.
 *
 * The three rules that used to be impossible here are the reason cell queries
 * and auctions exist, and they are written with the same actions any author
 * gets from the editor:
 *
 *   - A colour group pays double once one player owns all of it: a query for
 *     "cells in this group the owner holds" compared against "cells in this
 *     group", so adding a square to a group needs no change here.
 *   - Station rent multiplies by how many stations the landlord owns.
 *   - Declining to buy sends the square to auction, and everybody bids.
 *
 * What it still does not do: nothing in the game reads the dice after the
 * move, so utility rent scales with how many utilities the owner holds rather
 * than with the roll.
 */

const GO_SALARY = 200;
const JAIL_TURNS = 2;
const BAIL = 50;

function action(type: string, extra: Partial<ActionDefinition> = {}): ActionDefinition {
  return { type, ...extra };
}

/** The rent the landlord is owed, however it was worked out. */
function payRent(formula: AmountFormula): ActionDefinition {
  // A player who cannot cover the rent is out. Without this the engine floors
  // money at zero and the game never ends.
  return action('if_resource_ge', {
    resource: 'money',
    formula,
    then: [action('transfer_resource', { resource: 'money', formula, target: 'owner' })],
    else: [
      action('transfer_resource', { resource: 'money', formula, target: 'owner' }),
      action('log_message', { title: 'Cannot cover the rent.' }),
      action('eliminate_player'),
    ],
  });
}

const fromField = (name: string): AmountFormula => ({ base: { kind: 'field', name } });

/** Every square in the same colour group as this one. */
const sameGroup = (owner?: CellQuery['owner']): CellQuery => ({
  type: 'property',
  field: 'group',
  sameAsCell: true,
  ...(owner ? { owner } : {}),
});

/**
 * Rent that grows with the buildings on the square, and doubles on a bare
 * square whose whole colour group has one owner.
 *
 * The monopoly check compares two queries rather than a count against a
 * number: "the squares in this group the landlord owns" against "the squares
 * in this group". Adding a fourth square to a group therefore changes nothing
 * here — the rule is written in terms of the board, not of a total somebody
 * has to remember to update.
 *
 * The tiers are a descending chain of level checks because the engine has no
 * table lookup: the first level that matches wins, so the order matters.
 */
function rentByLevel(): ActionDefinition {
  return action('if_cell_mortgaged', {
    then: [action('log_message', { title: 'The square is mortgaged, so no rent is due.' })],
    else: [
      action('if_cell_level_ge', {
        amount: 5,
        then: [payRent(fromField('rentHotel'))],
        else: [
          action('if_cell_level_ge', {
            amount: 3,
            then: [payRent(fromField('rent3'))],
            else: [
              action('if_cell_level_ge', {
                amount: 1,
                then: [payRent(fromField('rent1'))],
                else: [
                  action('if_cells_ge', {
                    // Owner here is the landlord, not the visitor standing on
                    // the square: it is their holdings that set the rent.
                    query: sameGroup('cellOwner'),
                    formula: { base: { kind: 'cells', query: sameGroup() } },
                    then: [
                      action('log_message', { title: 'One owner holds the whole colour group: rent is doubled.' }),
                      payRent({ base: { kind: 'field', name: 'rent' }, times: { kind: 'const', value: 2 } }),
                    ],
                    else: [payRent(fromField('rent'))],
                  }),
                ],
              }),
            ],
          }),
        ],
      }),
    ],
  });
}

/**
 * Rent for a square whose value is how many of its kind the landlord holds:
 * stations and utilities.
 */
function rentByHoldings(type: string): ActionDefinition {
  return action('if_cell_mortgaged', {
    then: [action('log_message', { title: 'The square is mortgaged, so no rent is due.' })],
    else: [
      payRent({
        base: { kind: 'field', name: 'rent' },
        times: { kind: 'cells', query: { type, owner: 'cellOwner' } },
      }),
    ],
  });
}

/**
 * The square goes under the hammer.
 *
 * This is the rule a two-player trade could never cover: everybody at the
 * table gets to bid, in turn, and the pending action moves round with them.
 */
function auctionThisSquare(): ActionDefinition {
  return action('start_auction', {
    resource: 'money',
    // Bidding opens at half the face value, so a square nobody wants at full
    // price still finds a buyer.
    amountField: 'mortgageValue',
    increment: 10,
    then: [action('set_cell_owner', { target: 'current' })],
    else: [action('log_message', { title: 'Nobody bid: the square stays with the bank.' })],
  });
}

/**
 * The offer made on an unowned square: buy it at face value, or let the table
 * bid for it.
 */
function buyOrAuction(): ActionDefinition {
  return action('if_resource_ge', {
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
          { id: 'pass', title: 'Leave it (it goes to auction)', then: [auctionThisSquare()] },
        ],
      }),
    ],
    else: [
      action('log_message', { title: 'Not enough money to buy this square, so it goes to auction.' }),
      auctionThisSquare(),
    ],
  });
}

/** A station or a utility: bought the same way, rented by the query. */
function holdingActions(type: string): ActionDefinition[] {
  return [
    action('if_cell_unowned', {
      then: [buyOrAuction()],
      else: [action('if_cell_owned_by_other', { then: [rentByHoldings(type)], else: [] })],
    }),
  ];
}

/** Landing on a square somebody may own, or may want to buy. */
function propertyActions(): ActionDefinition[] {
  return [
    action('if_cell_unowned', {
      then: [buyOrAuction()],
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
          title: 'Street repairs: pay 40 for every square you have built on',
          then: [
            // A card that reads the whole board rather than charging a flat
            // fee: the loop visits each built square the player owns.
            action('for_each_cell', {
              query: { type: 'property', owner: 'current', minLevel: 1 },
              then: [action('lose_resource', { resource: 'money', amount: 40 })],
              else: [action('log_message', { title: 'Nothing built, nothing to repair.' })],
            }),
          ],
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
  /**
   * The colour group, as a name rather than the hex colour.
   *
   * Queries compare field values as text, and an author looking at the
   * dropdown in the editor should see "brown", not "#8B4513".
   */
  group: string;
};

type Special = { id: string; title: string; kind: string; cost?: number; rent?: number };

/** The board, walking clockwise from GO. */
const STREETS: (Street | Special)[] = [
  { id: 'go', title: 'GO', kind: 'go' },
  { id: 'old_kent', title: 'Old Kent Road', colour: '#8B4513', cost: 60, rent: 4 , group: 'brown' },
  { id: 'chance_1', title: 'Chance', kind: 'chance' },
  { id: 'whitechapel', title: 'Whitechapel Road', colour: '#8B4513', cost: 60, rent: 8 , group: 'brown' },
  { id: 'income_tax', title: 'Income Tax', kind: 'tax' },
  { id: 'kings_cross', title: "King's Cross Station", kind: 'station', cost: 200, rent: 25 },
  { id: 'angel', title: 'The Angel, Islington', colour: '#87CEEB', cost: 100, rent: 12 , group: 'sky' },
  { id: 'chance_2', title: 'Chance', kind: 'chance' },
  { id: 'euston', title: 'Euston Road', colour: '#87CEEB', cost: 100, rent: 12 , group: 'sky' },
  { id: 'pentonville', title: 'Pentonville Road', colour: '#87CEEB', cost: 120, rent: 16 , group: 'sky' },

  { id: 'jail', title: 'Jail', kind: 'jail' },
  { id: 'pall_mall', title: 'Pall Mall', colour: '#FF69B4', cost: 140, rent: 20 , group: 'pink' },
  { id: 'whitehall', title: 'Whitehall', colour: '#FF69B4', cost: 140, rent: 20 , group: 'pink' },
  { id: 'northumberland', title: 'Northumberland Avenue', colour: '#FF69B4', cost: 160, rent: 24 , group: 'pink' },
  { id: 'marylebone', title: 'Marylebone Station', kind: 'station', cost: 200, rent: 25 },
  { id: 'bow', title: 'Bow Street', colour: '#FFA500', cost: 180, rent: 28 , group: 'orange' },
  { id: 'chance_3', title: 'Chance', kind: 'chance' },
  { id: 'marlborough', title: 'Marlborough Street', colour: '#FFA500', cost: 180, rent: 28 , group: 'orange' },
  { id: 'vine', title: 'Vine Street', colour: '#FFA500', cost: 200, rent: 32 , group: 'orange' },
  { id: 'free_parking', title: 'Free Parking', kind: 'free' },

  { id: 'strand', title: 'Strand', colour: '#DC143C', cost: 220, rent: 36 , group: 'red' },
  { id: 'chance_4', title: 'Chance', kind: 'chance' },
  { id: 'fleet', title: 'Fleet Street', colour: '#DC143C', cost: 220, rent: 36 , group: 'red' },
  { id: 'trafalgar', title: 'Trafalgar Square', colour: '#DC143C', cost: 240, rent: 40 , group: 'red' },
  { id: 'fenchurch', title: 'Fenchurch St Station', kind: 'station', cost: 200, rent: 25 },
  { id: 'leicester', title: 'Leicester Square', colour: '#FFFF00', cost: 260, rent: 44 , group: 'yellow' },
  { id: 'coventry', title: 'Coventry Street', colour: '#FFFF00', cost: 260, rent: 44 , group: 'yellow' },
  { id: 'piccadilly', title: 'Piccadilly', colour: '#FFFF00', cost: 280, rent: 48 , group: 'yellow' },
  { id: 'go_to_jail', title: 'Go To Jail', kind: 'gotojail' },
  { id: 'regent', title: 'Regent Street', colour: '#008000', cost: 300, rent: 52 , group: 'green' },

  { id: 'oxford', title: 'Oxford Street', colour: '#008000', cost: 300, rent: 52 , group: 'green' },
  { id: 'chance_5', title: 'Chance', kind: 'chance' },
  { id: 'bond', title: 'Bond Street', colour: '#008000', cost: 320, rent: 56 , group: 'green' },
  { id: 'liverpool', title: 'Liverpool St Station', kind: 'station', cost: 200, rent: 25 },
  { id: 'chance_6', title: 'Chance', kind: 'chance' },
  { id: 'park_lane', title: 'Park Lane', colour: '#00008B', cost: 350, rent: 70 , group: 'blue' },
  { id: 'super_tax', title: 'Super Tax', kind: 'tax' },
  { id: 'mayfair', title: 'Mayfair', colour: '#00008B', cost: 400, rent: 100 , group: 'blue' },
  { id: 'water_works', title: 'Water Works', kind: 'utility', cost: 150, rent: 20 },
  { id: 'electric', title: 'Electric Company', kind: 'utility', cost: 150, rent: 20 },
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

    if ('group' in square) {
      const rent = square.rent;
      cells.push({
        ...base,
        type: 'property',
        visual: { baseColor: square.colour, baseImage: '' },
        fields: {
          // The group is a field like any other, which is the point: the
          // engine knows nothing about colour groups, the query does.
          group: square.group,
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
        case 'station':
        case 'utility': {
          // Same shape as a street, but rent comes from the query rather than
          // from buildings, so these squares carry no tier fields at all.
          const cost = square.cost ?? 200;
          cells.push({
            ...base,
            type: square.kind,
            visual: { baseColor: square.kind === 'station' ? '#2F4F4F' : '#B0C4DE', baseImage: '' },
            fields: { cost, rent: square.rent ?? 25, mortgageValue: Math.round(cost / 2) },
            onLand: holdingActions(square.kind),
          });
          break;
        }
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
            group: { type: 'string', label: 'Colour group', default: 'brown' },
            cost: { type: 'number', label: 'Cost', default: 100 },
            rent: { type: 'number', label: 'Rent', default: 10 },
            rent1: { type: 'number', label: 'Rent with 1 house', default: 30 },
            rent3: { type: 'number', label: 'Rent with 3 houses', default: 80 },
            rentHotel: { type: 'number', label: 'Rent with a hotel', default: 150 },
            buildCost: { type: 'number', label: 'Cost to build', default: 50 },
            mortgageValue: { type: 'number', label: 'Mortgage value', default: 50 },
          },
        },
        station: {
          title: 'Station',
          fields: {
            cost: { type: 'number', label: 'Cost', default: 200 },
            rent: { type: 'number', label: 'Rent per station owned', default: 25 },
            mortgageValue: { type: 'number', label: 'Mortgage value', default: 100 },
          },
        },
        utility: {
          title: 'Utility',
          fields: {
            cost: { type: 'number', label: 'Cost', default: 150 },
            rent: { type: 'number', label: 'Rent per utility owned', default: 20 },
            mortgageValue: { type: 'number', label: 'Mortgage value', default: 75 },
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
