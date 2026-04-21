package service

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"lazyops-server/internal/models"
)

type ProjectRuntimeService struct {
	projects      ProjectStore
	projectSvc    *ProjectService
	deployments   *DeploymentService
	bindings      DeploymentBindingStore
	clusterNodes  *ClusterNodeService
	observability *ObservabilityService
}

func NewProjectRuntimeService(
	projects ProjectStore,
	projectSvc *ProjectService,
	deployments *DeploymentService,
	bindings DeploymentBindingStore,
	clusterNodes *ClusterNodeService,
	observability *ObservabilityService,
) *ProjectRuntimeService {
	return &ProjectRuntimeService{
		projects:      projects,
		projectSvc:    projectSvc,
		deployments:   deployments,
		bindings:      bindings,
		clusterNodes:  clusterNodes,
		observability: observability,
	}
}

func (s *ProjectRuntimeService) Get(ctx context.Context, requesterUserID, requesterRole, projectID string) (*ProjectRuntimeSummaryResult, error) {
	if s == nil || s.projects == nil || s.projectSvc == nil || s.deployments == nil {
		return nil, ErrInvalidInput
	}

	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	serviceList, err := s.projectSvc.ListServices(requesterUserID, requesterRole, project.ID)
	if err != nil {
		return nil, err
	}

	deployments, err := s.deployments.List(requesterUserID, requesterRole, project.ID)
	if err != nil {
		return nil, err
	}

	clusterID, err := s.resolveClusterID(project.ID, project)
	if err != nil {
		return nil, err
	}

	nodeItems := []ProjectRuntimeNodeRecord{}
	if clusterID != "" && s.clusterNodes != nil {
		clusterNodes, listErr := s.clusterNodes.ListClusterNodes(project.UserID, clusterID)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range clusterNodes.Items {
			nodeItems = append(nodeItems, ProjectRuntimeNodeRecord{
				ClusterID:   item.ClusterID,
				InstanceID:  item.InstanceID,
				Name:        item.Name,
				Status:      item.Status,
				K8sNodeName: item.K8sNodeName,
				Labels:      item.Labels,
				LastSeenAt:  item.LastSeenAt,
				IsReady:     item.IsReady,
			})
		}
	}

	deploymentByService := make(map[string]DeploymentOverviewRecord)
	for _, item := range deployments {
		for _, svc := range item.Services {
			if _, exists := deploymentByService[svc.Name]; exists {
				continue
			}
			deploymentByService[svc.Name] = item
		}
	}

	liveDeployment, stableDeployment := summarizeProjectDeployments(deployments)
	serviceRecords := make([]ProjectRuntimeServiceRecord, 0, len(serviceList.Items))
	runtimeByName := make(map[string]ProjectRuntimeServiceRecord, len(serviceList.Items))
	for _, item := range serviceList.Items {
		record := s.buildRuntimeServiceRecord(
			ctx,
			item,
			deploymentByService[item.Name],
			firstRuntimeNonEmpty(liveDeployment.Namespace, project.NamespaceSlug, project.Slug),
			project.ID,
		)
		serviceRecords = append(serviceRecords, record)
		runtimeByName[item.Name] = record
	}

	for i := range serviceRecords {
		targetName := strings.TrimSpace(serviceList.Items[i].ConnectionTargetService)
		if targetName == "" {
			continue
		}
		serviceRecords[i].Dependencies = append(serviceRecords[i].Dependencies, buildRuntimeDependency(targetName, runtimeByName, serviceList.Items, firstRuntimeNonEmpty(project.NamespaceSlug, project.Slug)))
	}

	result := &ProjectRuntimeSummaryResult{
		ProjectID:        project.ID,
		RuntimeMode:      firstRuntimeNonEmpty(liveDeployment.RuntimeMode, project.RuntimeMode),
		ClusterID:        clusterID,
		Namespace:        firstRuntimeNonEmpty(liveDeployment.Namespace, project.NamespaceSlug, project.Slug),
		LiveRevisionID:   liveDeployment.RevisionID,
		LiveRevision:     liveDeployment.Revision,
		StableRevisionID: stableDeployment.RevisionID,
		StableRevision:   stableDeployment.Revision,
		SyncState:        "missing",
		SyncReason:       "Chua co deployment nao cho project nay.",
		PublicURLs:       uniqueNonEmptyStrings(liveDeployment.PublicURLs),
		PublicURLStatus:  strings.TrimSpace(liveDeployment.PublicURLStatus),
		PublicURLReason:  strings.TrimSpace(liveDeployment.PublicURLReason),
		Nodes:            nodeItems,
		Services:         serviceRecords,
	}
	if liveDeployment.ID != "" {
		result.SyncState = "synced"
		result.SyncReason = ""
	}
	if result.RuntimeMode == "" {
		result.RuntimeMode = project.RuntimeMode
	}
	if result.Namespace == "" {
		result.Namespace = firstRuntimeNonEmpty(project.NamespaceSlug, project.Slug)
	}

	return result, nil
}

