import type { ReactNode } from 'react';
import { LayoutGrid } from 'lucide-react';

export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen w-full bg-background">
      {/* Left Pane - Branding */}
      <div className="relative hidden w-1/2 flex-col justify-between overflow-hidden border-r border-border bg-card/30 p-12 lg:flex">
        <div className="absolute inset-0 bg-gradient-to-br from-primary/10 via-background to-background opacity-80" />
        <div className="absolute top-0 right-0 h-[500px] w-[500px] -translate-y-1/2 translate-x-1/3 rounded-full bg-primary/20 blur-[120px]" />
        
        <div className="relative z-10 flex items-center gap-3 text-2xl font-bold tracking-tight text-foreground">
          <div className="flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-lg shadow-primary/20">
            <LayoutGrid className="size-5" />
          </div>
          LazyOps
        </div>
        
        <div className="relative z-10 max-w-lg space-y-6 animate-in fade-in slide-in-from-left-4 duration-700">
          <h1 className="text-4xl font-semibold tracking-tight text-foreground leading-[1.1]">
            Multi-surface deployment <br/>
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-primary to-blue-400">control plane.</span>
          </h1>
          <p className="text-lg text-muted-foreground leading-relaxed">
            Manage your standalone endpoints, distributed meshes, and K3s clusters with zero friction.
          </p>
        </div>
      </div>
      
      {/* Right Pane - Form Container */}
      <div className="flex flex-1 flex-col items-center justify-center px-6 lg:px-8 relative">
        <div className="absolute top-0 right-0 h-[300px] w-[300px] translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/10 blur-[100px] lg:hidden" />
        <div className="w-full max-w-[400px] relative z-10">
          {children}
        </div>
      </div>
    </div>
  );
}
