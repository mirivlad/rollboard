// Inventory and trading reached online rooms after the rest: the protocol
// carried only start, roll, action and chat, so a player given an item by a
// cell had no way to put it on.
type CommandType = 'start' | 'roll' | 'action' | 'chat' | 'inventory' | 'trade';

export function roomCommand<T extends Record<string, unknown> = Record<string, never>>(type: CommandType, fields?: T): { type: CommandType; commandId: string } & T {
  return { type, commandId: crypto.randomUUID(), ...(fields ?? {}) } as { type: CommandType; commandId: string } & T;
}
