import { NextRequest } from 'next/server';
import { API_BASE_URL, SESSION_COOKIE_NAME, isSecureRequest, sessionCookieOptions } from '@/lib/auth/auth-config';
import { redirectRelative } from '@/lib/auth/redirect';
import type { AuthTokens } from '@/lib/auth/auth-types';

const OAUTH_NEXT_COOKIE = 'lazyops_oauth_next';

export async function GET(request: NextRequest) {
  const { searchParams } = request.nextUrl;
  const code = searchParams.get('code');
  const state = searchParams.get('state');
  const error = searchParams.get('error');

  if (error) {
    return redirectRelative(`/login?error=oauth_denied&error_description=${encodeURIComponent(error)}`);
  }

  if (!code || !state) {
    return redirectRelative('/login?error=oauth_missing_params');
  }

  try {
    const callbackURL = new URL(`${API_BASE_URL}/api/v1/auth/oauth/google/callback`);
    callbackURL.searchParams.set('code', code);
    callbackURL.searchParams.set('state', state);
    callbackURL.searchParams.set('mode', 'json');

    const stateNonce = request.cookies.get('lazyops_oauth_google_state')?.value;
    const headers: HeadersInit = {};
    if (stateNonce) {
      headers['X-LazyOps-OAuth-State-Nonce'] = stateNonce;
    }

    const response = await fetch(callbackURL, { headers });

    if (!response.ok) {
      const errorBody = await response.json().catch(() => null);
      const errorCode = errorBody?.code ?? errorBody?.message ?? 'oauth_failed';
      return redirectRelative(`/login?error=${encodeURIComponent(errorCode)}`);
    }

    const payload = await response.json();
    const data = (payload?.data ?? payload) as AuthTokens;
    if (!data?.access_token) {
      return redirectRelative('/login?error=oauth_invalid_response');
    }
    const cookieOpts = sessionCookieOptions(isSecureRequest(request));
    const nextPathRaw = request.cookies.get(OAUTH_NEXT_COOKIE)?.value ?? '';
    const nextPath = nextPathRaw.startsWith('/') ? nextPathRaw : '/dashboard';

    const redirectResponse = redirectRelative(nextPath);
    redirectResponse.cookies.set(SESSION_COOKIE_NAME, data.access_token, cookieOpts);
    redirectResponse.cookies.set(OAUTH_NEXT_COOKIE, '', {
      path: '/',
      maxAge: 0,
      httpOnly: true,
      sameSite: 'lax',
      secure: isSecureRequest(request),
    });

    return redirectResponse;
  } catch {
    return redirectRelative('/login?error=oauth_network');
  }
}
