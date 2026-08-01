import type { GameEvent } from './types';

export type RollPlayback = {
  rolls: number[];
  total: number;
  playerId: string;
  positions: string[];
};

export function confirmedRollPlayback(events: GameEvent[]): RollPlayback | null {
  const diceEvent = events.find((event) => event.type === 'dice_roll');
  const moveEvent = events.find((event) => event.type === 'move');
  const dice = diceEvent?.payload;
  const move = moveEvent?.payload;

  if (!dice || !move || !Array.isArray(dice.rolls) || dice.rolls.length === 0 || !dice.rolls.every((roll: unknown) => typeof roll === 'number') ||
    typeof dice.total !== 'number' || typeof dice.playerId !== 'string' || typeof move.playerId !== 'string' ||
    dice.playerId !== move.playerId || typeof move.from !== 'string' || !Array.isArray(move.path) ||
    !move.path.every((cellID: unknown) => typeof cellID === 'string')) {
    return null;
  }

  return { rolls: dice.rolls, total: dice.total, playerId: dice.playerId, positions: [move.from, ...move.path] };
}
