'use client';

import { useSession, useLogout } from '@/lib/auth/auth-hooks';
import { StatusBadge } from '@/components/primitives/status-badge';

export function UserMenu() {
  const { data: session } = useSession();
  const logout = useLogout();

  if (!session) return null;

  return (
    <div className="flex items-center gap-3">
      <StatusBadge label={session.role} variant="info" size="sm" dot={false} />
      <div className="flex flex-col">
        <span className="text-sm font-medium text-lazyops-text">{session.display_name}</span>
        <span className="text-xs text-lazyops-muted">{session.email}</span>
      </div>
      <button
        type="button"
        className="ml-2 rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 px-3 py-1.5 text-xs text-lazyops-muted transition-colors hover:text-lazyops-text disabled:opacity-50"
        onClick={() => logout.mutate()}
        disabled={logout.isPending}
      >
        {logout.isPending ? 'Signing out…' : 'Sign out'}
      </button>
    </div>
  );
}
