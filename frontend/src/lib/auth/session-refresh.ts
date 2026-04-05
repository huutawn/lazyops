'use client';

import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';

const SESSION_QUERY_KEY = ['auth', 'session'];
const REFRESH_INTERVAL_MS = 15 * 60 * 1000;

export function useSessionRefresh() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    intervalRef.current = setInterval(async () => {
      try {
        const response = await fetch('/api/auth/me');
        if (!response.ok) {
          queryClient.setQueryData(SESSION_QUERY_KEY, null);
          router.push('/login');
          return;
        }
        const data = await response.json();
        queryClient.setQueryData(SESSION_QUERY_KEY, data.user);
      } catch {
        queryClient.setQueryData(SESSION_QUERY_KEY, null);
        router.push('/login');
      }
    }, REFRESH_INTERVAL_MS);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [queryClient, router]);
}
