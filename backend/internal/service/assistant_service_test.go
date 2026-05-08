package service

import (
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeProjectRepoLinkStoreForAssistant struct {
	items map[string]*models.ProjectRepoLink
}

func newFakeProjectRepoLinkStoreForAssistant(items ...*models.ProjectRepoLink) *fakeProjectRepoLinkStoreForAssistant {
	store := &fakeProjectRepoLinkStoreForAssistant{items: map[string]*models.ProjectRepoLink{}}
	for _, item := range items {
		if item != nil {
			copy := *item
			store.items[item.ProjectID] = &copy
		}
	}
	return store
}

func (f *fakeProjectRepoLinkStoreForAssistant) Upsert(link *models.ProjectRepoLink) error {
	copy := *link
	f.items[link.ProjectID] = &copy
	return nil
}

func (f *fakeProjectRepoLinkStoreForAssistant) GetByProjectID(projectID string) (*models.ProjectRepoLink, error) {
	item, ok := f.items[projectID]
	if !ok {
		return nil, nil
	}
	copy := *item
	return &copy, nil
}

func (f *fakeProjectRepoLinkStoreForAssistant) GetByRepoBranch(githubInstallationID string, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	return nil, nil
}

func (f *fakeProjectRepoLinkStoreForAssistant) LookupWebhookRoute(githubInstallationID int64, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	return nil, nil
}

type fakeDeploymentBindingStoreForAssistant struct {
	items map[string][]models.DeploymentBinding
}

func newFakeDeploymentBindingStoreForAssistant(items map[string][]models.DeploymentBinding) *fakeDeploymentBindingStoreForAssistant {
	return &fakeDeploymentBindingStoreForAssistant{items: items}
}

func (f *fakeDeploymentBindingStoreForAssistant) Create(binding *models.DeploymentBinding) error {
	f.items[binding.ProjectID] = append(f.items[binding.ProjectID], *binding)
	return nil
}

func (f *fakeDeploymentBindingStoreForAssistant) UpsertAuto(binding *models.DeploymentBinding) error {
	return f.Create(binding)
}

func (f *fakeDeploymentBindingStoreForAssistant) ListByProject(projectID string) ([]models.DeploymentBinding, error) {
	return append([]models.DeploymentBinding{}, f.items[projectID]...), nil
}

func (f *fakeDeploymentBindingStoreForAssistant) GetByTargetRefForProject(projectID, targetRef string) (*models.DeploymentBinding, error) {
	for _, item := range f.items[projectID] {
		if item.TargetRef == targetRef {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeDeploymentBindingStoreForAssistant) GetByIDForProject(projectID, bindingID string) (*models.DeploymentBinding, error) {
	for _, item := range f.items[projectID] {
		if item.ID == bindingID {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}

type fakeAssistantSessionStore struct {
	items map[string]models.AssistantSession
}

func newFakeAssistantSessionStore() *fakeAssistantSessionStore {
	return &fakeAssistantSessionStore{items: map[string]models.AssistantSession{}}
}

func (f *fakeAssistantSessionStore) Create(item *models.AssistantSession) error {
	f.items[item.ID] = *item
	return nil
}

func (f *fakeAssistantSessionStore) GetByIDForUser(sessionID, userID string) (*models.AssistantSession, error) {
	item, ok := f.items[sessionID]
	if !ok || item.UserID != userID {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (f *fakeAssistantSessionStore) ListByUser(userID string) ([]models.AssistantSession, error) {
	out := make([]models.AssistantSession, 0)
	for _, item := range f.items {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeAssistantSessionStore) Update(item *models.AssistantSession) error {
	f.items[item.ID] = *item
	return nil
}

func (f *fakeAssistantSessionStore) Touch(sessionID string, at time.Time) error {
	item := f.items[sessionID]
	item.LastMessageAt = at
	item.UpdatedAt = at
	f.items[sessionID] = item
	return nil
}

type fakeAssistantMessageStore struct {
	items []models.AssistantMessage
}

func (f *fakeAssistantMessageStore) Create(item *models.AssistantMessage) error {
	f.items = append(f.items, *item)
	return nil
}

func (f *fakeAssistantMessageStore) ListBySession(sessionID string) ([]models.AssistantMessage, error) {
	out := make([]models.AssistantMessage, 0)
	for _, item := range f.items {
		if item.SessionID == sessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

type fakeAssistantActionPlanStore struct {
	items map[string]models.AssistantActionPlan
}

func newFakeAssistantActionPlanStore() *fakeAssistantActionPlanStore {
	return &fakeAssistantActionPlanStore{items: map[string]models.AssistantActionPlan{}}
}

func (f *fakeAssistantActionPlanStore) Create(item *models.AssistantActionPlan) error {
	f.items[item.ID] = *item
	return nil
}

func (f *fakeAssistantActionPlanStore) GetByID(planID string) (*models.AssistantActionPlan, error) {
	item, ok := f.items[planID]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (f *fakeAssistantActionPlanStore) GetLatestPendingBySession(sessionID string) (*models.AssistantActionPlan, error) {
	var latest *models.AssistantActionPlan
	for _, item := range f.items {
		if item.SessionID != sessionID {
			continue
		}
		copy := item
		if latest == nil || latest.CreatedAt.Before(copy.CreatedAt) {
			latest = &copy
		}
	}
	return latest, nil
}

func (f *fakeAssistantActionPlanStore) Update(item *models.AssistantActionPlan) error {
	f.items[item.ID] = *item
	return nil
}

type fakeAssistantAuditStore struct {
	items []models.AssistantAuditEvent
}

func (f *fakeAssistantAuditStore) Create(item *models.AssistantAuditEvent) error {
	f.items = append(f.items, *item)
	return nil
}

type fakeDeploymentStoreForAssistant struct {
	items []DeploymentOverviewRecord
}

func (f *fakeDeploymentStoreForAssistant) Create(deployment *models.Deployment) error { return nil }
func (f *fakeDeploymentStoreForAssistant) GetByIDForProject(projectID, deploymentID string) (*models.Deployment, error) {
	return nil, nil
}
func (f *fakeDeploymentStoreForAssistant) ListByProject(projectID string) ([]models.Deployment, error) {
	return nil, nil
}
func (f *fakeDeploymentStoreForAssistant) UpdateStatus(deploymentID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
	return nil
}

type fakeRevisionStoreForAssistant struct{}

func (f *fakeRevisionStoreForAssistant) Create(revision *models.DesiredStateRevision) error { return nil }
func (f *fakeRevisionStoreForAssistant) GetByIDForProject(projectID, revisionID string) (*models.DesiredStateRevision, error) {
	return nil, nil
}
func (f *fakeRevisionStoreForAssistant) ListByProject(projectID string) ([]models.DesiredStateRevision, error) {
	return nil, nil
}
func (f *fakeRevisionStoreForAssistant) UpdateStatus(revisionID, status string, at time.Time) error { return nil }
func (f *fakeRevisionStoreForAssistant) UpdateSnapshot(revisionID, compiledRevisionJSON, manifestBundleJSON string, at time.Time) error {
	return nil
}

func newAssistantDeploymentService(projects ProjectStore) *DeploymentService {
	return &DeploymentService{
		projects:    projects,
		blueprints:  newFakeBlueprintStore(),
		revisions:   &fakeRevisionStoreForAssistant{},
		deployments: newFakeDeploymentStore(),
	}
}

type fakeDesiredStateRevisionStoreForAssistant struct {
	items map[string][]models.DesiredStateRevision
}

func (f *fakeDesiredStateRevisionStoreForAssistant) Create(revision *models.DesiredStateRevision) error { return nil }
func (f *fakeDesiredStateRevisionStoreForAssistant) GetByIDForProject(projectID, revisionID string) (*models.DesiredStateRevision, error) {
	for _, item := range f.items[projectID] {
		if item.ID == revisionID {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}
func (f *fakeDesiredStateRevisionStoreForAssistant) ListByProject(projectID string) ([]models.DesiredStateRevision, error) {
	return append([]models.DesiredStateRevision{}, f.items[projectID]...), nil
}
func (f *fakeDesiredStateRevisionStoreForAssistant) UpdateStatus(revisionID, status string, at time.Time) error { return nil }
func (f *fakeDesiredStateRevisionStoreForAssistant) UpdateSnapshot(revisionID, compiledRevisionJSON, manifestBundleJSON string, at time.Time) error {
	return nil
}

type fakeDeploymentStoreForAssistantReadOnly struct {
	items map[string][]models.Deployment
}

func (f *fakeDeploymentStoreForAssistantReadOnly) Create(deployment *models.Deployment) error { return nil }
func (f *fakeDeploymentStoreForAssistantReadOnly) GetByIDForProject(projectID, deploymentID string) (*models.Deployment, error) {
	for _, item := range f.items[projectID] {
		if item.ID == deploymentID {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}
func (f *fakeDeploymentStoreForAssistantReadOnly) ListByProject(projectID string) ([]models.Deployment, error) {
	return append([]models.Deployment{}, f.items[projectID]...), nil
}
func (f *fakeDeploymentStoreForAssistantReadOnly) UpdateStatus(deploymentID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
	return nil
}

func TestAssistantServiceCreatesClarificationPlan(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {{ID: "bind_123", ProjectID: "prj_123", TargetRef: "prod-main", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_123"}},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy this",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil {
		t.Fatalf("expected pending plan")
	}
	if conversation.UIState != AssistantUIStatePlanning {
		t.Fatalf("expected planning state, got %q", conversation.UIState)
	}
	if len(conversation.PendingPlan.MissingInputs) == 0 {
		t.Fatalf("expected missing inputs, got %#v", conversation.PendingPlan)
	}
}

func TestAssistantServiceCreatesConfirmationPlanForProduction(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, newFakeProjectRepoLinkStoreForAssistant(&models.ProjectRepoLink{ProjectID: "prj_123", RepoOwner: "lazyops", RepoName: "backend", TrackedBranch: "main"}), nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {{ID: "bind_123", ProjectID: "prj_123", TargetRef: "prod-main", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_123"}},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch main to production",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil {
		t.Fatalf("expected pending plan")
	}
	if conversation.PendingPlan.Status != AssistantPlanStatusAwaitingConfirmation {
		t.Fatalf("expected awaiting confirmation, got %q", conversation.PendingPlan.Status)
	}
	if !conversation.PendingPlan.RequiresConfirmation {
		t.Fatalf("expected confirmation to be required")
	}
	if sourceRef, _ := conversation.PendingPlan.Plan["source_ref"].(string); sourceRef != "refs/heads/main" {
		t.Fatalf("expected canonical branch ref, got %q", sourceRef)
	}
}

func TestAssistantServiceRequiresRepoLinkForPullRequestDeploy(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, newFakeProjectRepoLinkStoreForAssistant(), nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {{ID: "bind_123", ProjectID: "prj_123", TargetRef: "prod-main", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_123"}},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy pr #42 to production",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil {
		t.Fatalf("expected pending plan")
	}
	if conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan, got %q", conversation.PendingPlan.Status)
	}
	if last := conversation.Messages[len(conversation.Messages)-1].Content; last == "" || last == "I prepared a production deploy plan. Review the target and confirm when ready." {
		t.Fatalf("expected clarification about repo link, got %q", last)
	}
}

func TestAssistantServiceRequiresDeploymentBindingBeforeApproval(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch main to staging",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil {
		t.Fatalf("expected pending plan")
	}
	if conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan when binding is missing, got %q", conversation.PendingPlan.Status)
	}
}

func TestAssistantServiceMatchesEnvironmentToBinding(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {
			{ID: "bind_stage", ProjectID: "prj_123", Name: "staging", TargetRef: "staging-main", TargetEnvironment: "staging", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_stage"},
			{ID: "bind_prod", ProjectID: "prj_123", Name: "production", TargetRef: "prod-main", TargetEnvironment: "production", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_prod"},
		},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch release to production",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil {
		t.Fatalf("expected pending plan")
	}
	if bindingID, _ := conversation.PendingPlan.Plan["deployment_binding_id"].(string); bindingID != "bind_prod" {
		t.Fatalf("expected prod binding, got %q", bindingID)
	}
}

func TestAssistantServiceRejectsRepoMismatchFromPrompt(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, newFakeProjectRepoLinkStoreForAssistant(&models.ProjectRepoLink{ProjectID: "prj_123", RepoOwner: "lazyops", RepoName: "backend", TrackedBranch: "main"}), nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {{ID: "bind_prod", ProjectID: "prj_123", Name: "production", TargetRef: "prod-main", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_prod"}},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch main from otherorg/otherrepo to production",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil || conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan for repo mismatch, got %#v", conversation.PendingPlan)
	}
}

func TestAssistantServiceRequiresEnvBundleWhenInternalServicesExist(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})
	internalStore := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{
		"prj_123": {{ID: "pis_123", ProjectID: "prj_123", Kind: "postgres", Alias: "postgres", Protocol: "postgres", Port: 5432}},
	})
	bundleStore := newFakeProjectEnvBundleStore()

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, bundleStore, internalStore, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {{ID: "bind_stage", ProjectID: "prj_123", Name: "staging", TargetRef: "staging-main", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_stage"}},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch main to staging",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil || conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan when env bundle is missing, got %#v", conversation.PendingPlan)
	}
}

func TestAssistantServiceParsesGitHubRepoAndPullRequestURLs(t *testing.T) {
	if repo := extractRepoFullName("deploy https://github.com/lazyops/backend/pull/42 to production"); repo != "lazyops/backend" {
		t.Fatalf("expected lazyops/backend from GitHub URL, got %q", repo)
	}
	if ref := extractSourceRef("deploy https://github.com/lazyops/backend/pull/42 to production"); ref != "refs/pull/42/head" {
		t.Fatalf("expected PR ref from GitHub URL, got %q", ref)
	}
	if repo := extractRepoFullName("deploy https://github.com/lazyops/backend/tree/main to staging"); repo != "lazyops/backend" {
		t.Fatalf("expected lazyops/backend from tree URL, got %q", repo)
	}
}

func TestAssistantServiceRequiresManagedEnvKeysForInternalServices(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})
	internalStore := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{
		"prj_123": {{ID: "pis_123", ProjectID: "prj_123", Kind: "postgres", Alias: "postgres", Protocol: "postgres", Port: 5432}},
	})
	bundleStore := newFakeProjectEnvBundleStore(&models.ProjectEnvBundle{
		ProjectID:    "prj_123",
		EnvEncrypted: "encrypted-placeholder",
	})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, bundleStore, internalStore, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {{ID: "bind_stage", ProjectID: "prj_123", Name: "staging", TargetRef: "staging-main", TargetEnvironment: "staging", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_stage"}},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch main to staging",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil || conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan when managed env keys are missing, got %#v", conversation.PendingPlan)
	}
}

func TestAssistantServiceRequestsBindingWhenEnvironmentIsAmbiguous(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {
			{ID: "bind_prod_a", ProjectID: "prj_123", Name: "production-a", TargetRef: "prod-a", TargetEnvironment: "production", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_a"},
			{ID: "bind_prod_b", ProjectID: "prj_123", Name: "production-b", TargetRef: "prod-b", TargetEnvironment: "production", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_b"},
		},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch release to production",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil || conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan for ambiguous production binding, got %#v", conversation.PendingPlan)
	}
}

func TestAssistantServiceUsesExplicitBindingHintFromPrompt(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {
			{ID: "bind_prod_a", ProjectID: "prj_123", Name: "production-a", TargetRef: "prod-a", TargetEnvironment: "production", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_a"},
			{ID: "bind_prod_b", ProjectID: "prj_123", Name: "production-b", TargetRef: "prod-b", TargetEnvironment: "production", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_b"},
		},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch release to production using binding prod-b",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil {
		t.Fatalf("expected pending plan")
	}
	if bindingID, _ := conversation.PendingPlan.Plan["deployment_binding_id"].(string); bindingID != "bind_prod_b" {
		t.Fatalf("expected bind_prod_b, got %q", bindingID)
	}
}

func TestAssistantServiceRejectsUnknownBindingHint(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, nil, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{
		"prj_123": {
			{ID: "bind_prod_a", ProjectID: "prj_123", Name: "production-a", TargetRef: "prod-a", TargetEnvironment: "production", RuntimeMode: "standalone", TargetKind: "instance", TargetID: "inst_a"},
		},
	}))
	session, err := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	conversation, err := svc.PostMessage(AssistantMessageCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		SessionID: session.ID,
		Content:   "deploy branch release to production using target_ref prod-z",
	})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if conversation.PendingPlan == nil || conversation.PendingPlan.Status != AssistantPlanStatusDraft {
		t.Fatalf("expected draft plan for unknown binding hint, got %#v", conversation.PendingPlan)
	}
}

func TestAssistantServiceReturnsDeploymentStatusSummary(t *testing.T) {
	sessions := newFakeAssistantSessionStore()
	messages := &fakeAssistantMessageStore{}
	plans := newFakeAssistantActionPlanStore()
	audit := &fakeAssistantAuditStore{}
	projectStore := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"})
	deploymentSvc := &DeploymentService{
		projects: projectStore,
		revisions: &fakeDesiredStateRevisionStoreForAssistant{items: map[string][]models.DesiredStateRevision{
			"prj_123": {{ID: "rev_123", ProjectID: "prj_123", Status: RevisionStatusPromoted}},
		}},
		deployments: &fakeDeploymentStoreForAssistantReadOnly{items: map[string][]models.Deployment{
			"prj_123": {{ID: "dep_123", ProjectID: "prj_123", RevisionID: "rev_123", Status: DeploymentStatusPromoted, CreatedAt: time.Now().UTC()}},
		}},
	}

	svc := NewAssistantService(sessions, messages, plans, audit, projectStore, nil, nil, nil, deploymentSvc, nil, nil, nil, &BootstrapOrchestrator{}, newFakeDeploymentBindingStoreForAssistant(map[string][]models.DeploymentBinding{}))
	session, _ := svc.CreateSession(CreateAssistantSessionCommand{UserID: "usr_123", ProjectID: "prj_123"})
	conversation, err := svc.PostMessage(AssistantMessageCommand{UserID: "usr_123", Role: RoleOperator, SessionID: session.ID, Content: "deployment status"})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if len(conversation.Messages) == 0 || conversation.Messages[len(conversation.Messages)-1].Content == "" {
		t.Fatalf("expected deployment status summary")
	}
}
