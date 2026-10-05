import type {Metadata} from 'next';
import type {ReactNode} from 'react';
import {ManagementProvider} from '@/components/management-provider';
import {Shell} from '@/components/shell';
import './globals.css';

export const metadata: Metadata = {
  title: 'FPBPack',
  description: 'FPBCraft mod management',
};

export default function RootLayout({children}: {children: ReactNode}) {
  return (
    <html lang="en" data-theme="fpbpack">
      <body>
        <ManagementProvider>
          <Shell>{children}</Shell>
        </ManagementProvider>
      </body>
    </html>
  );
}
