package usecases

import (
	"context"
	"di-matrix-cli/internal/domain"
	"sync"

	"go.uber.org/zap"
)

const (
	// Default number of workers for concurrent project processing
	defaultProjectWorkers = 5
	// Default number of workers for concurrent dependency file processing per project
	defaultDependencyFileWorkers = 3
)

// AnalyzeResponse represents the result of the analysis
type AnalyzeResponse struct {
	TotalProjects     int `json:"total_projects"`
	TotalDependencies int `json:"total_dependencies"`
	InternalCount     int `json:"internal_count"`
	ExternalCount     int `json:"external_count"`
}

// AnalyzeUseCase orchestrates the dependency analysis workflow
type AnalyzeUseCase struct {
	gitlabClient domain.GitlabClient
	scanner      domain.RepositoryScanner
	parser       domain.DependencyParser
	classifier   domain.DependencyClassifier
	generator    domain.ReportGenerator
	logger       *zap.Logger
	ctx          context.Context
}

// NewAnalyzeUseCase creates a new analyze use case with dependency injection
func NewAnalyzeUseCase(
	ctx context.Context,
	gitlabClient domain.GitlabClient,
	scanner domain.RepositoryScanner,
	parser domain.DependencyParser,
	classifier domain.DependencyClassifier,
	generator domain.ReportGenerator,
	logger *zap.Logger,
) *AnalyzeUseCase {
	return &AnalyzeUseCase{
		gitlabClient: gitlabClient,
		scanner:      scanner,
		parser:       parser,
		classifier:   classifier,
		generator:    generator,
		logger:       logger,
		ctx:          ctx,
	}
}

