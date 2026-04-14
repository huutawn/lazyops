import { ObservabilityConsole } from '@/modules/observability/observability-console';

export default function ObservabilityPage() {
  return (
    <ObservabilityConsole
      title="Observability"
      subtitle="Logs, traces, incidents, and metrics for your services."
    />
  );
}
