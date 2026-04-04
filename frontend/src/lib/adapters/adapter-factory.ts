import type { ApiResponse } from '@/lib/types';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';

export type Adapter<T> = {
  fetch: () => Promise<ApiResponse<T>>;
  mode: 'live' | 'mock';
};

export function createAdapter<T>(liveFn: () => Promise<ApiResponse<T>>, mockFn: () => Promise<ApiResponse<T>>): Adapter<T> {
  const mode = isFeatureEnabled('mock_mode') ? 'mock' : 'live';

  return {
    mode,
    fetch: mode === 'mock' ? mockFn : liveFn,
  };
}
