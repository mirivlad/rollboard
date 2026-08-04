import type { ActionDefinition, CellDefinition, EdgeDefinition, GameDefinition } from './types';

/**
 * A dungeon crawl: an open grid you explore, with stats, levels, loot and a
 * boss to kill.
 *
 * Like the Monopoly template this is entirely a `GameDefinition`. The engine
 * has no idea what a goblin or a sword is; it knows how to run actions, carry
 * items, apply equipment bonuses and turn cards over.
 *
 * The board is a 6×6 grid with edges in all four directions, and
 * `movement: 'free'` means a roll is a budget the player spends on any square
 * within reach rather than a fixed path. Every square except the camp starts
 * face down.
 */

function action(type: string, extra: Partial<ActionDefinition> = {}): ActionDefinition {
  return { type, ...extra };
}

const CELL = 96;
const COLS = 6;
const ROWS = 6;

/**
 * A fight resolved from the player's effective attack against the square's
 * difficulty.
 *
 * The engine has no combat, and deliberately so: this is `if_stat_ge` on a
 * stat the equipped weapon contributes to, with a random branch for the swing
 * of the dice. Winning grants experience, which the progression rule turns into
 * levels on its own.
 */
function fight(difficulty: number, reward: number, damage: number): ActionDefinition {
  return action('if_stat_ge', {
    resource: 'attack',
    amount: difficulty,
    then: [
      action('log_message', { title: 'You overpower it.' }),
      action('gain_resource', { resource: 'experience', amount: reward }),
      action('gain_resource', { resource: 'gold', amount: Math.round(reward / 2) }),
    ],
    else: [
      // Outmatched, but not automatically dead: armour and luck still matter.
      action('random_branch', {
        options: [
          {
            id: 'wounded',
            title: 'It wounds you, but you drive it off',
            then: [
              action('lose_resource', { resource: 'health', amount: damage }),
              action('gain_resource', { resource: 'experience', amount: Math.round(reward / 3) }),
            ],
          },
          {
            id: 'badly_hurt',
            title: 'It gets the better of you',
            then: [action('lose_resource', { resource: 'health', amount: damage * 2 })],
          },
          {
            id: 'lucky',
            title: 'A lucky strike lands',
            then: [
              action('gain_resource', { resource: 'experience', amount: reward }),
              action('lose_resource', { resource: 'health', amount: Math.round(damage / 2) }),
            ],
          },
        ],
      }),
    ],
  });
}

/** Death check, run after anything that can cost health. */
function deathCheck(): ActionDefinition {
  return action('if_resource_ge', {
    resource: 'health',
    amount: 1,
    then: [],
    else: [
      action('log_message', { title: 'Your wounds are too much.' }),
      action('eliminate_player'),
    ],
  });
}

/** The camp: spend the points a level brought you. */
function campActions(): ActionDefinition[] {
  const spend = (stat: string, title: string) => ({
    id: `train_${stat}`,
    title,
    then: [
      action('if_resource_ge', {
        resource: 'points',
        amount: 1,
        then: [
          action('lose_resource', { resource: 'points', amount: 1 }),
          action('gain_resource', { resource: stat, amount: 1 }),
        ],
        else: [action('log_message', { title: 'No training points left.' })],
      }),
    ],
  });

  return [
    action('gain_resource', { resource: 'health', amount: 4 }),
    action('log_message', { title: 'You rest at the camp and recover.' }),
    action('offer_choice', {
      title: 'Spend a training point?',
      options: [
        spend('attack', 'Train attack'),
        spend('defence', 'Train defence'),
        spend('agility', 'Train agility'),
        { id: 'rest', title: 'Just rest', then: [] },
      ],
    }),
  ];
}

type Square =
  | { kind: 'camp' }
  | { kind: 'empty' }
  | { kind: 'enemy'; title: string; difficulty: number; reward: number; damage: number }
  | { kind: 'loot'; title: string; item: string }
  | { kind: 'trap'; title: string; damage: number }
  | { kind: 'shrine'; title: string }
  | { kind: 'boss'; title: string };

/**
 * The map, read row by row. The boss sits in the far corner, so the shortest
 * route there runs past enough fights to make arriving under-levelled a bad
 * idea.
 */
