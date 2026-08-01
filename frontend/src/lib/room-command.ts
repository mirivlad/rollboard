type CommandType = 'start' | 'roll' | 'action' | 'chat';

export function roomCommand<T extends Record<string, string> = Record<string, never>>(type: CommandType, fields?: T): { type: CommandType; commandId: string } & T {
  return { type, commandId: crypto.randomUUID(), ...(fields ?? {}) } as { type: CommandType; commandId: string } & T;
}
