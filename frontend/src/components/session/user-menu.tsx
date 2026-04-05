'use client';

import { LogOut } from 'lucide-react';
import { useSession, useLogout } from '@/lib/auth/auth-hooks';
import { StatusBadge } from '@/components/primitives/status-badge';

export function UserMenu() {
  const { data: session } = useSession();
  const logout = useLogout();

  if (!session) return null;

  return (
    <div className="flex items-center gap-4">
      <div className="hidden lg:flex items-center gap-3 border-r border-border/50 pr-4">
        <div className="flex flex-col items-end">
          <span className="text-sm font-semibold tracking-tight text-foreground">{session.display_name}</span>
          <span className="text-xs text-muted-foreground">{session.email}</span>
        </div>
        <div className="flex size-9 items-center justify-center rounded-full bg-primary/20 text-primary font-bold shadow-inner">
          {session.display_name.charAt(0).toUpperCase()}
        </div>
      </div>
      
      <StatusBadge label={session.role} variant="info" size="sm" dot={true} className="hidden sm:inline-flex" />
      
      <button
        type="button"
        className="group flex size-9 items-center justify-center rounded-lg border border-border/50 bg-card transition-all hover:bg-destructive/10 hover:border-destructive/30 hover:text-destructive disabled:opacity-50 shadow-sm"
        onClick={() => logout.mutate()}
        disabled={logout.isPending}
        title="Sign out"
      >
        <LogOut className="size-4 text-muted-foreground transition-colors group-hover:text-destructive" />
      </button>
    </div>
  );
}
