export interface GameDefinition {
  id: string;
  title: string;
  version: number;
  board: Board;
  rules: RuleSet;
}

export interface Board {
  width: number;
  height: number;
  cellSize: number;
  cells: CellDefinition[];
  edges: EdgeDefinition[];
}

export interface CellDefinition {
  id: string;
  title: string;
  type: string;
  x: number;
  y: number;
  visual: CellVisual;
  fields: Record<string, any>;
  onLand?: ActionDefinition[];
  onPass?: ActionDefinition[];
}

export interface CellVisual {
  baseColor: string;
  baseImage: string;
}

export interface EdgeDefinition {
  id: string;
  from: string;
  to: string;
  condition: EdgeCondition;
}

export interface EdgeCondition {
  type: string;
  values?: number[];
  resource?: string;
  amount?: number;
  label?: string;
  dice_count?: number;
  even?: boolean;
  odd?: boolean;
}

export interface RuleSet {
  dice: DiceRule;
  resources: Record<string, ResourceRule>;
  cellTypes: Record<string, CellTypeDefinition>;
  startBonus: number;
  startBonusResource?: string;
}

export interface DiceRule {
  count: number;
  sides: number;
}

export interface ResourceRule {
  initial: number;
  min?: number;
  max?: number;
}

export interface CellTypeDefinition {
  title: string;
  fields: Record<string, FieldDefinition>;
}

export interface FieldDefinition {
  type: 'string' | 'number' | 'boolean' | 'select';
  label: string;
  default?: any;
  options?: string[];
}

export interface ActionDefinition {
  type: string;
  resource?: string;
  amount?: number;
  amountField?: string;
  target?: string;
  to?: string;
  field?: string;
  title?: string;
  actionId?: string;
  miniGame?: MiniGameReference;
  then?: ActionDefinition[];
  else?: ActionDefinition[];
  options?: ActionOption[];
}

export interface ActionOption {
  id: string;
  title: string;
  then?: ActionDefinition[];
}

export interface MiniGameReference {
  moduleId: string;
  version: number;
  input?: Record<string, any>;
}

export interface PlayerConfig {
  name: string;
  color: string;
}

export interface GameSummary {
  id: string;
  title: string;
  version: number;
  updatedAt: string;
}

export interface PublicUser {
  id: string;
  email: string;
  displayName: string;
  createdAt: string;
}

export interface PublicGuest {
  id: string;
  displayName: string;
}

export type Principal =
  | { kind: 'user'; user: PublicUser }
  | { kind: 'guest'; guest: PublicGuest };

export interface CatalogGame {
  id: string;
  title: string;
  ownerUserId: string;
  createdAt: string;
  updatedAt: string;
}

export interface GameVersion {
  id: string;
  gameId: string;
  versionNumber: number;
  definition: GameDefinition;
  publishedAt: string;
}

export interface RoomMember {
  id: string;
  roomId: string;
  actorKind: 'user' | 'guest';
  actorId: string;
  playerId?: string;
  displayName: string;
  mutedAt?: string;
  joinedAt: string;
}

export interface Room {
  id: string;
  gameVersionId: string;
  hostUserId: string;
  hostMemberId: string;
  title: string;
  maxPlayers: number;
  status: 'lobby' | 'active' | 'finished';
  sequence: number;
  session?: GameSession;
  members: RoomMember[];
}

export interface RoomMessage {
  id: string;
  roomId: string;
  memberId: string;
  displayName: string;
  body: string;
  createdAt: string;
  sequence: number;
}

export interface GameSession {
  id: string;
  gameId: string;
  gameVersion: number;
  mode: string;
  definition: GameDefinition;
  state: GameState;
}

export interface GameState {
  currentPlayerIndex: number;
  players: PlayerState[];
  cellStates: Record<string, CellState>;
  turnNumber: number;
  roundNumber: number;
  status: 'active' | 'finished';
  winnerPlayerId?: string;
  log: GameEvent[];
  pendingAction?: PendingAction;
}

export interface PlayerState {
  id: string;
  name: string;
  color: string;
  positionCellId: string;
  resources: Record<string, number>;
  bankrupt: boolean;
}

export interface CellState {
  ownerPlayerId?: string;
  mortgaged?: boolean;
  level?: number;
}

export interface GameEvent {
  id: string;
  type: string;
  message: string;
  createdAt: string;
  payload?: any;
}

export interface PendingAction {
  type: string;
  playerId: string;
  title?: string;
  cellId?: string;
  options?: ActionOption[];
}

/** What somebody holding an invite link may see before joining. */
export interface RoomInvite {
  roomId: string;
  title: string;
  gameTitle: string;
  status: string;
  memberCount: number;
  maxPlayers: number;
  joinable: boolean;
}
