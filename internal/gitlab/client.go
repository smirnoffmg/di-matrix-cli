package gitlab

import (
	"context"
	"di-matrix-cli/internal/domain"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

// Client handles GitLab API operations
type Client struct {
	baseURL string
	token   string
	client  *gitlab.Client
	logger  *zap.Logger
}

// NewClient creates a new GitLab client
func NewClient(baseURL, token string, logger *zap.Logger) (*Client, error) {
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		client:  client,
		logger:  logger,
	}, nil
}

// GetRepository retrieves a repository by URL or ID
func (c *Client) GetRepository(ctx context.Context, identifier string) (*domain.Repository, error) {
	c.logger.Debug("Starting GetRepository", zap.String("identifier", identifier))

	// Extract project path from URL
	projectPath, err := c.ExtractProjectPath(identifier)
	if err != nil {
		c.logger.Error("Failed to extract project path",
			zap.String("identifier", identifier),
			zap.Error(err))
		return nil, fmt.Errorf("failed to extract project path from URL %s: %w", identifier, err)
	}
	c.logger.Debug("Extracted project path", zap.String("project_path", projectPath))

	// Get project from GitLab API
	c.logger.Debug("Calling GitLab API to get project", zap.String("project_path", projectPath))
	project, _, err := c.client.Projects.GetProject(projectPath, nil, gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to get project from API",
			zap.String("project_path", projectPath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get project %s: %w", projectPath, err)
	}
	c.logger.Debug("Retrieved project from API",
		zap.String("project_name", project.Name),
		zap.Int("project_id", project.ID))

	// Convert to domain.Repository
	repo := &domain.Repository{
		ID:            project.ID,
		Name:          project.Name,
		URL:           project.WebURL,
		DefaultBranch: project.DefaultBranch,
		WebURL:        project.WebURL,
	}

	c.logger.Debug("Completed GetRepository", zap.String("project_name", repo.Name))

	return repo, nil
}

// CheckPermissions verifies if the token has sufficient permissions
func (c *Client) CheckPermissions(ctx context.Context) error {
	c.logger.Debug("Starting CheckPermissions")

	// Try to get current user to verify token permissions
	c.logger.Debug("Calling GitLab API to verify token permissions")
	user, _, err := c.client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to verify token permissions", zap.Error(err))
		return fmt.Errorf("failed to verify token permissions: %w", err)
	}

	c.logger.Debug("Successfully verified token permissions",
		zap.String("username", user.Username),
		zap.Int("user_id", user.ID))

	return nil
}

// GetRepositoriesList returns a list of repositories from a group or project URL
func (c *Client) GetRepositoriesList(ctx context.Context, repoURL string) ([]*domain.Repository, error) {
	c.logger.Debug("Starting GetRepositoriesList", zap.String("repo_url", repoURL))

	// Extract path from URL to determine if it's a group or project
	path, err := c.ExtractProjectPath(repoURL)
	if err != nil {
		c.logger.Error("Failed to extract path from URL",
			zap.String("repo_url", repoURL),
			zap.Error(err))
		return nil, fmt.Errorf("failed to extract path from URL %s: %w", repoURL, err)
	}
	c.logger.Debug("Extracted path from URL", zap.String("path", path))

	// Check if it's a group by trying to get group info first
	c.logger.Debug("Checking if path is a group", zap.String("path", path))
	group, _, err := c.client.Groups.GetGroup(path, nil, gitlab.WithContext(ctx))
	if err == nil {
		c.logger.Debug("Path is a group, fetching group projects",
			zap.String("group_name", group.Name),
			zap.Int("group_id", group.ID))
		// It's a group, get all projects in the group
		return c.getGroupProjects(ctx, group.ID)
	}
	c.logger.Debug("Path is not a group, trying as single project", zap.String("path", path))

	// If not a group, try to get as a single project
	c.logger.Debug("Calling GitLab API to get single project", zap.String("path", path))
	project, _, err := c.client.Projects.GetProject(path, nil, gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to get project or group",
			zap.String("path", path),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get project or group %s: %w", path, err)
	}

	c.logger.Debug("Retrieved single project",
		zap.String("project_name", project.Name),
		zap.Int("project_id", project.ID))

	// Convert single project to repository list
	repo := &domain.Repository{
		ID:            project.ID,
		Name:          project.Name,
		URL:           project.WebURL,
		DefaultBranch: project.DefaultBranch,
		WebURL:        project.WebURL,
	}

	c.logger.Debug("Completed GetRepositoriesList for single project",
		zap.String("project_name", repo.Name))

	return []*domain.Repository{repo}, nil
}

