export { useSession, useLogin, useRegister, useLogout } from './auth-hooks';
export { AuthGuard } from './auth-guard';
export { normalizeAuthError, getAuthErrorMessage } from './auth-errors';
export { SESSION_COOKIE_NAME } from './auth-config';
export type {
  UserSession,
  AuthTokens,
  LoginCredentials,
  RegisterCredentials,
  AuthError,
} from './auth-types';
export { AUTH_ERROR_CODES } from './auth-types';
