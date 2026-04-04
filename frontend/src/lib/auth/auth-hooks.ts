import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import type { UserSession } from '@/lib/auth/auth-types';
import { normalizeAuthError } from '@/lib/auth/auth-errors';

const SESSION_QUERY_KEY = ['auth', 'session'];

export function useSession() {
  return useQuery({
    queryKey: SESSION_QUERY_KEY,
    queryFn: async () => {
      const response = await fetch('/api/auth/me');
      if (!response.ok) {
        const error = await response.json().catch(() => null);
        throw new Error(error?.error?.message ?? 'Session check failed');
      }
      const data = await response.json();
      return data.user as UserSession;
    },
    retry: false,
    staleTime: 60 * 1000,
  });
}

export function useLogin() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (credentials: { email: string; password: string; redirect?: string }) => {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: credentials.email, password: credentials.password }),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => null);
        throw normalizeAuthError(error?.error ?? new Error('Login failed'));
      }

      return response.json() as Promise<{ user: UserSession }>;
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: SESSION_QUERY_KEY });
      const target = variables.redirect ?? '/dashboard';
      router.push(target);
      router.refresh();
    },
  });
}

export function useRegister() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (credentials: { name: string; email: string; password: string }) => {
      const response = await fetch('/api/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(credentials),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => null);
        throw normalizeAuthError(error?.error ?? new Error('Registration failed'));
      }

      return response.json() as Promise<{ user: UserSession }>;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: SESSION_QUERY_KEY });
      router.push('/dashboard');
      router.refresh();
    },
  });
}

export function useLogout() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const response = await fetch('/api/auth/logout', { method: 'POST' });
      if (!response.ok) {
        throw new Error('Logout failed');
      }
      return response.json();
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: SESSION_QUERY_KEY });
      void queryClient.clear();
      router.push('/login');
      router.refresh();
    },
  });
}
