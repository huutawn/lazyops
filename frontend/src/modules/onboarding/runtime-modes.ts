export type RuntimeMode = 'standalone' | 'distributed-mesh' | 'distributed-k3s';

export type RuntimeModeInfo = {
  id: RuntimeMode;
  title: string;
  description: string;
  useCase: string;
  icon: string;
};

export const RUNTIME_MODES: RuntimeModeInfo[] = [
  {
    id: 'standalone',
    title: 'Standalone',
    description: 'Single-instance deployment on one target machine. Simple, predictable, and ideal for small workloads or development environments.',
    useCase: 'Best for single-server apps, staging, and dev environments.',
    icon: '🖥️',
  },
  {
    id: 'distributed-mesh',
    title: 'Distributed Mesh',
    description: 'Services spread across multiple targets with a service mesh for inter-service communication. Provides high availability and horizontal scaling.',
    useCase: 'Best for production workloads that need resilience across multiple machines.',
    icon: '🌐',
  },
  {
    id: 'distributed-k3s',
    title: 'Distributed K3s',
    description: 'Lightweight Kubernetes clusters managed by LazyOps. Full orchestration capabilities with minimal operational overhead.',
    useCase: 'Best for teams that need K8s features without the complexity.',
    icon: '☸️',
  },
];
