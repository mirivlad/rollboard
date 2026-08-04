import type { ActionDefinition } from './types';

/**
 * What every action type looks like, so the editor can be generated instead of
 * hand-written.
 *
 * The visual editor used to know seven action types while the engine executed
 * twenty-seven, which meant everything added for the property and dungeon
 * templates could only be reached by writing a definition in code. That is the
 * opposite of what this editor is for.
 *
 * Adding an action type to the engine now means adding one entry here, and the
 * editor picks it up with no further work. A test asserts the two lists agree.
 */

export type FieldKind =
  | 'resource'   // a stat or currency declared in rules.resources
  | 'item'       // an item declared in rules.items
  | 'slot'       // an equipment slot declared in rules.equipmentSlots
  | 'cell'       // another cell on the board
  | 'number'
  | 'text'
  | 'payee'      // who a payment reaches — only the recipients the engine resolves
  | 'cellOwner'  // who a square belongs to
  | 'boolean'    // stored as the string "true" or "false"
  | 'auctionAudience' // everyone at the table, or everyone but the acting player
  | 'formula'    // a computed amount
  | 'query'      // which other cells this action asks about
  | 'actions'    // a nested list, for then/else
  | 'options';   // a nested list of choices

export interface ActionField {
  /** The property on ActionDefinition this control writes to. */
  name: keyof ActionDefinition;
  kind: FieldKind;
  labelKey: string;
  /** Shown when the author has not filled it in yet. */
  required?: boolean;
}

export interface ActionSchema {
  type: string;
  labelKey: string;
  /** Groups the picker, so thirty entries stay navigable. */
  group: 'resources' | 'items' | 'cell' | 'player' | 'flow' | 'board';
  fields: ActionField[];
}

const amount: ActionField = { name: 'amount', kind: 'number', labelKey: 'inspector.amount' };
const formula: ActionField = { name: 'formula', kind: 'formula', labelKey: 'inspector.formula' };
const resource: ActionField = { name: 'resource', kind: 'resource', labelKey: 'inspector.resource', required: true };
const branches: ActionField[] = [
  { name: 'then', kind: 'actions', labelKey: 'inspector.then' },
  { name: 'else', kind: 'actions', labelKey: 'inspector.else' },
];

