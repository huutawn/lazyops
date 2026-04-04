'use client';

import type { ReactNode } from 'react';
import { Providers } from '@/lib/providers/providers';
import { MockProvider } from '@/lib/mocks/mock-provider';

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <Providers>
      <MockProvider>{children}</MockProvider>
    </Providers>
  );
}
