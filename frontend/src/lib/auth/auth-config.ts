import type { NextRequest } from 'next/server';

const API_BASE_URL =
  process.env.INTERNAL_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
const SESSION_COOKIE_NAME = 'lazyops_session';

function sessionCookieOptions(secure: boolean): {
  httpOnly: boolean;
  secure: boolean;
  sameSite: 'lax';
  path: string;
  maxAge: number;
} {
  return {
    httpOnly: true,
    secure,
    sameSite: 'lax' as const,
    path: '/',
    maxAge: 60 * 60 * 24 * 7,
  };
}

function isSecureRequest(request: NextRequest): boolean {
  const forwardedProto = request.headers.get('x-forwarded-proto');
  if (forwardedProto) {
    const proto = forwardedProto.split(',')[0]?.trim().toLowerCase();
    if (proto) {
      return proto === 'https';
    }
  }

  return request.nextUrl.protocol === 'https:';
}

export { API_BASE_URL, SESSION_COOKIE_NAME, sessionCookieOptions, isSecureRequest };
