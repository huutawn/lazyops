'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from '@/lib/auth/auth-hooks';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';

export function resolveGuidedProjectFlow(threeStepFlowEnabled: boolean, role?: string | null) {
  return threeStepFlowEnabled && role !== 'admin';
}

export function resolveProjectExpertRoute(projectId: string, guidedProjectFlow: boolean) {
  if (!guidedProjectFlow || !projectId) {
    return null;
  }
  return `/projects/${projectId}`;
}

export function useProjectNavigationMode() {
  const { data: session, isLoading: sessionLoading } = useSession();
  const threeStepFlowEnabled = isFeatureEnabled('ux_three_step_flow');
  const isAdmin = session?.role === 'admin';
  const guidedProjectFlow = resolveGuidedProjectFlow(threeStepFlowEnabled, session?.role);

  return {
    sessionLoading,
    isAdmin,
    threeStepFlowEnabled,
    guidedProjectFlow,
  };
}

export function useProjectExpertRouteGuard(projectId: string) {
  const router = useRouter();
  const { sessionLoading, guidedProjectFlow } = useProjectNavigationMode();

  useEffect(() => {
    const redirectTarget = resolveProjectExpertRoute(projectId, guidedProjectFlow);
    if (!sessionLoading && redirectTarget) {
      router.replace(redirectTarget);
    }
  }, [guidedProjectFlow, projectId, router, sessionLoading]);

  return {
    sessionLoading,
    guidedProjectFlow,
    shouldBlock: sessionLoading || guidedProjectFlow,
  };
}
