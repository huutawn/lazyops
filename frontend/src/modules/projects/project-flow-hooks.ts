'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from '@/lib/auth/auth-hooks';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';

export function useProjectNavigationMode() {
  const { data: session, isLoading: sessionLoading } = useSession();
  const threeStepFlowEnabled = isFeatureEnabled('ux_three_step_flow');
  const isAdmin = session?.role === 'admin';
  const guidedProjectFlow = threeStepFlowEnabled && !isAdmin;

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
    if (!sessionLoading && guidedProjectFlow && projectId) {
      router.replace(`/projects/${projectId}`);
    }
  }, [guidedProjectFlow, projectId, router, sessionLoading]);

  return {
    sessionLoading,
    guidedProjectFlow,
    shouldBlock: sessionLoading || guidedProjectFlow,
  };
}
