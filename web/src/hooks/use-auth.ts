import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { login, register } from '@/api/auth';
import { useAuthStore } from '@/stores/auth-store';

/** TanStack Query mutation for login. On success, stores JWT and navigates to dashboard. */
export function useLogin() {
  const navigate = useNavigate();

  return useMutation({
    mutationFn: login,
    onSuccess: (data) => {
      useAuthStore.getState().setToken(data.token);
      navigate('/');
    },
  });
}

/** TanStack Query mutation for registration. On success, stores JWT and navigates to dashboard. */
export function useRegister() {
  const navigate = useNavigate();

  return useMutation({
    mutationFn: register,
    onSuccess: (data) => {
      useAuthStore.getState().setToken(data.token);
      navigate('/');
    },
  });
}

/** Returns a logout function that clears auth state and navigates to login. */
export function useLogout() {
  const navigate = useNavigate();

  return () => {
    useAuthStore.getState().logout();
    navigate('/login');
  };
}
