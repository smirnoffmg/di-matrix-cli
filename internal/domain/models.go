package domain

import "time"

type Repository struct {
	ID            int    `json:"id"`             // GitLab project ID
	Name          string `json:"name"`           // "user-service"
	URL           string `json:"url"`            // GitLab project URL
	DefaultBranch string `json:"default_branch"` // "main"
	WebURL        string `json:"web_url"`        // Browser URL
}

type Project struct {
	ID              string            `json:"id"`         // Generated: "repo-123-backend-go"
	Name            string            `json:"name"`       // "User Service Backend"
	Repository      Repository        `json:"repository"` // Parent repository
	Path            string            `json:"path"`       // "backend/" or "" for root
	Language        string            `json:"language"`   // "go", "nodejs", "java", "python"
	DependencyFiles []*DependencyFile `json:"dependency_files"`
	Dependencies    []*Dependency     `json:"dependencies"`
}

type DependencyFile struct {
	Path         string    `json:"path"`     // "backend/go.mod"
	Language     string    `json:"language"` // "go"
	Content      []byte    `json:"content"`  // Raw file content
	LastModified time.Time `json:"last_modified"`
}

type Dependency struct {
	Name       string `json:"name"`        // "github.com/gin-gonic/gin"
	Version    string `json:"version"`     // "v1.9.1"
	Constraint string `json:"constraint"`  // "^1.9.0"
	MinVersion string `json:"min_version"` // "1.9.0"
	MaxVersion string `json:"max_version"` // "2.0.0"
	IsInternal bool   `json:"is_internal"` // true/false
	Ecosystem  string `json:"ecosystem"`   // "go-modules", "npm", "maven"
}