const MAP: Square[] = [
  { kind: 'camp' },
  { kind: 'loot', title: 'Old Chest', item: 'rusty_sword' },
  { kind: 'empty' },
  { kind: 'enemy', title: 'Giant Rat', difficulty: 4, reward: 6, damage: 2 },
  { kind: 'empty' },
  { kind: 'loot', title: 'Supply Crate', item: 'potion' },

  { kind: 'empty' },
  { kind: 'trap', title: 'Spike Pit', damage: 4 },
  { kind: 'enemy', title: 'Goblin', difficulty: 6, reward: 10, damage: 4 },
  { kind: 'empty' },
  { kind: 'loot', title: 'Armour Rack', item: 'leather_armour' },
  { kind: 'empty' },

  { kind: 'shrine', title: 'Scouting Shrine' },
  { kind: 'empty' },
  { kind: 'enemy', title: 'Wolf Pack', difficulty: 8, reward: 14, damage: 5 },
  { kind: 'trap', title: 'Collapsing Floor', damage: 6 },
  { kind: 'empty' },
  { kind: 'loot', title: 'Hidden Cache', item: 'potion' },

  { kind: 'empty' },
  { kind: 'loot', title: 'Weapon Stash', item: 'steel_sword' },
  { kind: 'empty' },
  { kind: 'enemy', title: 'Ogre', difficulty: 11, reward: 20, damage: 7 },
  { kind: 'empty' },
  { kind: 'trap', title: 'Poison Darts', damage: 5 },

  { kind: 'loot', title: 'Deep Vault', item: 'tower_shield' },
  { kind: 'empty' },
  { kind: 'enemy', title: 'Wraith', difficulty: 13, reward: 26, damage: 9 },
  { kind: 'empty' },
  { kind: 'shrine', title: 'Healing Spring' },
  { kind: 'empty' },

  { kind: 'empty' },
  { kind: 'loot', title: 'Champion Arms', item: 'runed_blade' },
  { kind: 'empty' },
  { kind: 'trap', title: 'Rune Trap', damage: 8 },
  { kind: 'empty' },
  { kind: 'boss', title: 'The Dragon' },
];

function cellFor(square: Square, index: number): CellDefinition {
  const x = (index % COLS) * CELL;
  const y = Math.floor(index / COLS) * CELL;
  const id = `c${index}`;
  const base = { id, x, y, fields: {} as Record<string, unknown> };

  switch (square.kind) {
    case 'camp':
      return {
        ...base, id: 'camp', title: 'Camp', type: 'start',
        visual: { baseColor: '#4CAF50', baseImage: '' },
        fields: {}, onLand: campActions(),
      };
    case 'enemy':
      return {
        ...base, title: square.title, type: 'enemy',
        visual: { baseColor: '#B71C1C', baseImage: '' },
        fields: { difficulty: square.difficulty, reward: square.reward },
        onLand: [fight(square.difficulty, square.reward, square.damage), deathCheck()],
      };
    case 'loot':
      return {
        ...base, title: square.title, type: 'loot',
        visual: { baseColor: '#FFD54F', baseImage: '' },
        fields: {},
        onLand: [
          action('grant_item', { field: square.item }),
          action('log_message', { title: 'Something useful, if you care to equip it.' }),
        ],
      };
    case 'trap':
      return {
        ...base, title: square.title, type: 'trap',
        visual: { baseColor: '#8D6E63', baseImage: '' },
        fields: { damage: square.damage },
        onLand: [
          // Agility is the escape roll; defence soaks what still lands.
          action('if_stat_ge', {
            resource: 'agility',
            amount: 6,
            then: [action('log_message', { title: 'You leap clear of the trap.' })],
            else: [
              action('lose_resource', { resource: 'health', amountField: 'damage' }),
              action('log_message', { title: 'The trap catches you.' }),
            ],
          }),
          deathCheck(),
        ],
      };
    case 'shrine':
      return {
        ...base, title: square.title, type: 'shrine',
        visual: { baseColor: '#4DD0E1', baseImage: '' },
        fields: {},
        onLand: [
          action('offer_choice', {
            title: 'The shrine offers a blessing.',
            options: [
              { id: 'scout', title: 'Reveal the surrounding area', then: [action('reveal_cells', { amount: 2 })] },
              { id: 'heal', title: 'Restore 8 health', then: [action('gain_resource', { resource: 'health', amount: 8 })] },
              { id: 'insight', title: 'Gain 8 experience', then: [action('gain_resource', { resource: 'experience', amount: 8 })] },
            ],
          }),
        ],
      };
    case 'boss':
      return {
        ...base, title: square.title, type: 'boss',
        visual: { baseColor: '#4A148C', baseImage: '' },
        fields: { difficulty: 16 },
        onLand: [
          // The declared victory condition: kill the dragon.
          action('if_stat_ge', {
            resource: 'attack',
            amount: 16,
            then: [
              action('log_message', { title: 'The dragon falls. The dungeon is yours.' }),
              action('finish_game'),
            ],
            else: [
              action('log_message', { title: 'The dragon is far beyond you.' }),
              action('lose_resource', { resource: 'health', amount: 12 }),
              deathCheck(),
            ],
          }),
        ],
      };
    default:
      return {
        ...base, title: 'Empty Passage', type: 'empty',
        visual: { baseColor: '#ECEFF1', baseImage: '' },
        fields: {}, onLand: [],
      };
  }
}

