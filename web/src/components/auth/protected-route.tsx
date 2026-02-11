import { Navigate, Outlet } from 'react-router';
import { useAuthStore } from '@/stores/auth-store';

/** Route guard: redirects to /login if no JWT token is present. */
export function ProtectedRoute() {
  const token = useAuthStore((s) => s.token);
  if (!token) return <Navigate to="/login" replace />;
  return <Outlet />;
}

/** Route guard: redirects to / if user is not an admin. */
export function AdminRoute() {
  const user = useAuthStore((s) => s.user);
  if (user?.role !== 'admin') return <Navigate to="/" replace />;
  return <Outlet />;
}
