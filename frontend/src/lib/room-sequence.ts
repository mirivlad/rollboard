export function acceptsRoomSequence(latest: number, incoming: unknown): incoming is number {
  return typeof incoming === 'number' && Number.isSafeInteger(incoming) && incoming > latest;
}
