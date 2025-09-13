package scanner

import (
	"context"
	"di-matrix-cli/internal/domain"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"
)

// Scanner finds dependency files in repositories and detects projects
type Scanner struct {
	gitlabClient domain.GitlabClient
	logger       *zap.Logger
}

// NewScanner creates a new file scanner
func NewScanner(gitlabClient domain.GitlabClient, logger *zap.Logger) *Scanner {
	return &Scanner{
		gitlabClient: gitlabClient,
		logger:       logger,
	}
}

// CapitalizeFirst capitalizes the first letter of a string
func CapitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// DetectProjects detects projects within a repository by analyzing dependency files
func (s *Scanner) DetectProjects(ctx context.Context, repo *domain.Repository) ([]*domain.Project, error) {
	s.logger.Info("Detecting projects in repository",
		zap.String("repo_name", repo.Name),
		zap.String("repo_url", repo.URL))

	// Get all files in the repository
	files, err := s.gitlabClient.GetFilesList(ctx, repo.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to get files list for repository %s: %w", repo.Name, err)
	}

	// Filter for dependency files
	dependencyFiles := s.filterDependencyFiles(files)
	if len(dependencyFiles) == 0 {
		s.logger.Info("No dependency files found in repository", zap.String("repo_name", repo.Name))
		return []*domain.Project{}, nil
	}

	// Group dependency files by project (language + path)
	projectGroups := s.groupDependencyFilesByProject(dependencyFiles)

	// Create projects from groups
	var projects []*domain.Project
	for _, group := range projectGroups {
		project, err := s.createProjectFromGroup(ctx, repo, group)
		if err != nil {
			s.logger.Error("Failed to create project from group",
				zap.String("repo_name", repo.Name),
				zap.Error(err))
			continue
		}
		// Only add projects that have at least one dependency file
		if len(project.DependencyFiles) > 0 {
			projects = append(projects, project)
		}
	}

	s.logger.Info("Detected projects in repository",
		zap.String("repo_name", repo.Name),
		zap.Int("project_count", len(projects)))

	return projects, nil
}

// filterDependencyFiles filters the file list to only include dependency files
func (s *Scanner) filterDependencyFiles(files []string) []string {
	var dependencyFiles []string
	supportedTypes := s.SupportedFileTypes()

	// Create a map for O(1) lookup instead of nested loops
	supportedMap := make(map[string]bool)
	for _, fileType := range supportedTypes {
		supportedMap[fileType] = true
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		if supportedMap[fileName] {
			dependencyFiles = append(dependencyFiles, file)
		}
	}

	return dependencyFiles
}

// dependencyFileGroup represents a group of dependency files that belong to the same project
type dependencyFileGroup struct {
	language string
	path     string
	files    []string
}

// groupDependencyFilesByProject groups dependency files by their project (language + path)
func (s *Scanner) groupDependencyFilesByProject(dependencyFiles []string) []dependencyFileGroup {
	projectMap := make(map[string]*dependencyFileGroup)

	for _, file := range dependencyFiles {
		language := s.DetectLanguageFromFile(file)
		projectPath := s.ExtractProjectPath(file)
		groupKey := fmt.Sprintf("%s:%s", language, projectPath)

		if group, exists := projectMap[groupKey]; exists {
			group.files = append(group.files, file)
		} else {
			projectMap[groupKey] = &dependencyFileGroup{
				language: language,
				path:     projectPath,
				files:    []string{file},
			}
		}
	}

	// Convert map to slice
	var groups []dependencyFileGroup
	for _, group := range projectMap {
		groups = append(groups, *group)
	}

	return groups
}

// DetectLanguageFromFile detects the programming language from a dependency file
func (s *Scanner) DetectLanguageFromFile(filePath string) string {
	fileName := strings.ToLower(filepath.Base(filePath))

	switch fileName {
	case "go.mod", "go.sum":
		return "go"
	case "package.json", "package-lock.json", "yarn.lock":
		return "nodejs"
	case "pom.xml", "build.gradle", "gradle.lockfile":
		return "java"
	case "requirements.txt", "pipfile", "poetry.lock", "uv.lock", "setup.py":
		return "python"
	default:
		return "unknown"
	}
}

// ExtractProjectPath extracts the project path from a file path
func (s *Scanner) ExtractProjectPath(filePath string) string {
	// Remove the dependency file name to get the directory path
	dir := filepath.Dir(filePath)

	// If it's in the root directory, return empty string
	if dir == "." || dir == "/" {
		return ""
	}

	return dir
}

// createProjectFromGroup creates a Project from a dependency file group
func (s *Scanner) createProjectFromGroup(
	ctx context.Context,
	repo *domain.Repository,
	group dependencyFileGroup,
) (*domain.Project, error) {
	// Generate project ID
	projectID := fmt.Sprintf("repo-%d-%s-%s", repo.ID, group.path, group.language)
	if group.path == "" {
		projectID = fmt.Sprintf("repo-%d-root-%s", repo.ID, group.language)
	}

	// Generate project name
	projectName := fmt.Sprintf("%s %s", repo.Name, CapitalizeFirst(group.language))
	if group.path != "" {
		projectName = fmt.Sprintf("%s %s (%s)", repo.Name, CapitalizeFirst(group.language), group.path)
	}

	// Create dependency files with content
	var dependencyFiles []*domain.DependencyFile
	for _, file := range group.files {
		content, err := s.gitlabClient.GetFileContent(ctx, repo.URL, file)
		if err != nil {
			s.logger.Error("Failed to get file content",
				zap.String("file", file),
				zap.Error(err))
			continue
		}

		dependencyFiles = append(dependencyFiles, &domain.DependencyFile{
			Path:         file,
			Language:     group.language,
			Content:      content,
			LastModified: time.Now(), // TODO: Get actual last modified time from GitLab API
		})
	}

	project := &domain.Project{
		ID:              projectID,
		Name:            projectName,
		Repository:      *repo,
		Path:            group.path,
		Language:        group.language,
		DependencyFiles: dependencyFiles,
		Dependencies:    []*domain.Dependency{}, // Will be populated by parser
	}

	return project, nil
}

// SupportedFileTypes returns the file types we can scan for
func (s *Scanner) SupportedFileTypes() []string {
	return []string{
		"go.mod", "go.sum",
		"package.json", "package-lock.json", "yarn.lock",
		"pom.xml", "build.gradle", "gradle.lockfile",
		"requirements.txt", "Pipfile", "poetry.lock", "uv.lock", "setup.py",
	}
}
