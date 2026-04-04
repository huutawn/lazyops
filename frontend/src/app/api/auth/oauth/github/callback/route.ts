import { NextRequest, NextResponse } from 'next/server';
import { API_BASE_URL, SESSION_COOKIE_NAME, sessionCookieOptions } from '@/lib/auth/auth-config';
import type { AuthTokens } from '@/lib/auth/auth-types';

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const code = searchParams.get('code');
  const state = searchParams.get('state');
  const error = searchParams.get('error');

  if (error) {
    return NextResponse.redirect(
      new URL(`/login?error=oauth_denied&error_description=${encodeURIComponent(error)}`, request.url),
    );
  }

  if (!code || !state) {
    return NextResponse.redirect(new URL('/login?error=oauth_missing_params', request.url));
  }

  try {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/auth/oauth/github/callback?code=${code}&state=${state}`,
    );

    if (!response.ok) {
      const errorBody = await response.json().catch(() => null);
      const errorMsg = errorBody?.message ?? 'oauth_failed';
      return NextResponse.redirect(
        new URL(`/login?error=${encodeURIComponent(errorMsg)}`, request.url),
      );
    }

    const data = (await response.json()) as AuthTokens;
    const isSecure = API_BASE_URL.startsWith('https');
    const cookieOpts = sessionCookieOptions(isSecure);

    const redirectResponse = NextResponse.redirect(new URL('/dashboard', request.url));
    redirectResponse.cookies.set(SESSION_COOKIE_NAME, data.access_token, cookieOpts);

    return redirectResponse;
  } catch {
    return NextResponse.redirect(new URL('/login?error=oauth_network', request.url));
  }
}
