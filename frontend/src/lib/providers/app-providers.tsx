'use client';

import type { ReactNode } from 'react';
import { Providers } from '@/lib/providers/providers';
import { MockProvider } from '@/lib/mocks/mock-provider';
import { AssistantProvider } from '@/modules/assistant/assistant-provider';

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <Providers>
      <MockProvider>
        <AssistantProvider>{children}</AssistantProvider>
      </MockProvider>
    </Providers>
  );
}
