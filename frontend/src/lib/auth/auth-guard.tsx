'use client';

import { useEffect } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { useSession } from '@/lib/auth/auth-hooks';
import { LoadingPage } from '@/components/primitives/loading';

type AuthGuardProps = {
  children: React.ReactNode;
};

const PUBLIC_PATHS = ['/login', '/register'];

function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.some((path) => pathname === path || pathname.startsWith(`${path}/`));
}

export function AuthGuard({ children }: AuthGuardProps) {
  const { data: session, isLoading, isError } = useSession();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (isError && !isPublicPath(pathname)) {
      router.push(`/login?redirect=${encodeURIComponent(pathname)}`);
    }
  }, [isError, pathname, router]);

  if (isLoading) {
    return <LoadingPage label="Checking session…" />;
  }

  if (isError && !isPublicPath(pathname)) {
    return <LoadingPage label="Redirecting to login…" />;
  }

  if (!session && !isPublicPath(pathname)) {
    return null;
  }

  return <>{children}</>;
}