// GetFilesList returns a list of file paths in the repository
func (c *Client) GetFilesList(ctx context.Context, repoURL string) ([]string, error) {
	c.logger.Debug("Starting GetFilesList", zap.String("repo_url", repoURL))

	// Extract project path from URL
	projectPath, err := c.ExtractProjectPath(repoURL)
	if err != nil {
		c.logger.Error("Failed to extract project path",
			zap.String("repo_url", repoURL),
			zap.Error(err))
		return nil, fmt.Errorf("failed to extract project path from URL %s: %w", repoURL, err)
	}
	c.logger.Debug("Extracted project path", zap.String("project_path", projectPath))

	// Get project to determine default branch
	c.logger.Debug("Getting project info to determine default branch", zap.String("project_path", projectPath))
	project, _, err := c.client.Projects.GetProject(projectPath, nil, gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to get project",
			zap.String("project_path", projectPath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get project %s: %w", projectPath, err)
	}
	c.logger.Debug("Retrieved project info",
		zap.String("project_name", project.Name),
		zap.String("default_branch", project.DefaultBranch))

	// Get repository tree with pagination
	c.logger.Debug("Starting repository tree traversal",
		zap.String("project_path", projectPath),
		zap.String("default_branch", project.DefaultBranch))

	var allFiles []string
	page := 1
	perPage := 100

	for {
		c.logger.Debug("Fetching repository tree page",
			zap.String("project_path", projectPath),
			zap.Int("page", page),
			zap.Int("per_page", perPage))

		tree, _, err := c.client.Repositories.ListTree(projectPath, &gitlab.ListTreeOptions{
			Recursive: gitlab.Ptr(true),
			Ref:       gitlab.Ptr(project.DefaultBranch),
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: perPage,
			},
		}, gitlab.WithContext(ctx))
		if err != nil {
			c.logger.Error("Failed to get repository tree",
				zap.String("project_path", projectPath),
				zap.Int("page", page),
				zap.Error(err))
			return nil, fmt.Errorf("failed to get repository tree for %s: %w", projectPath, err)
		}

		// Extract file paths (exclude directories)
		filesInPage := 0
		for _, item := range tree {
			if item.Type == "blob" { // blob = file, tree = directory
				allFiles = append(allFiles, item.Path)
				filesInPage++
			}
		}

		c.logger.Debug("Processed repository tree page",
			zap.String("project_path", projectPath),
			zap.Int("page", page),
			zap.Int("total_items", len(tree)),
			zap.Int("files_in_page", filesInPage),
			zap.Int("total_files_so_far", len(allFiles)))

		// If we got fewer items than requested, we've reached the end
		if len(tree) < perPage {
			c.logger.Debug("Reached end of repository tree",
				zap.String("project_path", projectPath),
				zap.Int("total_pages", page),
				zap.Int("total_files", len(allFiles)))
			break
		}

		page++
	}

	c.logger.Debug("Completed GetFilesList",
		zap.String("project_path", projectPath),
		zap.Int("total_files", len(allFiles)))

	return allFiles, nil
}

