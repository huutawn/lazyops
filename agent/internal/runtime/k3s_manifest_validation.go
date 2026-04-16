package runtime

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"lazyops-agent/internal/contracts"
)

var k3sServiceNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

type manifestPreflightResult struct {
	Warnings []string
}

type manifestDocumentRef struct {
	Order   int
	Content string
}

func validateK3sManifestPreflight(runtimeCtx RuntimeContext) (manifestPreflightResult, error) {
	result := manifestPreflightResult{Warnings: []string{}}
	serviceNames := make(map[string]contracts.K3sServiceSpecPayload, len(runtimeCtx.Revision.ServiceSpecs))
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		name := strings.TrimSpace(spec.Name)
		if err := validateK3sServiceName(name); err != nil {
			return result, err
		}
		if _, exists := serviceNames[name]; exists {
			return result, fmt.Errorf("duplicate k3s service name %q in revision payload", name)
		}
		serviceNames[name] = spec
	}

	for _, binding := range runtimeCtx.Revision.InternalBindings {
		source := strings.TrimSpace(binding.ServiceName)
		target := strings.TrimSpace(binding.TargetService)
		if source == "" || target == "" {
			return result, fmt.Errorf("internal binding must include service_name and target_service")
		}
		if _, ok := serviceNames[source]; !ok {
			return result, fmt.Errorf("internal binding references unknown service %q", source)
		}
		if _, ok := serviceNames[target]; !ok {
			return result, fmt.Errorf("internal binding target_service %q was not found in service specs", target)
		}
		if source == target {
			return result, fmt.Errorf("internal binding for service %q must not depend on itself", source)
		}
	}

	if len(runtimeCtx.Revision.ManifestBundle.Documents) == 0 {
		result.Warnings = append(result.Warnings, "manifest documents are empty; skipped secret/pvc/dependency document validation")
		return result, nil
	}

	docIndex, err := buildManifestDocumentIndex(runtimeCtx.Revision.ManifestBundle.Documents)
	if err != nil {
		return result, err
	}
	if _, ok := docIndex[manifestDocKey("Namespace", strings.TrimSpace(runtimeCtx.Project.Namespace))]; !ok && strings.TrimSpace(runtimeCtx.Project.Namespace) != "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("namespace document %q not found in manifest bundle", runtimeCtx.Project.Namespace))
	}

	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		name := strings.TrimSpace(spec.Name)
		serviceDoc, ok := docIndex[manifestDocKey("Service", name)]
		if !ok {
			return result, fmt.Errorf("manifest bundle is missing Service document for %q", name)
		}
		deploymentDoc, ok := docIndex[manifestDocKey("Deployment", name)]
		if !ok {
			return result, fmt.Errorf("manifest bundle is missing Deployment document for %q", name)
		}
		if serviceDoc.Order > deploymentDoc.Order {
			result.Warnings = append(result.Warnings, fmt.Sprintf("service document %q appears after deployment document", name))
		}

		if len(spec.EnvBundle) > 0 {
			secretName := name + "-env"
			secretDoc, ok := docIndex[manifestDocKey("Secret", secretName)]
			if !ok {
				return result, fmt.Errorf("manifest bundle is missing Secret document %q for service %q", secretName, name)
			}
			if !strings.Contains(deploymentDoc.Content, "secretRef:") || !strings.Contains(deploymentDoc.Content, "name: "+secretName) {
				return result, fmt.Errorf("deployment %q does not reference expected secret %q", name, secretName)
			}
			if secretDoc.Order > deploymentDoc.Order {
				result.Warnings = append(result.Warnings, fmt.Sprintf("secret document %q appears after deployment %q", secretName, name))
			}
		}

		if requiresPVCForK3sSpec(spec) {
			pvcName := name + "-data"
			pvcDoc, ok := docIndex[manifestDocKey("PersistentVolumeClaim", pvcName)]
			if !ok {
				return result, fmt.Errorf("manifest bundle is missing PersistentVolumeClaim document %q for service %q", pvcName, name)
			}
			if !strings.Contains(deploymentDoc.Content, "claimName: "+pvcName) {
				return result, fmt.Errorf("deployment %q does not reference expected pvc claim %q", name, pvcName)
			}
			if pvcDoc.Order > deploymentDoc.Order {
				result.Warnings = append(result.Warnings, fmt.Sprintf("pvc document %q appears after deployment %q", pvcName, name))
			}
		}
	}

	for _, binding := range runtimeCtx.Revision.InternalBindings {
		targetServiceDoc, ok := docIndex[manifestDocKey("Service", binding.TargetService)]
		if !ok {
			return result, fmt.Errorf("dependency target service document %q was not found", binding.TargetService)
		}
		targetDeploymentDoc, ok := docIndex[manifestDocKey("Deployment", binding.TargetService)]
		if !ok {
			return result, fmt.Errorf("dependency target deployment document %q was not found", binding.TargetService)
		}
		dependentDeploymentDoc, ok := docIndex[manifestDocKey("Deployment", binding.ServiceName)]
		if !ok {
			return result, fmt.Errorf("dependent deployment document %q was not found", binding.ServiceName)
		}
		if targetServiceDoc.Order > dependentDeploymentDoc.Order {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"dependency order warning: service %q depends on %q but target Service document appears after dependent Deployment",
				binding.ServiceName,
				binding.TargetService,
			))
		}
		if targetDeploymentDoc.Order > dependentDeploymentDoc.Order {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"dependency order warning: service %q depends on %q but target Deployment document appears after dependent Deployment",
				binding.ServiceName,
				binding.TargetService,
			))
		}
	}

	sort.Strings(result.Warnings)
	return result, nil
}

func buildManifestDocumentIndex(items []contracts.ManifestDocumentPayload) (map[string]manifestDocumentRef, error) {
	index := make(map[string]manifestDocumentRef, len(items))
	for order, item := range items {
		kind := strings.TrimSpace(item.Kind)
		name := strings.TrimSpace(item.Name)
		if kind == "" || name == "" {
			return nil, fmt.Errorf("manifest document at index %d must include kind and name", order)
		}
		key := manifestDocKey(kind, name)
		if _, exists := index[key]; exists {
			return nil, fmt.Errorf("duplicate manifest document %s", key)
		}
		index[key] = manifestDocumentRef{
			Order:   order,
			Content: item.Content,
		}
	}
	return index, nil
}

func manifestDocKey(kind, name string) string {
	return strings.TrimSpace(kind) + ":" + strings.TrimSpace(name)
}

func validateK3sServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("service name %q exceeds 63 characters", name)
	}
	if strings.Contains(name, "_") {
		return fmt.Errorf("service name %q must not contain underscores", name)
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("service name %q must be lowercase for k3s resources", name)
	}
	if !k3sServiceNamePattern.MatchString(name) {
		return fmt.Errorf("service name %q is not a valid k3s resource name", name)
	}
	return nil
}

func requiresPVCForK3sSpec(spec contracts.K3sServiceSpecPayload) bool {
	if size, ok := spec.PVCSpec["size"].(string); ok && strings.TrimSpace(size) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(spec.Kind)) {
	case "postgres", "mysql", "redis", "rabbitmq", "internal-db":
		return true
	default:
		return false
	}
}
