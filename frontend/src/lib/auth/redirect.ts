import { NextResponse } from 'next/server';

export function redirectRelative(path: string, status = 302): NextResponse {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return new NextResponse(null, {
    status,
    headers: {
      Location: normalizedPath,
    },
  });
}
