'use client';

import Link from 'next/link';
import Image from 'next/image';
import { useState } from 'react';
import { usePathname } from 'next/navigation';
import { LayoutGrid, LogOut } from 'lucide-react';
import { cn } from '@/lib/utils';
import { NAV_ITEMS } from '@/lib/navigation';
import { useAuth } from '@/lib/auth/auth-hooks';

type SidebarProps = {
  mobileOpen?: boolean;
  onClose?: () => void;
};

export function Sidebar({ mobileOpen, onClose }: SidebarProps) {
  const pathname = usePathname();
  const [logoError, setLogoError] = useState(false);
  const { logout } = useAuth();

  return (
    <>
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-background/80 backdrop-blur-sm lg:hidden animate-in fade-in"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-[260px] flex-col border-r border-[#1e293b] bg-[#090E17] transition-all duration-300 ease-in-out lg:static lg:translate-x-0',
          mobileOpen ? 'translate-x-0 shadow-2xl shadow-primary/10' : '-translate-x-full',
        )}
      >
        <div className="flex h-20 shrink-0 items-center px-6 mt-4">
          {!logoError ? (
            <div className="relative h-8 w-32">
              <Image
                src="/logo.png"
                alt="LazyOps Logo"
                fill
                className="object-contain"
                onError={() => setLogoError(true)}
                unoptimized
                priority
              />
            </div>
          ) : (
            <div className="flex flex-col items-start leading-none tracking-tight">
              <span className="text-2xl font-black text-primary">LazyOps</span>
            </div>
          )}
        </div>

        <nav className="flex-1 overflow-y-auto px-4 py-4 scrollbar-hide" aria-label="Main navigation">
          <ul className="flex flex-col gap-2">
            {NAV_ITEMS.map((item) => {
              // Exact match or starting with the path, except for dashboard which usually matches /
              const isActive = pathname === item.href || (item.href !== '/dashboard' && pathname.startsWith(`${item.href}/`)) || (pathname === '/' && item.href === '/dashboard');
              const Icon = item.icon;
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className={cn(
                      'group flex items-center gap-3 rounded-lg px-4 py-3 text-[15px] font-medium transition-all duration-200 relative overflow-hidden',
                      isActive
                        ? 'text-[#38BDF8] bg-[#0c1a2c]'
                        : 'text-slate-400 hover:text-slate-200 hover:bg-[#111c2e]',
                    )}
                    onClick={onClose}
                  >
                    {Icon && (
                      <Icon className={cn("size-[18px]", isActive ? "text-[#38BDF8]" : "text-slate-400")} />
                    )}
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        <div className="p-4 border-t border-[#1e293b]">
          <button
            onClick={() => logout()}
            className="flex w-full items-center gap-3 rounded-lg px-4 py-3 text-[15px] font-medium text-slate-400 transition-all hover:text-slate-200 hover:bg-[#111c2e]"
          >
            <LogOut className="size-[18px]" />
            Đăng xuất
          </button>
        </div>
      </aside>
    </>
  );
}
