package service

import (
	"errors"
	"strings"
	"unicode"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var ErrProjectSlugExists = errors.New("project slug already exists")

type ProjectService struct {
	projects ProjectStore
}

func NewProjectService(projects ProjectStore) *ProjectService {
	return &ProjectService{projects: projects}
}

func (s *ProjectService) Create(cmd CreateProjectCommand) (*ProjectSummary, error) {
	userID := strings.TrimSpace(cmd.UserID)
	name := utils.NormalizeSpace(cmd.Name)
	if userID == "" || name == "" {
		return nil, ErrInvalidInput
	}

	slugSource := cmd.Slug
	if strings.TrimSpace(slugSource) == "" {
		slugSource = name
	}
	slug := normalizeProjectSlug(slugSource)
	if slug == "" {
		return nil, ErrInvalidInput
	}

	defaultBranch, err := normalizeDefaultBranch(cmd.DefaultBranch)
	if err != nil {
		return nil, err
	}

	existing, err := s.projects.GetBySlugForUser(userID, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrProjectSlugExists
	}

	project := &models.Project{
		ID:            utils.NewPrefixedID("prj"),
		UserID:        userID,
		Name:          name,
		Slug:          slug,
		DefaultBranch: defaultBranch,
	}
	if err := s.projects.Create(project); err != nil {
		return nil, err
	}

	summary := ToProjectSummary(*project)
	return &summary, nil
}

func (s *ProjectService) List(userID string) ([]ProjectSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}

	projects, err := s.projects.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	items := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		items = append(items, ToProjectSummary(project))
	}

	return items, nil
}

func ToProjectSummary(project models.Project) ProjectSummary {
	return ProjectSummary{
		ID:            project.ID,
		Name:          project.Name,
		Slug:          project.Slug,
		DefaultBranch: project.DefaultBranch,
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.UpdatedAt,
	}
}

func normalizeDefaultBranch(branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main", nil
	}
	if strings.ContainsAny(branch, " \t\r\n") {
		return "", ErrInvalidInput
	}

	return branch, nil
}

func normalizeProjectSlug(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(input))
	lastHyphen := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		default:
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}

	return slug
}
