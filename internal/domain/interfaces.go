package domain

import "context"

type GitlabClient interface {
	// checks if the token has enough permissions
	CheckPermissions(ctx context.Context) error

	// returns list of repositories, including repositories in subgroups
	GetRepositoriesList(ctx context.Context, repoURL string) ([]*Repository, error)

	// returns list of filepaths in the repository
	GetFilesList(ctx context.Context, repoURL string) ([]string, error)

	// returns the content of the file
	GetFileContent(ctx context.Context, repoURL string, filePath string) ([]byte, error)
}

type RepositoryScanner interface {
	// detects projects in the repository, scanning for dependency files with
	DetectProjects(ctx context.Context, repo *Repository) ([]*Project, error)
}

type DependencyParser interface {
	// parses a dependency file and extracts dependencies
	ParseFile(ctx context.Context, file *DependencyFile) ([]*Dependency, error)
}

type DependencyClassifier interface {
	// classifies a list of dependencies
	ClassifyDependencies(ctx context.Context, dependencies []*Dependency) ([]*Dependency, error)
	// checks if a single dependency is internal
	IsInternal(ctx context.Context, dependency *Dependency) bool
}

type ReportGenerator interface {
	// generates an HTML report from projects
	GenerateHTML(ctx context.Context, projects []*Project) error
	// generates a CSV report from projects
	GenerateCSV(ctx context.Context, projects []*Project) error
	// generates a JSON report from projects
	GenerateJSON(ctx context.Context, projects []*Project) error
}
