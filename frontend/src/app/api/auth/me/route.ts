import { NextRequest, NextResponse } from 'next/server';
import { API_BASE_URL, SESSION_COOKIE_NAME, isSecureRequest, sessionCookieOptions } from '@/lib/auth/auth-config';
import type { UserSession } from '@/lib/auth/auth-types';

export async function GET(request: NextRequest) {
  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;

  if (!token) {
    return NextResponse.json({ error: { code: 'unauthorized', message: 'No session found' } }, { status: 401 });
  }

  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/users/me`, {
      headers: { Cookie: `${SESSION_COOKIE_NAME}=${token}` },
    });

    if (!response.ok) {
      const nextResponse = NextResponse.json(
        { error: { code: 'unauthorized', message: 'Session invalid' } },
        { status: 401 },
      );
      nextResponse.cookies.set(SESSION_COOKIE_NAME, '', {
        ...sessionCookieOptions(isSecureRequest(request)),
        maxAge: 0,
      });
      return nextResponse;
    }

    const user = (await response.json()) as UserSession;
    return NextResponse.json({ user });
  } catch {
    return NextResponse.json(
      { error: { code: 'network_error', message: 'Unable to verify session' } },
      { status: 500 },
    );
  }
}
