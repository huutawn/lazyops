import type { Metadata } from 'next';
import type { ReactNode } from 'react';

import { AppProviders } from '@/lib/providers/app-providers';
import '@/styles/globals.css';

export const metadata: Metadata = {
  title: 'LazyOps Console',
  description: 'Trình quản lý và triển khai dự án dễ dàng cho LazyOps.',
  icons: {
    icon: '/favicon.ico',
  },
};

type RootLayoutProps = {
  children: ReactNode;
};

export default function RootLayout({ children }: RootLayoutProps) {
  return (
    <html lang="en">
      <body>
        <AppProviders>{children}</AppProviders>
      </body>
    </html>
  );
}