func (s *ProjectRuntimeService) resolveClusterID(projectID string, project *models.Project) (string, error) {
	if project != nil && project.ClusterID != nil && strings.TrimSpace(*project.ClusterID) != "" {
		return strings.TrimSpace(*project.ClusterID), nil
	}
	if s.bindings == nil {
		return "", nil
	}
	autoBinding, err := s.bindings.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return "", err
	}
	if autoBinding != nil && strings.TrimSpace(autoBinding.TargetKind) == "cluster" {
		return strings.TrimSpace(autoBinding.TargetID), nil
	}
	items, err := s.bindings.ListByProject(projectID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if strings.TrimSpace(item.TargetKind) == "cluster" {
			return strings.TrimSpace(item.TargetID), nil
		}
	}
	return "", nil
}

func (s *ProjectRuntimeService) buildRuntimeServiceRecord(
	ctx context.Context,
	service ProjectServiceRecord,
	deployment DeploymentOverviewRecord,
	namespace string,
	projectID string,
) ProjectRuntimeServiceRecord {
	serviceSpec := matchRuntimeServiceSpec(service, deployment)
	recentLogs := []ProjectRuntimeLogPreviewRecord{}
	if s.observability != nil && strings.TrimSpace(projectID) != "" {
		logs, err := s.observability.ListRecentLogs(ctx, projectID, service.Name, "", "", "", 5)
		if err == nil {
			recentLogs = toRuntimeLogPreviewRecords(logs)
		}
	}

	publicURLs := filterServicePublicURLs(service.Name, service.Public, deployment.PublicURLs, deployment.Services)
	effectiveNodeIDs := placementNodeIDsForService(service.Name, deployment.PlacementAssignments)
	runtimeStatus, runtimeReason := resolveRuntimeStatus(service, deployment, effectiveNodeIDs)

	record := ProjectRuntimeServiceRecord{
		ServiceID:         service.ID,
		Name:              service.Name,
		Kind:              service.Kind,
		SourceType:        service.SourceType,
		Public:            service.Public,
		RuntimeProfile:    service.RuntimeProfile,
		RuntimeStatus:     runtimeStatus,
		RuntimeReason:     runtimeReason,
		BuildState:        deployment.BuildState,
		RolloutState:      deployment.RolloutState,
		PlacementMode:     service.PlacementMode,
		RequestedNodeID:   service.PlacementNodeID,
		EffectiveNodeIDs:  effectiveNodeIDs,
		ImageRef:          firstRuntimeNonEmpty(serviceSpec.ImageRef, service.ImageRef),
		ImageDigest:       firstRuntimeNonEmpty(serviceSpec.ImageDigest, service.ImageDigest),
		RevisionID:        deployment.RevisionID,
		Revision:          deployment.Revision,
		DeploymentID:      deployment.ID,
		PublicURLs:        publicURLs,
		InternalEndpoints: buildInternalEndpoints(service, namespace),
		Dependencies:      []ProjectRuntimeDependencyRecord{},
		RecentLogs:        recentLogs,
	}

	if len(record.PublicURLs) == 0 && service.Public && deployment.ID != "" {
		record.PublicURLs = uniqueNonEmptyStrings(deployment.PublicURLs)
	}
	if len(record.EffectiveNodeIDs) == 0 && service.PlacementMode == servicePlacementModePinnedNode && service.PlacementNodeID != "" {
		record.EffectiveNodeIDs = []string{service.PlacementNodeID}
	}
	if deployment.ID == "" {
		record.ImageRef = firstRuntimeNonEmpty(service.ImageRef, serviceSpec.ImageRef)
		record.ImageDigest = firstRuntimeNonEmpty(service.ImageDigest, serviceSpec.ImageDigest)
	}

	return record
}

