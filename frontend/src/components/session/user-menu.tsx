'use client';

import { LogOut } from 'lucide-react';
import { useSession } from '@/lib/auth/auth-hooks';
import { StatusBadge } from '@/components/primitives/status-badge';

export function UserMenu() {
  const { data: session } = useSession();

  if (!session) return null;
  const displayName = session.display_name || session.email || 'User';
  const avatarInitial = displayName.charAt(0).toUpperCase() || 'U';

  return (
    <div className="flex items-center gap-4">
      <div className="flex size-[36px] items-center justify-center rounded-full bg-[#0E3B4D] text-[#38BDF8] border border-[#38BDF8]/20 font-bold shadow-sm">
        {avatarInitial}
      </div>
    </div>
  );
}
