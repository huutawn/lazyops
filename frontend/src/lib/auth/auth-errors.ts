import { AUTH_ERROR_CODES, type AuthError } from '@/lib/auth/auth-types';

export function normalizeAuthError(error: unknown): AuthError {
  if (error instanceof Response) {
    return {
      code: error.status === 401 ? AUTH_ERROR_CODES.UNAUTHORIZED : AUTH_ERROR_CODES.UNKNOWN,
      message: `Request failed with status ${error.status}`,
    };
  }

  if (error instanceof Error) {
    if (error.message.includes('Failed to fetch') || error.message.includes('NetworkError')) {
      return {
        code: AUTH_ERROR_CODES.NETWORK_ERROR,
        message: 'Unable to connect to the server. Please check your connection.',
      };
    }

    if (error.message.includes('expired')) {
      return {
        code: AUTH_ERROR_CODES.EXPIRED_TOKEN,
        message: 'Your session has expired. Please log in again.',
      };
    }

    return {
      code: AUTH_ERROR_CODES.UNKNOWN,
      message: error.message,
    };
  }

  if (typeof error === 'object' && error !== null && 'code' in error && 'message' in error) {
    return {
      code: (error as { code: string }).code,
      message: (error as { message: string }).message,
    };
  }

  return {
    code: AUTH_ERROR_CODES.UNKNOWN,
    message: 'An unexpected error occurred.',
  };
}

export function getAuthErrorMessage(code: string): string {
  const messages: Record<string, string> = {
    [AUTH_ERROR_CODES.INVALID_CREDENTIALS]: 'Invalid email or password.',
    [AUTH_ERROR_CODES.USER_EXISTS]: 'An account with this email already exists.',
    [AUTH_ERROR_CODES.WEAK_PASSWORD]:
      'Password must be 8-72 characters with at least one uppercase, one lowercase, and one digit.',
    [AUTH_ERROR_CODES.EXPIRED_TOKEN]: 'Your session has expired. Please log in again.',
    [AUTH_ERROR_CODES.REVOKED_TOKEN]: 'Your session was revoked. Please log in again.',
    [AUTH_ERROR_CODES.ACCOUNT_DISABLED]: 'Your account has been disabled. Contact support.',
    [AUTH_ERROR_CODES.UNAUTHORIZED]: 'You must be logged in to access this page.',
    [AUTH_ERROR_CODES.NETWORK_ERROR]: 'Unable to connect to the server. Please check your connection.',
  };
  return messages[code] ?? 'An unexpected error occurred.';
}