// GetFileContent returns the content of a specific file
func (c *Client) GetFileContent(ctx context.Context, repoURL, filePath string) ([]byte, error) {
	c.logger.Debug("Starting GetFileContent",
		zap.String("repo_url", repoURL),
		zap.String("file_path", filePath))

	// Extract project path from URL
	projectPath, err := c.ExtractProjectPath(repoURL)
	if err != nil {
		c.logger.Error("Failed to extract project path",
			zap.String("repo_url", repoURL),
			zap.Error(err))
		return nil, fmt.Errorf("failed to extract project path from URL %s: %w", repoURL, err)
	}
	c.logger.Debug("Extracted project path", zap.String("project_path", projectPath))

	// Get project to determine default branch
	c.logger.Debug("Getting project info for file access", zap.String("project_path", projectPath))
	project, _, err := c.client.Projects.GetProject(projectPath, nil, gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to get project",
			zap.String("project_path", projectPath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get project %s: %w", projectPath, err)
	}
	c.logger.Debug("Retrieved project info",
		zap.String("project_name", project.Name),
		zap.String("default_branch", project.DefaultBranch))

	// Get file content
	c.logger.Debug("Fetching file content",
		zap.String("project_path", projectPath),
		zap.String("file_path", filePath),
		zap.String("ref", project.DefaultBranch))

	file, _, err := c.client.RepositoryFiles.GetFile(projectPath, filePath, &gitlab.GetFileOptions{
		Ref: gitlab.Ptr(project.DefaultBranch),
	}, gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to get file content",
			zap.String("project_path", projectPath),
			zap.String("file_path", filePath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get file %s from project %s: %w", filePath, projectPath, err)
	}

	c.logger.Debug("Retrieved file content",
		zap.String("project_path", projectPath),
		zap.String("file_path", filePath),
		zap.String("file_encoding", file.Encoding),
		zap.Int("content_size_bytes", len(file.Content)))

	// Decode base64 content
	c.logger.Debug("Decoding base64 content",
		zap.String("file_path", filePath),
		zap.String("encoding", file.Encoding))

	content, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		c.logger.Error("Failed to decode file content",
			zap.String("file_path", filePath),
			zap.String("encoding", file.Encoding),
			zap.Error(err))
		return nil, fmt.Errorf("failed to decode file content for %s: %w", filePath, err)
	}

	c.logger.Debug("Completed GetFileContent",
		zap.String("file_path", filePath),
		zap.Int("decoded_content_size_bytes", len(content)))

	return content, nil
}

// getGroupProjects retrieves all projects within a group and its subgroups using concurrent pagination
func (c *Client) getGroupProjects(ctx context.Context, groupID int) ([]*domain.Repository, error) {
	c.logger.Debug("Starting getGroupProjects", zap.Int("group_id", groupID))

	// First, get the first page to determine total pages
	perPage := 100
	c.logger.Debug("Fetching first page to determine pagination",
		zap.Int("group_id", groupID),
		zap.Int("per_page", perPage))

	firstPage, resp, err := c.client.Groups.ListGroupProjects(groupID, &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: perPage,
		},
		IncludeSubGroups: gitlab.Ptr(true),
	}, gitlab.WithContext(ctx))
	if err != nil {
		c.logger.Error("Failed to get first page of projects",
			zap.Int("group_id", groupID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get first page of projects for group %d: %w", groupID, err)
	}

	c.logger.Debug("Retrieved first page",
		zap.Int("group_id", groupID),
		zap.Int("projects_in_first_page", len(firstPage)),
		zap.Int("total_projects", resp.TotalItems),
		zap.Int("total_pages", resp.TotalPages))

	// If we got fewer projects than requested, we only have one page
	if len(firstPage) < perPage {
		c.logger.Debug("Single page detected, returning results",
			zap.Int("group_id", groupID),
			zap.Int("total_projects", len(firstPage)))
		return c.ConvertProjectsToRepositories(firstPage), nil
	}

	// Calculate total pages from response headers
	totalPages := resp.TotalPages
	if totalPages <= 1 {
		c.logger.Debug("Only one page total, returning results",
			zap.Int("group_id", groupID),
			zap.Int("total_projects", len(firstPage)))
		return c.ConvertProjectsToRepositories(firstPage), nil
	}

	c.logger.Debug("Multi-page group detected, starting concurrent fetch",
		zap.Int("group_id", groupID),
		zap.Int("total_pages", totalPages),
		zap.Int("per_page", perPage),
		zap.Int("total_projects", resp.TotalItems))

	// Use worker pool pattern for concurrent pagination
	const maxWorkers = 5                     // Limit concurrent requests to avoid overwhelming the API
	pageChan := make(chan int, totalPages-1) // Channel for page numbers (skip page 1, already fetched)
	resultChan := make(chan []*domain.Repository, totalPages-1)
	errorChan := make(chan error, totalPages-1)

	// Send page numbers to workers (skip page 1)
	for page := 2; page <= totalPages; page++ {
		pageChan <- page
	}
	close(pageChan)

	// Start workers
	c.logger.Debug("Starting worker pool",
		zap.Int("group_id", groupID),
		zap.Int("max_workers", maxWorkers),
		zap.Int("pages_to_fetch", totalPages-1))

	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.logger.Debug("Started project worker",
				zap.Int("group_id", groupID),
				zap.Int("worker_id", workerID))

			for page := range pageChan {
				c.logger.Debug("Worker processing page",
					zap.Int("group_id", groupID),
					zap.Int("worker_id", workerID),
					zap.Int("page", page))

				select {
				case <-ctx.Done():
					c.logger.Debug("Worker cancelled due to context",
						zap.Int("group_id", groupID),
						zap.Int("worker_id", workerID),
						zap.Int("page", page))
					errorChan <- ctx.Err()
					return
				default:
					projects, _, err := c.client.Groups.ListGroupProjects(groupID, &gitlab.ListGroupProjectsOptions{
						ListOptions: gitlab.ListOptions{
							Page:    page,
							PerPage: perPage,
						},
						IncludeSubGroups: gitlab.Ptr(true),
					}, gitlab.WithContext(ctx))
					if err != nil {
						c.logger.Error("Worker failed to get page",
							zap.Int("group_id", groupID),
							zap.Int("worker_id", workerID),
							zap.Int("page", page),
							zap.Error(err))
						errorChan <- fmt.Errorf("failed to get page %d for group %d: %w", page, groupID, err)
						return
					}

					c.logger.Debug("Worker completed page",
						zap.Int("group_id", groupID),
						zap.Int("worker_id", workerID),
						zap.Int("page", page),
						zap.Int("projects_count", len(projects)))

					resultChan <- c.ConvertProjectsToRepositories(projects)
				}
			}

			c.logger.Debug("Worker finished",
				zap.Int("group_id", groupID),
				zap.Int("worker_id", workerID))
		}(i)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	c.logger.Debug("Starting result collection",
		zap.Int("group_id", groupID),
		zap.Int("expected_results", totalPages-1))

	var allRepos []*domain.Repository
	allRepos = append(allRepos, c.ConvertProjectsToRepositories(firstPage)...) // Add first page results

	c.logger.Debug("Added first page results",
		zap.Int("group_id", groupID),
		zap.Int("first_page_repos", len(firstPage)),
		zap.Int("total_repos_so_far", len(allRepos)))

	// Collect results from workers
	collectedPages := 0
	for i := 0; i < totalPages-1; i++ {
		select {
		case <-ctx.Done():
			c.logger.Debug("Result collection cancelled",
				zap.Int("group_id", groupID),
				zap.Int("collected_pages", collectedPages))
			return nil, ctx.Err()
		case err := <-errorChan:
			c.logger.Error("Error during result collection",
				zap.Int("group_id", groupID),
				zap.Int("collected_pages", collectedPages),
				zap.Error(err))
			return nil, err
		case repos := <-resultChan:
			collectedPages++
			allRepos = append(allRepos, repos...)
			c.logger.Debug("Collected page results",
				zap.Int("group_id", groupID),
				zap.Int("page_number", collectedPages+1), // +1 because we start from page 2
				zap.Int("repos_in_page", len(repos)),
				zap.Int("total_repos_so_far", len(allRepos)))
		}
	}

	c.logger.Debug("Completed concurrent project fetch",
		zap.Int("group_id", groupID),
		zap.Int("total_pages_processed", totalPages),
		zap.Int("total_repositories", len(allRepos)))

	return allRepos, nil
}

// ConvertProjectsToRepositories converts GitLab projects to domain repositories
func (c *Client) ConvertProjectsToRepositories(projects []*gitlab.Project) []*domain.Repository {
	repos := make([]*domain.Repository, 0, len(projects))
	for _, project := range projects {
		repos = append(repos, &domain.Repository{
			ID:            project.ID,
			Name:          project.Name,
			URL:           project.WebURL,
			DefaultBranch: project.DefaultBranch,
			WebURL:        project.WebURL,
		})
	}
	return repos
}

// ExtractProjectPath extracts the project path from a GitLab URL
func (c *Client) ExtractProjectPath(gitlabURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		return "", err
	}

	// Extract path segments after the base URL and remove trailing slash
	path := strings.TrimPrefix(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "", fmt.Errorf("no path found in URL: %s", gitlabURL)
	}

	// Decode the path to handle any existing URL encoding
	// This prevents double-encoding when the GitLab API client encodes it again
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		// If decoding fails, use the original path
		decodedPath = path
	}

	return decodedPath, nil
}
