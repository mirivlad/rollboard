import type { GameDefinition, ActionDefinition } from './types';

function action(type: string, extra: Partial<ActionDefinition> = {}): ActionDefinition {
  return { type, ...extra };
}

export function createDefaultGame(): GameDefinition {
  return {
    id: '',
    title: 'New Game',
    version: 1,
    board: {
      width: 1200,
      height: 800,
      cellSize: 96,
      cells: [
        {
          id: 'start',
          title: 'Start',
          type: 'start',
          x: 96,
          y: 96,
          visual: { baseColor: '#4CAF50', baseImage: '' },
          fields: {},
        },
      ],
      edges: [],
    },
    rules: {
      dice: { count: 1, sides: 6 },
      resources: {
        money: { initial: 500 },
      },
      cellTypes: {
        start: { title: 'Start', fields: {} },
        empty: { title: 'Empty', fields: {} },
        property: {
          title: 'Property',
          fields: {
            cost: { type: 'number', label: 'Cost', default: 100 },
            rent: { type: 'number', label: 'Rent', default: 20 },
            colorGroup: { type: 'string', label: 'Color Group', default: '' },
          },
        },
        bonus: {
          title: 'Bonus',
          fields: {
            amount: { type: 'number', label: 'Amount', default: 50 },
          },
        },
        penalty: {
          title: 'Penalty',
          fields: {
            amount: { type: 'number', label: 'Amount', default: 40 },
          },
        },
      },
      startBonus: 100,
      startBonusResource: 'money',
    },
  };
}

export function createMiniMonopolyDemo(): GameDefinition {
  const cell = (id: string, title: string, type: string, x: number, y: number, color: string, fields: Record<string, any> = {}, onLand: ActionDefinition[] = []) => ({
    id, title, type, x, y, visual: { baseColor: color, baseImage: '' }, fields, onLand,
  });

  const propertyOnLand = (costField: string, rentField: string): ActionDefinition[] => [
    action('if_cell_unowned', {
      then: [
        action('offer_choice', {
          title: 'Buy this property?',
          options: [
            {
              id: 'buy_property', title: 'Buy',
              then: [
                action('lose_resource', { resource: 'money', amountField: costField }),
                action('set_cell_owner', { target: 'current' }),
              ],
            },
            {
              id: 'skip_purchase', title: "Don't Buy",
              then: [],
            },
          ],
        }),
      ],
      else: [
        action('if_cell_owned_by_other', {
          then: [
            action('transfer_resource', { resource: 'money', amountField: rentField, target: 'owner' }),
          ],
        }),
      ],
    }),
  ];

  const bonusOnLand = (field: string): ActionDefinition[] => [
    action('gain_resource', { resource: 'money', amountField: field }),
  ];

  const penaltyOnLand = (field: string): ActionDefinition[] => [
    action('lose_resource', { resource: 'money', amountField: field }),
  ];

  return {
    id: '',
    title: 'Mini-Monopoly Demo',
    version: 1,
    board: {
      width: 800,
      height: 700,
      cellSize: 96,
      cells: [
        cell('cell_1', 'Start', 'start', 50, 550, '#4CAF50'),
        cell('cell_2', 'Street A', 'property', 200, 550, '#E3F2FD', { cost: 100, rent: 20, colorGroup: 'brown' }, propertyOnLand('cost', 'rent')),
        cell('cell_3', 'Bonus +50', 'bonus', 350, 550, '#C8E6C9', { amount: 50 }, bonusOnLand('amount')),
        cell('cell_4', 'Street B', 'property', 500, 550, '#E3F2FD', { cost: 120, rent: 25, colorGroup: 'brown' }, propertyOnLand('cost', 'rent')),
        cell('cell_5', 'Penalty -40', 'penalty', 650, 550, '#FFCDD2', { amount: 40 }, penaltyOnLand('amount')),
        cell('cell_6', 'Street C', 'property', 50, 400, '#FFE0B2', { cost: 150, rent: 30, colorGroup: 'orange' }, propertyOnLand('cost', 'rent')),
        cell('cell_7', 'Empty', 'empty', 200, 400, '#F5F5F5'),
        cell('cell_8', 'Street D', 'property', 350, 400, '#FFE0B2', { cost: 180, rent: 35, colorGroup: 'orange' }, propertyOnLand('cost', 'rent')),
        cell('cell_9', 'Bonus +70', 'bonus', 500, 400, '#C8E6C9', { amount: 70 }, bonusOnLand('amount')),
        cell('cell_10', 'Street E', 'property', 650, 400, '#FFF9C4', { cost: 200, rent: 40, colorGroup: 'yellow' }, propertyOnLand('cost', 'rent')),
        cell('cell_11', 'Penalty -60', 'penalty', 50, 250, '#FFCDD2', { amount: 60 }, penaltyOnLand('amount')),
        cell('cell_12', 'Street F', 'property', 200, 250, '#FFF9C4', { cost: 220, rent: 45, colorGroup: 'yellow' }, propertyOnLand('cost', 'rent')),
        cell('cell_13', 'Empty', 'empty', 350, 250, '#F5F5F5'),
        cell('cell_14', 'Street G', 'property', 500, 250, '#E1BEE7', { cost: 240, rent: 50, colorGroup: 'purple' }, propertyOnLand('cost', 'rent')),
        cell('cell_15', 'Bonus +100', 'bonus', 650, 250, '#C8E6C9', { amount: 100 }, bonusOnLand('amount')),
        cell('cell_16', 'Street H', 'property', 50, 100, '#E1BEE7', { cost: 260, rent: 55, colorGroup: 'purple' }, propertyOnLand('cost', 'rent')),
      ],
      edges: ['e1_cell_1_cell_2', 'e2_cell_2_cell_3', 'e3_cell_3_cell_4', 'e4_cell_4_cell_5',
        'e5_cell_5_cell_6', 'e6_cell_6_cell_7', 'e7_cell_7_cell_8', 'e8_cell_8_cell_9',
        'e9_cell_9_cell_10', 'e10_cell_10_cell_11', 'e11_cell_11_cell_12', 'e12_cell_12_cell_13',
        'e13_cell_13_cell_14', 'e14_cell_14_cell_15', 'e15_cell_15_cell_16', 'e16_cell_16_cell_1',
      ].map((id, i) => {
        const cells = id.split('_').slice(1);
        return { id, from: cells[0], to: cells[1], condition: { type: 'always' } };
      }),
    },
    rules: {
      dice: { count: 1, sides: 6 },
      resources: { money: { initial: 500 } },
      cellTypes: {
        start: { title: 'Start', fields: {} },
        empty: { title: 'Empty', fields: {} },
        property: {
          title: 'Property',
          fields: {
            cost: { type: 'number', label: 'Cost', default: 100 },
            rent: { type: 'number', label: 'Rent', default: 20 },
            colorGroup: { type: 'string', label: 'Color Group', default: '' },
          },
        },
        bonus: { title: 'Bonus', fields: { amount: { type: 'number', label: 'Amount', default: 50 } } },
        penalty: { title: 'Penalty', fields: { amount: { type: 'number', label: 'Amount', default: 40 } } },
      },
      startBonus: 100,
      startBonusResource: 'money',
    },
  };
}

