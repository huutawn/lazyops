import { NextRequest } from 'next/server';
import { isSecureRequest } from '@/lib/auth/auth-config';
import { redirectRelative } from '@/lib/auth/redirect';

const OAUTH_NEXT_COOKIE = 'lazyops_oauth_next';

export async function GET(request: NextRequest) {
  const response = redirectRelative('/api/v1/auth/oauth/google/start', 307);
  const next = request.nextUrl.searchParams.get('next') ?? '';
  if (next.startsWith('/')) {
    response.cookies.set(OAUTH_NEXT_COOKIE, next, {
      path: '/',
      httpOnly: true,
      sameSite: 'lax',
      secure: isSecureRequest(request),
      maxAge: 10 * 60,
    });
  }
  return response;
}
