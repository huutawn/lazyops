import { NextRequest, NextResponse } from 'next/server';
import { API_BASE_URL, SESSION_COOKIE_NAME, isSecureRequest, sessionCookieOptions } from '@/lib/auth/auth-config';
import { getAuthErrorMessage } from '@/lib/auth/auth-errors';
import type { RegisterCredentials, AuthTokens } from '@/lib/auth/auth-types';

export async function POST(request: NextRequest) {
  try {
    const body: RegisterCredentials = await request.json();

    const response = await fetch(`${API_BASE_URL}/api/v1/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    const data = await response.json();

    if (!response.ok) {
      return NextResponse.json(
        { error: { code: data?.code ?? 'register_failed', message: data?.message ?? 'Registration failed' } },
        { status: response.status },
      );
    }

    const authData = data as AuthTokens;
    const cookieOpts = sessionCookieOptions(isSecureRequest(request));

    const nextResponse = NextResponse.json({ user: authData.user });
    nextResponse.cookies.set(SESSION_COOKIE_NAME, authData.access_token, cookieOpts);

    return nextResponse;
  } catch {
    return NextResponse.json(
      { error: { code: 'register_failed', message: getAuthErrorMessage('network_error') } },
      { status: 500 },
    );
  }
}
