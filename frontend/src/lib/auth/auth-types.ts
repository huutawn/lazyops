export type UserSession = {
  id: string;
  display_name: string;
  email: string;
  role: string;
  status: string;
};

export type AuthTokens = {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: UserSession;
};

export type LoginCredentials = {
  email: string;
  password: string;
};

export type RegisterCredentials = {
  name: string;
  email: string;
  password: string;
  role?: string;
};

export type SessionCookie = {
  name: string;
  value: string;
  secure: boolean;
};

export const SESSION_COOKIE_NAME = 'lazyops_session';

export type AuthError = {
  code: string;
  message: string;
};

export const AUTH_ERROR_CODES = {
  INVALID_CREDENTIALS: 'invalid_credentials',
  USER_EXISTS: 'user_exists',
  WEAK_PASSWORD: 'weak_password',
  EXPIRED_TOKEN: 'expired_token',
  REVOKED_TOKEN: 'revoked_token',
  ACCOUNT_DISABLED: 'account_disabled',
  UNAUTHORIZED: 'unauthorized',
  NETWORK_ERROR: 'network_error',
  UNKNOWN: 'unknown',
} as const;
