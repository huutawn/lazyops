import { NextRequest, NextResponse } from 'next/server';

const OAUTH_NEXT_COOKIE = 'lazyops_oauth_next';

export async function GET(request: NextRequest) {
  const response = NextResponse.redirect(new URL('/api/v1/auth/oauth/google/start', request.url));
  const next = request.nextUrl.searchParams.get('next') ?? '';
  if (next.startsWith('/')) {
    response.cookies.set(OAUTH_NEXT_COOKIE, next, {
      path: '/',
      httpOnly: true,
      sameSite: 'lax',
      secure: request.nextUrl.protocol === 'https:',
      maxAge: 10 * 60,
    });
  }
  return response;
}
