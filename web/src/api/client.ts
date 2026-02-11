import { useAuthStore } from '@/stores/auth-store';

export const API_BASE = '/api/v1';

/** Custom error class for API responses with HTTP status codes. */
export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/**
 * Fetch wrapper with JWT auth header injection and token refresh handling.
 *
 * - Injects Authorization: Bearer <token> when a token exists in the auth store.
 * - Checks for X-Refresh-Token response header and updates the store if present.
 * - Throws ApiError on non-ok responses.
 * - Returns undefined as T for 204 No Content responses.
 */
export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const token = useAuthStore.getState().token;

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  // Auto-refresh: if server sends a refreshed token, update store
  const refreshToken = res.headers.get('X-Refresh-Token');
  if (refreshToken) {
    useAuthStore.getState().setToken(refreshToken);
  }

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, error.error || 'Request failed');
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}
