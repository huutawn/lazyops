import type { ReactNode } from 'react';
import { AuthGuard } from '@/lib/auth/auth-guard';
import { ShellLayout } from '@/components/shell/shell-layout';

type AppLayoutProps = {
  children: ReactNode;
};

export default function AppLayout({ children }: AppLayoutProps) {
  return (
    <AuthGuard>
      <ShellLayout>{children}</ShellLayout>
    </AuthGuard>
  );
}