func summarizeProjectDeployments(items []DeploymentOverviewRecord) (DeploymentOverviewRecord, DeploymentOverviewRecord) {
	if len(items) == 0 {
		return DeploymentOverviewRecord{}, DeploymentOverviewRecord{}
	}
	live := items[0]
	for _, item := range items[1:] {
		if item.CreatedAt.After(live.CreatedAt) {
			live = item
		}
	}
	stable := DeploymentOverviewRecord{}
	for _, item := range items {
		if item.Promoted || item.RolloutState == DeploymentStatusPromoted || item.BuildState == RevisionStatusPromoted {
			stable = item
			break
		}
	}
	if stable.ID == "" {
		stable = live
	}
	return live, stable
}

func matchRuntimeServiceSpec(service ProjectServiceRecord, deployment DeploymentOverviewRecord) K3sServiceSpecRecord {
	for _, item := range deployment.ServiceSpecs {
		if strings.TrimSpace(item.Name) == strings.TrimSpace(service.Name) {
			return item
		}
	}
	return K3sServiceSpecRecord{
		Name:            service.Name,
		Kind:            service.Kind,
		Path:            service.Path,
		Public:          service.Public,
		PlacementMode:   service.PlacementMode,
		PlacementNodeID: service.PlacementNodeID,
		RuntimeProfile:  service.RuntimeProfile,
		StartHint:       service.StartHint,
		ImageRef:        service.ImageRef,
		ImageDigest:     service.ImageDigest,
		TargetPort:      service.TargetPort,
		ServicePort:     service.ServicePort,
		Replicas:        service.Replicas,
		Healthcheck:     service.Healthcheck,
		DetectedPorts:   service.DetectedPorts,
		EnvBundle:       service.EnvBundle,
		PVCSpec:         service.PVCSpec,
		DeployStrategy:  service.DeployStrategy,
	}
}

func placementNodeIDsForService(serviceName string, placements []PlacementAssignmentRecord) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range placements {
		if strings.TrimSpace(item.ServiceName) != strings.TrimSpace(serviceName) {
			continue
		}
		if strings.TrimSpace(item.TargetKind) != "instance" {
			continue
		}
		targetID := strings.TrimSpace(item.TargetID)
		if targetID == "" {
			continue
		}
		if _, exists := seen[targetID]; exists {
			continue
		}
		seen[targetID] = struct{}{}
		result = append(result, targetID)
	}
	sort.Strings(result)
	return result
}

func resolveRuntimeStatus(service ProjectServiceRecord, deployment DeploymentOverviewRecord, effectiveNodeIDs []string) (string, string) {
	if deployment.ID == "" {
		if service.SourceType == serviceSourceTypeInternal {
			return "configured", "Internal service da duoc khai bao nhung chua co rollout."
		}
		return "not_deployed", "Service chua co deployment nao."
	}
	switch deployment.RolloutState {
	case DeploymentStatusPromoted, DeploymentStatusCandidateReady:
		return "live", "Service dang duoc phuc vu tu revision moi nhat."
	case DeploymentStatusRunning, DeploymentStatusQueued:
		return "deploying", "Service dang rollout."
	case DeploymentStatusFailed, DeploymentStatusRolledBack, DeploymentStatusCanceled:
		return "degraded", fmt.Sprintf("Rollout hien tai o trang thai %s.", deployment.RolloutState)
	}
	switch deployment.BuildState {
	case RevisionStatusFailed, RevisionStatusRolledBack:
		return "degraded", fmt.Sprintf("Build/revision hien tai o trang thai %s.", deployment.BuildState)
	case RevisionStatusApplying, RevisionStatusPlanned, RevisionStatusArtifactReady, RevisionStatusQueued, RevisionStatusBuilding:
		return "deploying", "Service dang duoc chuan bi cho runtime."
	}
	if service.PlacementMode == servicePlacementModePinnedNode && len(effectiveNodeIDs) == 0 {
		return "waiting_for_node", "Service da pin node nhung chua ghi nhan node runtime hieu luc."
	}
	return "configured", "Service da co deployment history nhung chua co runtime signal cu the."
}

