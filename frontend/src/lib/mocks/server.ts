import { setupServer } from 'msw/node';
import { mockHandlers } from '@/lib/mocks/handlers';

export const mockServer = setupServer(...mockHandlers);
