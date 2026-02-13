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

/**
 * Upload wrapper for multipart file uploads with progress tracking.
 *
 * Uses XMLHttpRequest instead of fetch because the fetch API does not
 * support upload progress events. Does NOT set Content-Type header --
 * the browser auto-sets multipart/form-data with boundary when body is FormData.
 */
export function apiUpload<T>(
  path: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<T> {
  const token = useAuthStore.getState().token;

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${API_BASE}${path}`);
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    // DO NOT set Content-Type -- browser sets it with multipart boundary

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          onProgress(Math.round((e.loaded / e.total) * 100));
        }
      };
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText));
      } else {
        try {
          const err = JSON.parse(xhr.responseText);
          reject(new ApiError(xhr.status, err.error || 'Upload failed'));
        } catch {
          reject(new ApiError(xhr.status, xhr.statusText || 'Upload failed'));
        }
      }
    };

    xhr.onerror = () => reject(new ApiError(0, 'Network error during upload'));

    const formData = new FormData();
    formData.append('file', file);
    xhr.send(formData);
  });
}
