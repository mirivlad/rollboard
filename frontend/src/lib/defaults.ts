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
      width: 1152,
      height: 768,
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
    id: 'mini-monopoly-demo',
    title: 'Mini-Monopoly Demo',
    version: 1,
    board: {
      width: 864,
      height: 768,
      cellSize: 96,
      cells: [
        cell('cell_1', 'Start', 'start', 0, 576, '#4CAF50'),
        cell('cell_2', 'Street A', 'property', 192, 576, '#E3F2FD', { cost: 100, rent: 20, colorGroup: 'brown' }, propertyOnLand('cost', 'rent')),
        cell('cell_3', 'Bonus +50', 'bonus', 384, 576, '#C8E6C9', { amount: 50 }, bonusOnLand('amount')),
        cell('cell_4', 'Street B', 'property', 576, 576, '#E3F2FD', { cost: 120, rent: 25, colorGroup: 'brown' }, propertyOnLand('cost', 'rent')),
        cell('cell_5', 'Penalty -40', 'penalty', 0, 480, '#FFCDD2', { amount: 40 }, penaltyOnLand('amount')),
        cell('cell_6', 'Street C', 'property', 192, 480, '#FFE0B2', { cost: 150, rent: 30, colorGroup: 'orange' }, propertyOnLand('cost', 'rent')),
        cell('cell_7', 'Empty', 'empty', 384, 480, '#F5F5F5'),
        cell('cell_8', 'Street D', 'property', 576, 480, '#FFE0B2', { cost: 180, rent: 35, colorGroup: 'orange' }, propertyOnLand('cost', 'rent')),
        cell('cell_9', 'Bonus +70', 'bonus', 0, 384, '#C8E6C9', { amount: 70 }, bonusOnLand('amount')),
        cell('cell_10', 'Street E', 'property', 192, 384, '#FFF9C4', { cost: 200, rent: 40, colorGroup: 'yellow' }, propertyOnLand('cost', 'rent')),
        cell('cell_11', 'Penalty -60', 'penalty', 384, 384, '#FFCDD2', { amount: 60 }, penaltyOnLand('amount')),
        cell('cell_12', 'Street F', 'property', 576, 384, '#FFF9C4', { cost: 220, rent: 45, colorGroup: 'yellow' }, propertyOnLand('cost', 'rent')),
        cell('cell_13', 'Empty', 'empty', 0, 288, '#F5F5F5'),
        cell('cell_14', 'Street G', 'property', 192, 288, '#E1BEE7', { cost: 240, rent: 50, colorGroup: 'purple' }, propertyOnLand('cost', 'rent')),
        cell('cell_15', 'Bonus +100', 'bonus', 384, 288, '#C8E6C9', { amount: 100 }, bonusOnLand('amount')),
        cell('cell_16', 'Street H', 'property', 576, 288, '#E1BEE7', { cost: 260, rent: 55, colorGroup: 'purple' }, propertyOnLand('cost', 'rent')),
      ],
      edges: [
        { id: 'e1',  from: 'cell_1',  to: 'cell_2',  condition: { type: 'always' } },
        { id: 'e2',  from: 'cell_2',  to: 'cell_3',  condition: { type: 'always' } },
        { id: 'e3',  from: 'cell_3',  to: 'cell_4',  condition: { type: 'always' } },
        { id: 'e4',  from: 'cell_4',  to: 'cell_5',  condition: { type: 'always' } },
        { id: 'e5',  from: 'cell_5',  to: 'cell_6',  condition: { type: 'always' } },
        { id: 'e6',  from: 'cell_6',  to: 'cell_7',  condition: { type: 'always' } },
        { id: 'e7',  from: 'cell_7',  to: 'cell_8',  condition: { type: 'always' } },
        { id: 'e8',  from: 'cell_8',  to: 'cell_9',  condition: { type: 'always' } },
        { id: 'e9',  from: 'cell_9',  to: 'cell_10', condition: { type: 'always' } },
        { id: 'e10', from: 'cell_10', to: 'cell_11', condition: { type: 'always' } },
        { id: 'e11', from: 'cell_11', to: 'cell_12', condition: { type: 'always' } },
        { id: 'e12', from: 'cell_12', to: 'cell_13', condition: { type: 'always' } },
        { id: 'e13', from: 'cell_13', to: 'cell_14', condition: { type: 'always' } },
        { id: 'e14', from: 'cell_14', to: 'cell_15', condition: { type: 'always' } },
        { id: 'e15', from: 'cell_15', to: 'cell_16', condition: { type: 'always' } },
        { id: 'e16', from: 'cell_16', to: 'cell_1',  condition: { type: 'always' } },
      ],
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
    id: 'dungeon-race-demo',
    title: 'Dungeon Race Demo',
    version: 1,
    board: {
      width: 1056,
      height: 384,
      cellSize: 96,
      cells: [
        cell('start', 'Start', 'start', 0, 96, '#4CAF50'),
        cell('trap', 'Trap -2 HP', 'trap', 192, 96, '#FFCDD2', [
          action('lose_resource', { resource: 'health', amount: 2 }),
        ]),
        cell('treasure', 'Treasure +5 Gold', 'treasure', 384, 96, '#FFF9C4', [
          action('gain_resource', { resource: 'gold', amount: 5 }),
        ]),
        cell('key', 'Key +1', 'key', 576, 96, '#E1BEE7', [
          action('gain_resource', { resource: 'keys', amount: 1 }),
        ]),
        cell('heal', 'Heal +2 HP', 'heal', 768, 96, '#C8E6C9', [
          action('gain_resource', { resource: 'health', amount: 2 }),
        ]),
        cell('finish', 'Finish!', 'finish', 960, 96, '#FFD700', [
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

export function createBranchingDemo(): GameDefinition {
  const cell = (id: string, title: string, type: string, x: number, y: number, color: string, onLand: ActionDefinition[] = []) => ({
    id, title, type, x, y, visual: { baseColor: color, baseImage: '' }, fields: {}, onLand,
  });

  const e = (id: string, from: string, to: string, condition: any) => ({ id, from, to, condition });

  return {
    id: 'branching-demo',
    title: 'Branching Demo',
    version: 1,
    board: {
      width: 768,
      height: 672,
      cellSize: 96,
      cells: [
        cell('start', 'Start', 'start', 96, 96, '#4CAF50'),
        cell('fork', 'Fork', 'empty', 96, 288, '#FF9800'),
        cell('even_cell', 'Even Path', 'empty', 0, 480, '#E3F2FD'),
        cell('odd_cell', 'Odd Path', 'empty', 192, 480, '#FFE0B2'),
        cell('finish', 'Finish', 'finish', 96, 576, '#FFD700', [
          action('finish_game'),
        ]),
      ],
      edges: [
        e('e1', 'start', 'fork', { type: 'always' }),
        e('e2', 'fork', 'even_cell', { type: 'dice_total_even' }),
        e('e3', 'fork', 'odd_cell', { type: 'dice_total_odd' }),
        e('e4', 'even_cell', 'finish', { type: 'always' }),
        e('e5', 'odd_cell', 'finish', { type: 'always' }),
      ],
    },
    rules: {
      dice: { count: 1, sides: 6 },
      resources: {},
      cellTypes: {
        start: { title: 'Start', fields: {} },
        empty: { title: 'Empty', fields: {} },
        finish: { title: 'Finish', fields: {} },
      },
      startBonus: 0,
      startBonusResource: '',
    },
  };
}

export function createManualBranchDemo(): GameDefinition {
  const cell = (id: string, title: string, type: string, x: number, y: number, color: string, onLand: ActionDefinition[] = []) => ({
    id, title, type, x, y, visual: { baseColor: color, baseImage: '' }, fields: {}, onLand,
  });

  const e = (id: string, from: string, to: string, condition: any) => ({ id, from, to, condition });

  return {
    id: 'manual-branch-demo',
    title: 'Manual Branch Demo',
    version: 1,
    board: {
      width: 768,
      height: 768,
      cellSize: 96,
      cells: [
        cell('start', 'Start', 'start', 96, 96, '#4CAF50'),
        cell('choice_fork', 'Choice Fork', 'empty', 96, 288, '#FF9800'),
        cell('safe', 'Safe Path', 'empty', 0, 480, '#C8E6C9'),
        cell('shortcut', 'Shortcut', 'empty', 192, 480, '#FFCDD2'),
        cell('finish', 'Finish', 'finish', 96, 576, '#FFD700', [
          action('finish_game'),
        ]),
      ],
      edges: [
        e('e1', 'start', 'choice_fork', { type: 'always' }),
        e('e2', 'choice_fork', 'safe', { type: 'manual_choice', label: 'Safe Path' }),
        e('e3', 'choice_fork', 'shortcut', { type: 'pay_resource', resource: 'gold', amount: 2, label: 'Pay 2 Gold' }),
        e('e4', 'safe', 'finish', { type: 'always' }),
        e('e5', 'shortcut', 'finish', { type: 'always' }),
      ],
    },
    rules: {
      dice: { count: 1, sides: 3 },
      resources: {
        gold: { initial: 5, min: 0 },
      },
      cellTypes: {
        start: { title: 'Start', fields: {} },
        empty: { title: 'Empty', fields: {} },
        finish: { title: 'Finish', fields: {} },
      },
      startBonus: 0,
      startBonusResource: '',
    },
  };
}