// Execute runs the main dependency analysis workflow
func (uc *AnalyzeUseCase) Execute(repositoryURLs []string) (*AnalyzeResponse, error) {
	uc.logger.Info("Starting dependency analysis workflow")

	// Step 1: Get repositories from URLs (with concurrency)
	var repositories []*domain.Repository
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Channel to collect errors
	errChan := make(chan error, len(repositoryURLs))

	for _, repoURL := range repositoryURLs {
		wg.Add(1)
		go func(repoURL string) {
			defer wg.Done()

			repos, err := uc.gitlabClient.GetRepositoriesList(uc.ctx, repoURL)
			if err != nil {
				errChan <- err
				return
			}

			mu.Lock()
			repositories = append(repositories, repos...)
			mu.Unlock()
		}(repoURL)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	for _, repo := range repositories {
		uc.logger.Info("Found repository", zap.String("name", repo.Name), zap.String("url", repo.URL))
	}

	// Step 2: Transform repositories to projects (with concurrency)
	var allProjects []*domain.Project
	var projectsMu sync.Mutex
	var projectsWg sync.WaitGroup

	for _, repo := range repositories {
		projectsWg.Add(1)
		go func(repository *domain.Repository) {
			defer projectsWg.Done()

			projects, err := uc.scanner.DetectProjects(uc.ctx, repository)
			if err != nil {
				uc.logger.Error("Failed to detect projects in repository",
					zap.String("repo_name", repository.Name),
					zap.Error(err))
				return
			}

			projectsMu.Lock()
			allProjects = append(allProjects, projects...)
			projectsMu.Unlock()
		}(repo)
	}

	// Wait for all project detection goroutines to complete
	projectsWg.Wait()

	uc.logger.Info("Detected projects across all repositories",
		zap.Int("total_projects", len(allProjects)))

	for _, project := range allProjects {
		uc.logger.Info("Detected project",
			zap.String("project_id", project.ID),
			zap.String("project_name", project.Name),
			zap.String("language", project.Language),
			zap.String("path", project.Path),
			zap.Int("dependency_files", len(project.DependencyFiles)))
	}

	// Step 3: Parse dependency files and classify dependencies (with concurrency)
	totalDependencies, internalCount, externalCount, err := uc.processProjectsConcurrently(allProjects)
	if err != nil {
		uc.logger.Error("Failed to process projects concurrently", zap.Error(err))
		return nil, err
	}

	// Step 4: Generate HTML report with all results
	uc.logger.Info("Generating HTML report", zap.Int("projects_count", len(allProjects)))
	if err := uc.generator.GenerateHTML(uc.ctx, allProjects); err != nil {
		uc.logger.Error("Failed to generate HTML report", zap.Error(err))
		return nil, err
	}
	uc.logger.Info("HTML report generated successfully")

	// Step 5: Save report to output file (handled by generator)

	// Calculate response metrics
	response := &AnalyzeResponse{
		TotalProjects:     len(allProjects),
		TotalDependencies: totalDependencies,
		InternalCount:     internalCount,
		ExternalCount:     externalCount,
	}

	uc.logger.Info("Dependency analysis completed",
		zap.Int("total_projects", response.TotalProjects),
		zap.Int("total_dependencies", response.TotalDependencies),
		zap.Int("internal_count", response.InternalCount),
		zap.Int("external_count", response.ExternalCount))

	return response, nil
}

// processProjectsConcurrently processes all projects concurrently using worker pools
func (uc *AnalyzeUseCase) processProjectsConcurrently(projects []*domain.Project) (int, int, int, error) {
	uc.logger.Info("Starting concurrent project processing",
		zap.Int("total_projects", len(projects)),
		zap.Int("project_workers", defaultProjectWorkers))

	// Shared counters with mutex protection
	var totalDependencies int
	var internalCount int
	var externalCount int
	var mu sync.Mutex

	// Error collection
	var errors []error
	var errorMu sync.Mutex

	// Create project processing channel
	projectChan := make(chan *domain.Project, len(projects))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < defaultProjectWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			uc.logger.Debug("Started project worker", zap.Int("worker_id", workerID))

			for project := range projectChan {
				uc.logger.Debug("Processing project in worker",
					zap.Int("worker_id", workerID),
					zap.String("project_id", project.ID),
					zap.String("project_name", project.Name))

				projectDeps, projectInternal, projectExternal, err := uc.processProject(project)
				if err != nil {
					errorMu.Lock()
					errors = append(errors, err)
					errorMu.Unlock()
					uc.logger.Error("Failed to process project",
						zap.String("project_id", project.ID),
						zap.String("project_name", project.Name),
						zap.Error(err))
					continue
				}

				// Update shared counters
				mu.Lock()
				totalDependencies += projectDeps
				internalCount += projectInternal
				externalCount += projectExternal
				mu.Unlock()

				uc.logger.Info("Completed project processing",
					zap.String("project_id", project.ID),
					zap.String("project_name", project.Name),
					zap.Int("dependencies", projectDeps),
					zap.Int("internal", projectInternal),
					zap.Int("external", projectExternal))
			}

			uc.logger.Debug("Finished project worker", zap.Int("worker_id", workerID))
		}(i)
	}

	// Send projects to workers
	for _, project := range projects {
		projectChan <- project
	}
	close(projectChan)

	// Wait for all workers to complete
	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		uc.logger.Error("Some projects failed to process",
			zap.Int("error_count", len(errors)),
			zap.Int("successful_projects", len(projects)-len(errors)))
		// Continue with partial results rather than failing completely
	}

	uc.logger.Info("Completed concurrent project processing",
		zap.Int("total_dependencies", totalDependencies),
		zap.Int("internal_count", internalCount),
		zap.Int("external_count", externalCount),
		zap.Int("errors", len(errors)))

	return totalDependencies, internalCount, externalCount, nil
}

