import { create } from 'zustand';

interface AuthUser {
  username: string;
  email: string;
  role: string;
}

interface AuthState {
  token: string | null;
  user: AuthUser | null;
  setToken: (token: string) => void;
  logout: () => void;
}

/**
 * Zustand auth store managing JWT token and decoded user claims.
 *
 * - setToken decodes the JWT payload (base64 middle segment) to extract user claims.
 * - Token is stored in memory only (not localStorage) per security best practices.
 * - Token is lost on page refresh; user re-authenticates.
 */
export const useAuthStore = create<AuthState>((set) => ({
  token: null,
  user: null,
  setToken: (token) => {
    // Decode JWT payload (base64url middle segment) to extract claims
    const payload = JSON.parse(atob(token.split('.')[1]));
    set({
      token,
      user: {
        username: payload.username,
        email: payload.email,
        role: payload.role,
      },
    });
  },
  logout: () => set({ token: null, user: null }),
}));