export function createDungeonRaceDemo(): GameDefinition {
  const cell = (id: string, title: string, type: string, x: number, y: number, color: string, onLand: ActionDefinition[] = []) => ({
    id, title, type, x, y, visual: { baseColor: color, baseImage: '' }, fields: {}, onLand,
  });

  return {
    id: '',
    title: 'Dungeon Race Demo',
    version: 1,
    board: {
      width: 1100,
      height: 400,
      cellSize: 96,
      cells: [
        cell('start', 'Start', 'start', 50, 150, '#4CAF50'),
        cell('trap', 'Trap -2 HP', 'trap', 200, 150, '#FFCDD2', [
          action('lose_resource', { resource: 'health', amount: 2 }),
        ]),
        cell('treasure', 'Treasure +5 Gold', 'treasure', 350, 150, '#FFF9C4', [
          action('gain_resource', { resource: 'gold', amount: 5 }),
        ]),
        cell('key', 'Key +1', 'key', 500, 150, '#E1BEE7', [
          action('gain_resource', { resource: 'keys', amount: 1 }),
        ]),
        cell('heal', 'Heal +2 HP', 'heal', 650, 150, '#C8E6C9', [
          action('gain_resource', { resource: 'health', amount: 2 }),
        ]),
        cell('finish', 'Finish!', 'finish', 800, 150, '#FFD700', [
          action('finish_game'),
        ]),
      ],
      edges: [
        { id: 'e1', from: 'start', to: 'trap', condition: { type: 'always' } },
        { id: 'e2', from: 'trap', to: 'treasure', condition: { type: 'always' } },
        { id: 'e3', from: 'treasure', to: 'key', condition: { type: 'always' } },
        { id: 'e4', from: 'key', to: 'heal', condition: { type: 'always' } },
        { id: 'e5', from: 'heal', to: 'finish', condition: { type: 'always' } },
      ],
    },
    rules: {
      dice: { count: 1, sides: 6 },
      resources: {
        health: { initial: 10, min: 0, max: 10 },
        gold: { initial: 0, min: 0 },
        keys: { initial: 0, min: 0 },
      },
      cellTypes: {
        start: { title: 'Start', fields: {} },
        trap: { title: 'Trap', fields: { amount: { type: 'number', label: 'Damage', default: 2 } } },
        treasure: { title: 'Treasure', fields: { amount: { type: 'number', label: 'Gold', default: 5 } } },
        key: { title: 'Key', fields: { amount: { type: 'number', label: 'Keys', default: 1 } } },
        heal: { title: 'Heal', fields: { amount: { type: 'number', label: 'HP', default: 2 } } },
        finish: { title: 'Finish', fields: {} },
      },
      startBonus: 0,
      startBonusResource: '',
    },
  };
}