// processProject processes a single project's dependency files concurrently
func (uc *AnalyzeUseCase) processProject(project *domain.Project) (int, int, int, error) {
	uc.logger.Info("Parsing dependencies for project",
		zap.String("project_id", project.ID),
		zap.String("project_name", project.Name),
		zap.Int("dependency_files", len(project.DependencyFiles)))

	// Shared project-level data
	var projectDependencies []*domain.Dependency
	var projectMu sync.Mutex
	var projectInternal int
	var projectExternal int

	// Error collection for this project
	var projectErrors []error
	var projectErrorMu sync.Mutex

	// Create dependency file processing channel
	fileChan := make(chan *domain.DependencyFile, len(project.DependencyFiles))

	// Start worker goroutines for dependency files
	var fileWg sync.WaitGroup
	workers := defaultDependencyFileWorkers
	if len(project.DependencyFiles) < workers {
		workers = len(project.DependencyFiles)
	}

	for i := 0; i < workers; i++ {
		fileWg.Add(1)
		go func(workerID int) {
			defer fileWg.Done()

			for dependencyFile := range fileChan {
				uc.logger.Debug("Parsing dependency file in worker",
					zap.Int("worker_id", workerID),
					zap.String("file_path", dependencyFile.Path),
					zap.String("language", dependencyFile.Language))

				dependencies, err := uc.parser.ParseFile(uc.ctx, dependencyFile)
				if err != nil {
					projectErrorMu.Lock()
					projectErrors = append(projectErrors, err)
					projectErrorMu.Unlock()
					uc.logger.Error("Failed to parse dependency file",
						zap.String("file_path", dependencyFile.Path),
						zap.String("language", dependencyFile.Language),
						zap.Error(err))
					continue
				}

				// Classify dependencies concurrently
				classifiedDeps, internalCount, externalCount := uc.classifyDependenciesConcurrently(dependencies)

				// Update project-level data
				projectMu.Lock()
				projectDependencies = append(projectDependencies, classifiedDeps...)
				projectInternal += internalCount
				projectExternal += externalCount
				projectMu.Unlock()

				uc.logger.Debug("Parsed dependencies from file",
					zap.String("file_path", dependencyFile.Path),
					zap.Int("dependencies_count", len(dependencies)))
			}
		}(i)
	}

	// Send dependency files to workers
	for _, dependencyFile := range project.DependencyFiles {
		fileChan <- dependencyFile
	}
	close(fileChan)

	// Wait for all file workers to complete
	fileWg.Wait()

	// Update project with parsed dependencies
	project.Dependencies = projectDependencies

	// Log project errors but don't fail the entire project
	if len(projectErrors) > 0 {
		uc.logger.Warn("Some dependency files failed to parse in project",
			zap.String("project_id", project.ID),
			zap.String("project_name", project.Name),
			zap.Int("error_count", len(projectErrors)),
			zap.Int("successful_files", len(project.DependencyFiles)-len(projectErrors)))
	}

	uc.logger.Info("Completed dependency parsing for project",
		zap.String("project_id", project.ID),
		zap.String("project_name", project.Name),
		zap.Int("total_dependencies", len(projectDependencies)))

	return len(projectDependencies), projectInternal, projectExternal, nil
}

// classifyDependenciesConcurrently classifies dependencies as internal or external concurrently
func (uc *AnalyzeUseCase) classifyDependenciesConcurrently(
	dependencies []*domain.Dependency,
) ([]*domain.Dependency, int, int) {
	if len(dependencies) == 0 {
		return dependencies, 0, 0
	}

	// For small numbers of dependencies, process sequentially to avoid overhead
	if len(dependencies) <= 10 {
		var internalCount int
		var externalCount int
		for _, dep := range dependencies {
			dep.IsInternal = uc.classifier.IsInternal(uc.ctx, dep)
			if dep.IsInternal {
				internalCount++
			} else {
				externalCount++
			}
		}
		return dependencies, internalCount, externalCount
	}

	// For larger numbers, use concurrency
	var internalCount int
	var externalCount int
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, dep := range dependencies {
		wg.Add(1)
		go func(dependency *domain.Dependency) {
			defer wg.Done()

			isInternal := uc.classifier.IsInternal(uc.ctx, dependency)

			// Use mutex to protect both the dependency field and counters
			mu.Lock()
			dependency.IsInternal = isInternal
			if isInternal {
				internalCount++
			} else {
				externalCount++
			}
			mu.Unlock()
		}(dep)
	}

	wg.Wait()
	return dependencies, internalCount, externalCount
}
