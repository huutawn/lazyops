import { setupWorker } from 'msw/browser';
import { mockHandlers } from '@/lib/mocks/handlers';

export const worker = setupWorker(...mockHandlers);

export async function startMockService() {
  try {
    await worker.start({
      onUnhandledRequest: 'bypass',
    });
    console.log('[MSW] Mock service worker started');
  } catch (error) {
    console.error('[MSW] Failed to start mock service worker:', error);
  }
}
