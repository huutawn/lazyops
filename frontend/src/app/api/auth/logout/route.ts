import { NextRequest, NextResponse } from 'next/server';
import { SESSION_COOKIE_NAME, isSecureRequest, sessionCookieOptions } from '@/lib/auth/auth-config';

export async function POST(request: NextRequest) {
  const response = NextResponse.json({ ok: true });
  response.cookies.set(SESSION_COOKIE_NAME, '', {
    ...sessionCookieOptions(isSecureRequest(request)),
    maxAge: 0,
  });
  return response;
}