export const ACTION_SCHEMAS: ActionSchema[] = [
  // --- resources -----------------------------------------------------------
  { type: 'gain_resource', labelKey: 'action.gain_resource', group: 'resources', fields: [resource, amount, formula] },
  { type: 'lose_resource', labelKey: 'action.lose_resource', group: 'resources', fields: [resource, amount, formula] },
  {
    type: 'transfer_resource', labelKey: 'action.transfer_resource', group: 'resources',
    fields: [resource, amount, formula, { name: 'target', kind: 'payee', labelKey: 'inspector.target', required: true }],
  },
  { type: 'if_resource_ge', labelKey: 'action.if_resource_ge', group: 'resources', fields: [resource, amount, formula, ...branches] },
  { type: 'if_stat_ge', labelKey: 'action.if_stat_ge', group: 'resources', fields: [resource, amount, formula, ...branches] },

  // --- items ---------------------------------------------------------------
  {
    type: 'grant_item', labelKey: 'action.grant_item', group: 'items',
    fields: [{ name: 'field', kind: 'item', labelKey: 'inspector.item', required: true }, amount, formula],
  },
  {
    type: 'remove_item', labelKey: 'action.remove_item', group: 'items',
    fields: [{ name: 'field', kind: 'item', labelKey: 'inspector.item', required: true }, amount, formula],
  },
  {
    type: 'equip_item', labelKey: 'action.equip_item', group: 'items',
    fields: [{ name: 'field', kind: 'item', labelKey: 'inspector.item', required: true }],
  },
  {
    type: 'use_item', labelKey: 'action.use_item', group: 'items',
    fields: [{ name: 'field', kind: 'item', labelKey: 'inspector.item', required: true }],
  },
  {
    type: 'unequip_slot', labelKey: 'action.unequip_slot', group: 'items',
    fields: [{ name: 'target', kind: 'slot', labelKey: 'inspector.slot', required: true }],
  },
  {
    type: 'if_has_item', labelKey: 'action.if_has_item', group: 'items',
    fields: [{ name: 'field', kind: 'item', labelKey: 'inspector.item', required: true }, amount, formula, ...branches],
  },

  // --- the cell being landed on -------------------------------------------
  {
    type: 'set_cell_owner', labelKey: 'action.set_cell_owner', group: 'cell',
    fields: [{ name: 'target', kind: 'cellOwner', labelKey: 'inspector.owner', required: true }],
  },
  { type: 'set_cell_level', labelKey: 'action.set_cell_level', group: 'cell', fields: [amount, formula] },
  {
    type: 'set_cell_mortgaged', labelKey: 'action.set_cell_mortgaged', group: 'cell',
    fields: [{ name: 'target', kind: 'boolean', labelKey: 'inspector.mortgaged', required: true }],
  },
  { type: 'if_cell_unowned', labelKey: 'action.if_cell_unowned', group: 'cell', fields: branches },
  { type: 'if_cell_owned_by_current', labelKey: 'action.if_cell_owned_by_current', group: 'cell', fields: branches },
  { type: 'if_cell_owned_by_other', labelKey: 'action.if_cell_owned_by_other', group: 'cell', fields: branches },
  { type: 'if_cell_level_ge', labelKey: 'action.if_cell_level_ge', group: 'cell', fields: [amount, formula, ...branches] },
  { type: 'if_cell_mortgaged', labelKey: 'action.if_cell_mortgaged', group: 'cell', fields: branches },

  // --- the player ----------------------------------------------------------
  {
    type: 'move_player_to', labelKey: 'action.move_player_to', group: 'player',
    fields: [{ name: 'to', kind: 'cell', labelKey: 'inspector.destination', required: true }],
  },
  { type: 'skip_turns', labelKey: 'action.skip_turns', group: 'player', fields: [amount, formula] },
  { type: 'eliminate_player', labelKey: 'action.eliminate_player', group: 'player', fields: [] },

  // --- the board -----------------------------------------------------------
  {
    type: 'reveal_cells', labelKey: 'action.reveal_cells', group: 'board',
    fields: [amount, formula, { name: 'to', kind: 'cell', labelKey: 'inspector.destination' }],
  },
  {
    type: 'if_cells_ge', labelKey: 'action.if_cells_ge', group: 'board',
    fields: [{ name: 'query', kind: 'query', labelKey: 'inspector.query', required: true }, amount, formula, ...branches],
  },
  {
    type: 'for_each_cell', labelKey: 'action.for_each_cell', group: 'board',
    fields: [{ name: 'query', kind: 'query', labelKey: 'inspector.query', required: true }, ...branches],
  },

  // --- flow ----------------------------------------------------------------
  {
    type: 'offer_choice', labelKey: 'action.offer_choice', group: 'flow',
    fields: [
      { name: 'title', kind: 'text', labelKey: 'inspector.title', required: true },
      { name: 'options', kind: 'options', labelKey: 'inspector.choices' },
    ],
  },
  {
    type: 'random_branch', labelKey: 'action.random_branch', group: 'flow',
    fields: [{ name: 'options', kind: 'options', labelKey: 'inspector.outcomes' }],
  },
  {
    // then is what the winner gets, so it is labelled as such rather than as
    // a branch: an auction that awards nothing is refused at publication.
    type: 'start_auction', labelKey: 'action.start_auction', group: 'flow',
    fields: [
      { name: 'resource', kind: 'resource', labelKey: 'inspector.bidCurrency', required: true },
      amount,
      formula,
      { name: 'increment', kind: 'number', labelKey: 'inspector.increment' },
      { name: 'target', kind: 'auctionAudience', labelKey: 'inspector.bidders' },
      { name: 'title', kind: 'text', labelKey: 'inspector.title' },
      { name: 'then', kind: 'actions', labelKey: 'inspector.winnerGets' },
      { name: 'else', kind: 'actions', labelKey: 'inspector.nobodyBids' },
    ],
  },
  { type: 'finish_game', labelKey: 'action.finish_game', group: 'flow', fields: [] },
  {
    type: 'log_message', labelKey: 'action.log_message', group: 'flow',
    fields: [{ name: 'title', kind: 'text', labelKey: 'inspector.message', required: true }],
  },
];

export const ACTION_GROUPS = ['resources', 'items', 'cell', 'player', 'board', 'flow'] as const;

export function schemaFor(type: string): ActionSchema | undefined {
  return ACTION_SCHEMAS.find((schema) => schema.type === type);
}

/**
 * The next free option ID for a list.
 *
 * Naming an option after the length of the list produced a duplicate as soon
 * as one was removed and another added: option_1, option_2, option_3, delete
 * the second, add one — and the new option is option_3 as well. The engine
 * resolves the first match, so the second one could never be chosen and
 * nothing said why.
 */
export function nextOptionId(options: { id?: string }[]): string {
  const used = new Set(options.map((option) => option.id));
  for (let n = 1; ; n += 1) {
    const candidate = `option_${n}`;
    if (!used.has(candidate)) return candidate;
  }
}

/** A new action of this type, with only the fields the schema declares. */
export function blankAction(type: string): ActionDefinition {
  const schema = schemaFor(type);
  const action: ActionDefinition = { type };
  for (const field of schema?.fields ?? []) {
    if (field.kind === 'actions') (action as unknown as Record<string, unknown>)[field.name] = [];
    if (field.kind === 'options') (action as unknown as Record<string, unknown>)[field.name] = [];
    // An empty query means "every cell on the board", which is a working
    // starting point the author narrows down rather than an error.
    if (field.kind === 'query') (action as unknown as Record<string, unknown>)[field.name] = {};
  }
  return action;
}
