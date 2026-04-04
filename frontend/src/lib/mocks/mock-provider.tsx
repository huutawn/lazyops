'use client';

import { useEffect, useState } from 'react';

type MockProviderProps = {
  children: React.ReactNode;
};

export function MockProvider({ children }: MockProviderProps) {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const enabled = process.env.NEXT_PUBLIC_MOCK_MODE === 'true';
    if (!enabled) {
      setReady(true);
      return;
    }

    let cancelled = false;
    import('@/lib/mocks/browser').then(({ startMockService }) => {
      if (!cancelled) {
        startMockService().then(() => setReady(true));
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  if (!ready) return null;

  return <>{children}</>;
}
