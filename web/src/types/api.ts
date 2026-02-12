// TypeScript types matching Go API response shapes exactly.
// Source: internal/api/handlers_gameserver.go, handlers_games.go, handlers_auth.go, request.go

/** Matches internal/api/handlers_gameserver.go GameServerResponse */
export interface GameServerResponse {
  name: string;
  gameType: string;
  state: 'Creating' | 'Starting' | 'Ready' | 'Allocated' | 'Shutdown' | 'Error';
  address?: string;
  ports?: PortResponse[];
  parameters?: Record<string, string>;
  createdAt: string;
}

/** Matches internal/api/handlers_gameserver.go PortResponse */
export interface PortResponse {
  name: string;
  port: number;
  protocol: string;
}

/** Matches internal/api/handlers_games.go GameResponse */
export interface GameResponse {
  name: string;
  displayName: string;
  image: string;
  ports: PortInfo[];
  parameters: Record<string, string>;
  parameterSchema?: Record<string, unknown>;
}

/** Matches internal/api/handlers_games.go PortInfo in GameResponse */
export interface PortInfo {
  name: string;
  containerPort: number;
  protocol: string;
}

/** API list wrapper: { data: T[], count: number } */
export interface ListResponse<T> {
  data: T[];
  count: number;
}

/** API error response: { error: string, details?: string } */
export interface ErrorResponse {
  error: string;
  details?: string;
}

/** Matches internal/api/request.go CreateGameServerRequest */
export interface CreateGameServerRequest {
  name: string;
  gameType: string;
  parameters?: Record<string, string>;
}

/** Matches internal/api/request.go UpdateGameServerRequest */
export interface UpdateGameServerRequest {
  parameters: Record<string, string>;
}

/** Matches internal/api/request.go LoginRequest */
export interface LoginRequest {
  username: string;
  password: string;
}

/** Matches internal/api/handlers_auth.go loginResponse */
export interface LoginResponse {
  token: string;
}

/** Matches internal/api/request.go RegisterRequest */
export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  inviteToken: string;
}

/** Matches internal/api/handlers_auth.go registerResponse */
export interface RegisterResponse {
  token: string;
  username: string;
}

/** Matches internal/api/handlers_admin.go user list item */
export interface UserResponse {
  username: string;
  email: string;
  role: string;
  createdAt: string;
}

/** Matches internal/api/request.go CreateInviteRequest */
export interface InviteRequest {
  email: string;
}

/** Matches internal/api/handlers_admin.go invite response */
export interface InviteResponse {
  token: string;
  email: string;
  expiresAt: string;
}

/** Matches internal/api/handlers_metrics.go MetricsResponse */
export interface MetricsResponse {
  cpu: number;          // millicores
  memoryMiB: number;    // MiB
  cpuLimit: number;     // millicores from spec
  memoryLimitMiB: number; // MiB from spec
}
