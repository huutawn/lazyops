'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { LayoutGrid } from 'lucide-react';
import { cn } from '@/lib/utils';
import { NAV_ITEMS } from '@/lib/navigation';

type SidebarProps = {
  mobileOpen?: boolean;
  onClose?: () => void;
};

export function Sidebar({ mobileOpen, onClose }: SidebarProps) {
  const pathname = usePathname();

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
          'fixed inset-y-0 left-0 z-50 flex w-[260px] flex-col border-r border-border bg-card/40 backdrop-blur-xl transition-all duration-300 ease-in-out lg:static lg:translate-x-0',
          mobileOpen ? 'translate-x-0 shadow-2xl shadow-primary/10' : '-translate-x-full',
        )}
      >
        <div className="flex h-20 shrink-0 items-center justify-center gap-3 border-b border-border/50 px-6 mt-2">
          {/* USER INSTRUCTION: Ensure /logo.png exists in /frontend/public/ */}
          <img src="/logo.png" alt="LazyOps Logo" className="h-10 object-contain drop-shadow-md" onError={(e) => {
            (e.target as HTMLImageElement).style.display = 'none';
            (e.target as HTMLImageElement).nextElementSibling?.classList.remove('hidden');
          }} />
          <div className="hidden flex-col items-start leading-none tracking-tight">
            <span className="text-xl font-black bg-gradient-to-r from-primary to-blue-500 bg-clip-text text-transparent">LazyOps</span>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto px-4 py-6 scrollbar-hide" aria-label="Main navigation">
          
          <div className="mb-4 px-2 text-xs font-bold uppercase tracking-wider text-muted-foreground/70">
            Menu Chính
          </div>

          <ul className="flex flex-col gap-1.5">
            {NAV_ITEMS.map((item) => {
              const isActive = pathname === item.href || pathname.startsWith(`${item.href}/`);
              const Icon = item.icon;
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className={cn(
                      'group flex items-center gap-3 rounded-md px-3 py-2.5 text-sm font-medium transition-all duration-200 relative overflow-hidden',
                      isActive
                        ? 'text-primary bg-primary/10 shadow-sm'
                        : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                    )}
                    onClick={onClose}
                  >
                    {isActive && (
                      <div className="absolute left-0 top-1/2 h-1/2 w-1 -translate-y-1/2 rounded-full bg-primary" />
                    )}
                    {Icon && (
                      <Icon className={cn("size-4 transition-transform group-hover:scale-110", isActive && "text-primary")} />
                    )}
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
      </aside>
    </>
  );
}