func buildInternalEndpoints(service ProjectServiceRecord, namespace string) []string {
	host := strings.TrimSpace(service.Name)
	namespace = strings.TrimSpace(namespace)
	port := service.ServicePort
	if port <= 0 {
		port = service.TargetPort
	}
	if host == "" || namespace == "" || port <= 0 {
		return []string{}
	}
	shortHost := fmt.Sprintf("%s:%d", host, port)
	fqdn := fmt.Sprintf("%s.%s.svc.cluster.local:%d", host, namespace, port)
	return []string{shortHost, fqdn}
}

func buildRuntimeDependency(
	targetName string,
	runtimeByName map[string]ProjectRuntimeServiceRecord,
	services []ProjectServiceRecord,
	namespace string,
) ProjectRuntimeDependencyRecord {
	record := ProjectRuntimeDependencyRecord{
		ServiceName:  strings.TrimSpace(targetName),
		Status:       "missing",
		StatusReason: "Dependency service khong ton tai trong inventory hien tai.",
	}
	for _, item := range services {
		if strings.TrimSpace(item.Name) != strings.TrimSpace(targetName) {
			continue
		}
		record.ServiceID = item.ID
		record.InternalEndpoint = firstNonEmptySlice(buildInternalEndpoints(item, namespace))
		runtime, ok := runtimeByName[item.Name]
		if !ok {
			record.Status = "configured"
			record.StatusReason = "Dependency da duoc cau hinh nhung chua co runtime summary."
			return record
		}
		record.Status = dependencyStatusFromRuntime(runtime.RuntimeStatus)
		record.StatusReason = runtime.RuntimeReason
		if endpoint := firstNonEmptySlice(runtime.InternalEndpoints); endpoint != "" {
			record.InternalEndpoint = endpoint
		}
		return record
	}
	return record
}

func dependencyStatusFromRuntime(runtimeStatus string) string {
	switch strings.TrimSpace(runtimeStatus) {
	case "live":
		return "ready"
	case "deploying", "configured":
		return "configured"
	case "waiting_for_node":
		return "degraded"
	default:
		return "missing"
	}
}

func toRuntimeLogPreviewRecords(items []LogLineRecord) []ProjectRuntimeLogPreviewRecord {
	out := make([]ProjectRuntimeLogPreviewRecord, 0, len(items))
	for _, item := range items {
		out = append(out, ProjectRuntimeLogPreviewRecord{
			ID:            item.ID,
			Source:        item.Source,
			Level:         item.Level,
			Message:       item.Message,
			Timestamp:     item.Timestamp,
			Node:          item.Node,
			CorrelationID: item.CorrelationID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	return out
}

func filterServicePublicURLs(serviceName string, isPublic bool, publicURLs []string, services []BlueprintServiceContractRecord) []string {
	if !isPublic {
		return []string{}
	}
	publicCount := 0
	for _, item := range services {
		if item.Public {
			publicCount++
		}
	}
	if publicCount <= 1 {
		return uniqueNonEmptyStrings(publicURLs)
	}
	filtered := make([]string, 0)
	for _, raw := range publicURLs {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		if strings.HasPrefix(parsed.Hostname(), strings.TrimSpace(serviceName)+".") {
			filtered = append(filtered, raw)
		}
	}
	return uniqueNonEmptyStrings(filtered)
}

func uniqueNonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func firstRuntimeNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptySlice(values ...[]string) string {
	for _, value := range values {
		for _, item := range value {
			if strings.TrimSpace(item) != "" {
				return strings.TrimSpace(item)
			}
		}
	}
	return ""
}