/** Grid neighbours, both ways, so a free move can go in any direction. */
function gridEdges(): EdgeDefinition[] {
  const edges: EdgeDefinition[] = [];
  const idAt = (index: number) => (index === 0 ? 'camp' : `c${index}`);

  for (let index = 0; index < COLS * ROWS; index++) {
    const col = index % COLS;
    const row = Math.floor(index / COLS);
    const neighbours: number[] = [];
    if (col > 0) neighbours.push(index - 1);
    if (col < COLS - 1) neighbours.push(index + 1);
    if (row > 0) neighbours.push(index - COLS);
    if (row < ROWS - 1) neighbours.push(index + COLS);

    for (const neighbour of neighbours) {
      edges.push({
        id: `e_${index}_${neighbour}`,
        from: idAt(index),
        to: idAt(neighbour),
        condition: { type: 'always' },
      });
    }
  }
  return edges;
}

export function createRpgDemo(): GameDefinition {
  const cells = MAP.map(cellFor);

  return {
    id: '',
    title: 'Dungeon of the Sunken Keep',
    version: 1,
    board: {
      width: COLS * CELL,
      height: ROWS * CELL,
      cellSize: CELL,
      cells,
      edges: gridEdges(),
    },
    rules: {
      dice: { count: 1, sides: 4 },
      // Free movement over a grid: the roll says how far, the player says where.
      movement: 'free',
      hiddenCells: true,
      resources: {
        health: { initial: 20 },
        attack: { initial: 4 },
        defence: { initial: 2 },
        agility: { initial: 3 },
        experience: { initial: 0 },
        level: { initial: 1 },
        points: { initial: 0 },
        gold: { initial: 0 },
      },
      progression: {
        experienceResource: 'experience',
        levelResource: 'level',
        pointsResource: 'points',
        pointsPerLevel: 2,
        thresholds: [10, 25, 45, 70, 100, 140],
      },
      equipmentSlots: ['weapon', 'armour', 'offhand'],
      items: {
        rusty_sword: { id: 'rusty_sword', title: 'Rusty Sword', slot: 'weapon', bonuses: { attack: 2 } },
        steel_sword: { id: 'steel_sword', title: 'Steel Sword', slot: 'weapon', bonuses: { attack: 5 } },
        runed_blade: { id: 'runed_blade', title: 'Runed Blade', slot: 'weapon', bonuses: { attack: 8, agility: 1 } },
        leather_armour: { id: 'leather_armour', title: 'Leather Armour', slot: 'armour', bonuses: { defence: 3, health: 4 } },
        tower_shield: { id: 'tower_shield', title: 'Tower Shield', slot: 'offhand', bonuses: { defence: 4 } },
        potion: {
          id: 'potion', title: 'Healing Potion', consumable: true,
          use: [action('gain_resource', { resource: 'health', amount: 10 })],
        },
      },
      cellTypes: {
        start: { title: 'Camp', fields: {} },
        empty: { title: 'Passage', fields: {} },
        enemy: {
          title: 'Enemy',
          fields: {
            difficulty: { type: 'number', label: 'Difficulty', default: 6 },
            reward: { type: 'number', label: 'Experience', default: 10 },
          },
        },
        loot: { title: 'Loot', fields: {} },
        trap: { title: 'Trap', fields: { damage: { type: 'number', label: 'Damage', default: 4 } } },
        shrine: { title: 'Shrine', fields: {} },
        boss: { title: 'Boss', fields: { difficulty: { type: 'number', label: 'Difficulty', default: 16 } } },
      },
      startBonus: 0,
      startBonusResource: '',
    },
  };
}
