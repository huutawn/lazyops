import { describe, expect, it } from 'vitest';
import { normalizeAuthError, getAuthErrorMessage } from '@/lib/auth/auth-errors';
import { AUTH_ERROR_CODES } from '@/lib/auth/auth-types';

describe('normalizeAuthError', () => {
  it('handles Response errors', () => {
    const response = new Response(null, { status: 401 });
    const result = normalizeAuthError(response);
    expect(result.code).toBe(AUTH_ERROR_CODES.UNAUTHORIZED);
  });

  it('handles network errors', () => {
    const error = new Error('Failed to fetch');
    const result = normalizeAuthError(error);
    expect(result.code).toBe(AUTH_ERROR_CODES.NETWORK_ERROR);
  });

  it('handles expired token errors', () => {
    const error = new Error('Token expired');
    const result = normalizeAuthError(error);
    expect(result.code).toBe(AUTH_ERROR_CODES.EXPIRED_TOKEN);
  });

  it('handles generic errors', () => {
    const error = new Error('Something broke');
    const result = normalizeAuthError(error);
    expect(result.code).toBe(AUTH_ERROR_CODES.UNKNOWN);
    expect(result.message).toBe('Something broke');
  });

  it('handles object errors with code and message', () => {
    const error = { code: 'custom_code', message: 'Custom message' };
    const result = normalizeAuthError(error);
    expect(result.code).toBe('custom_code');
    expect(result.message).toBe('Custom message');
  });

  it('handles unknown error types', () => {
    const result = normalizeAuthError(null);
    expect(result.code).toBe(AUTH_ERROR_CODES.UNKNOWN);
  });
});

describe('getAuthErrorMessage', () => {
  it('returns message for known error codes', () => {
    expect(getAuthErrorMessage(AUTH_ERROR_CODES.INVALID_CREDENTIALS)).toBe('Invalid email or password.');
    expect(getAuthErrorMessage(AUTH_ERROR_CODES.USER_EXISTS)).toBe('An account with this email already exists.');
    expect(getAuthErrorMessage(AUTH_ERROR_CODES.WEAK_PASSWORD)).toContain('8-72 characters');
    expect(getAuthErrorMessage(AUTH_ERROR_CODES.UNAUTHORIZED)).toBe('You must be logged in to access this page.');
  });

  it('returns fallback message for unknown codes', () => {
    expect(getAuthErrorMessage('unknown_code')).toBe('An unexpected error occurred.');
  });
});
