import { NextRequest, NextResponse } from 'next/server';
import { API_BASE_URL, SESSION_COOKIE_NAME, isSecureRequest, sessionCookieOptions } from '@/lib/auth/auth-config';
import { getAuthErrorMessage } from '@/lib/auth/auth-errors';
import type { LoginCredentials, AuthTokens } from '@/lib/auth/auth-types';

export async function POST(request: NextRequest) {
  try {
    const body: LoginCredentials = await request.json();

    const response = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    const payload = await response.json().catch(() => null);

    if (!response.ok) {
      return NextResponse.json(
        {
          error: {
            code: payload?.error?.code ?? 'login_failed',
            message: payload?.message ?? 'Login failed',
          },
        },
        { status: response.status },
      );
    }

    const authData = (payload?.data ?? payload) as AuthTokens | null;
    if (!authData?.access_token || !authData?.user) {
      return NextResponse.json(
        { error: { code: 'invalid_auth_payload', message: 'Login failed' } },
        { status: 502 },
      );
    }
    const cookieOpts = sessionCookieOptions(isSecureRequest(request));

    const nextResponse = NextResponse.json({ user: authData.user });
    nextResponse.cookies.set(SESSION_COOKIE_NAME, authData.access_token, cookieOpts);

    return nextResponse;
  } catch {
    return NextResponse.json(
      { error: { code: 'login_failed', message: getAuthErrorMessage('network_error') } },
      { status: 500 },
    );
  }
}
