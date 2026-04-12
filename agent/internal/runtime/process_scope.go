package runtime

import "fmt"

func workloadProcessKey(runtimeCtx RuntimeContext, serviceName string) string {
	return scopedProcessKey("workload", runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, serviceName)
}

func sidecarProcessKey(runtimeCtx RuntimeContext, serviceName string) string {
	return scopedProcessKey("sidecar", runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, serviceName)
}

func scopedProcessKey(scope, projectID, bindingID, serviceName string) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		normalizeContainerToken(scope),
		normalizeContainerToken(projectID),
		normalizeContainerToken(bindingID),
		normalizeContainerToken(serviceName),
	)
}
