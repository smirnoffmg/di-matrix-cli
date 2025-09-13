package classifier_test

import (
	"context"
	"di-matrix-cli/internal/classifier"
	"di-matrix-cli/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClassifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		internalPatterns []string
	}{
		{
			name:             "empty patterns",
			internalPatterns: []string{},
		},
		{
			name:             "single pattern",
			internalPatterns: []string{"github.com/company/*"},
		},
		{
			name:             "multiple patterns",
			internalPatterns: []string{"github.com/company/*", "gitlab.com/company/*", "@company/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := classifier.NewClassifier(tt.internalPatterns)
			assert.NotNil(t, result)
			// Test that the classifier can be used by checking IsInternal behavior
			ctx := context.Background()
			dependency := &domain.Dependency{
				Name:      "github.com/company/test",
				Version:   "v1.0.0",
				Ecosystem: "go-modules",
			}
			// Should return true if patterns contain "github.com/company/*"
			expected := len(tt.internalPatterns) > 0 && tt.internalPatterns[0] == "github.com/company/*"
			assert.Equal(t, expected, result.IsInternal(ctx, dependency))
		})
	}
}

func TestClassifier_IsInternal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name             string
		internalPatterns []string
		dependency       *domain.Dependency
		expected         bool
	}{
		{
			name:             "no patterns - external",
			internalPatterns: []string{},
			dependency: &domain.Dependency{
				Name:      "github.com/gin-gonic/gin",
				Version:   "v1.9.1",
				Ecosystem: "go-modules",
			},
			expected: false,
		},
		{
			name:             "exact match - internal",
			internalPatterns: []string{"github.com/company/service"},
			dependency: &domain.Dependency{
				Name:      "github.com/company/service",
				Version:   "v1.0.0",
				Ecosystem: "go-modules",
			},
			expected: true,
		},
		{
			name:             "wildcard match - internal",
			internalPatterns: []string{"github.com/company/*"},
			dependency: &domain.Dependency{
				Name:      "github.com/company/user-service",
				Version:   "v2.1.0",
				Ecosystem: "go-modules",
			},
			expected: true,
		},
		{
			name:             "wildcard match - external",
			internalPatterns: []string{"github.com/company/*"},
			dependency: &domain.Dependency{
				Name:      "github.com/gin-gonic/gin",
				Version:   "v1.9.1",
				Ecosystem: "go-modules",
			},
			expected: false,
		},
		{
			name:             "npm scoped package - internal",
			internalPatterns: []string{"@company/*"},
			dependency: &domain.Dependency{
				Name:      "@company/ui-components",
				Version:   "1.2.3",
				Ecosystem: "npm",
			},
			expected: true,
		},
		{
			name:             "npm scoped package - external",
			internalPatterns: []string{"@company/*"},
			dependency: &domain.Dependency{
				Name:      "@angular/core",
				Version:   "15.0.0",
				Ecosystem: "npm",
			},
			expected: false,
		},
		{
			name:             "java package - internal",
			internalPatterns: []string{"com.company.*"},
			dependency: &domain.Dependency{
				Name:      "com.company.user.service",
				Version:   "1.0.0",
				Ecosystem: "maven",
			},
			expected: true,
		},
		{
			name:             "java package - external",
			internalPatterns: []string{"com.company.*"},
			dependency: &domain.Dependency{
				Name:      "org.springframework.boot",
				Version:   "2.7.0",
				Ecosystem: "maven",
			},
			expected: false,
		},
		{
			name:             "python package - internal",
			internalPatterns: []string{"company-*"},
			dependency: &domain.Dependency{
				Name:      "company-utils",
				Version:   "1.0.0",
				Ecosystem: "pypi",
			},
			expected: true,
		},
		{
			name:             "python package - external",
			internalPatterns: []string{"company-*"},
			dependency: &domain.Dependency{
				Name:      "requests",
				Version:   "2.28.0",
				Ecosystem: "pypi",
			},
			expected: false,
		},
		{
			name:             "multiple patterns - first match",
			internalPatterns: []string{"github.com/company/*", "gitlab.com/company/*"},
			dependency: &domain.Dependency{
				Name:      "github.com/company/auth-service",
				Version:   "v1.0.0",
				Ecosystem: "go-modules",
			},
			expected: true,
		},
		{
			name:             "multiple patterns - second match",
			internalPatterns: []string{"github.com/company/*", "gitlab.com/company/*"},
			dependency: &domain.Dependency{
				Name:      "gitlab.com/company/notification-service",
				Version:   "v2.0.0",
				Ecosystem: "go-modules",
			},
			expected: true,
		},
		{
			name:             "multiple patterns - no match",
			internalPatterns: []string{"github.com/company/*", "gitlab.com/company/*"},
			dependency: &domain.Dependency{
				Name:      "github.com/gin-gonic/gin",
				Version:   "v1.9.1",
				Ecosystem: "go-modules",
			},
			expected: false,
		},
		{
			name:             "case sensitive match",
			internalPatterns: []string{"github.com/Company/*"},
			dependency: &domain.Dependency{
				Name:      "github.com/company/service",
				Version:   "v1.0.0",
				Ecosystem: "go-modules",
			},
			expected: false,
		},
		{
			name:             "empty dependency name",
			internalPatterns: []string{"github.com/company/*"},
			dependency: &domain.Dependency{
				Name:      "",
				Version:   "v1.0.0",
				Ecosystem: "go-modules",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			classifierInstance := classifier.NewClassifier(tt.internalPatterns)
			result := classifierInstance.IsInternal(ctx, tt.dependency)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClassifier_ClassifyDependencies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name             string
		internalPatterns []string
		dependencies     []*domain.Dependency
		expected         []*domain.Dependency
		expectedError    bool
	}{
		{
			name:             "empty dependencies list",
			internalPatterns: []string{"github.com/company/*"},
			dependencies:     []*domain.Dependency{},
			expected:         []*domain.Dependency{},
			expectedError:    false,
		},
		{
			name:             "nil dependencies list",
			internalPatterns: []string{"github.com/company/*"},
			dependencies:     nil,
			expected:         nil,
			expectedError:    false,
		},
		{
			name:             "single internal dependency",
			internalPatterns: []string{"github.com/company/*"},
			dependencies: []*domain.Dependency{
				{
					Name:       "github.com/company/user-service",
					Version:    "v1.0.0",
					Ecosystem:  "go-modules",
					IsInternal: false, // Will be set by classifier
				},
			},
			expected: []*domain.Dependency{
				{
					Name:       "github.com/company/user-service",
					Version:    "v1.0.0",
					Ecosystem:  "go-modules",
					IsInternal: true,
				},
			},
			expectedError: false,
		},
		{
			name:             "single external dependency",
			internalPatterns: []string{"github.com/company/*"},
			dependencies: []*domain.Dependency{
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
			},
			expected: []*domain.Dependency{
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
			},
			expectedError: false,
		},
		{
			name:             "mixed internal and external dependencies",
			internalPatterns: []string{"github.com/company/*", "@company/*"},
			dependencies: []*domain.Dependency{
				{
					Name:       "github.com/company/user-service",
					Version:    "v1.0.0",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
				{
					Name:       "@company/ui-components",
					Version:    "1.2.3",
					Ecosystem:  "npm",
					IsInternal: false,
				},
				{
					Name:       "react",
					Version:    "18.0.0",
					Ecosystem:  "npm",
					IsInternal: false,
				},
			},
			expected: []*domain.Dependency{
				{
					Name:       "github.com/company/user-service",
					Version:    "v1.0.0",
					Ecosystem:  "go-modules",
					IsInternal: true,
				},
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
				{
					Name:       "@company/ui-components",
					Version:    "1.2.3",
					Ecosystem:  "npm",
					IsInternal: true,
				},
				{
					Name:       "react",
					Version:    "18.0.0",
					Ecosystem:  "npm",
					IsInternal: false,
				},
			},
			expectedError: false,
		},
		{
			name:             "dependencies with nil elements",
			internalPatterns: []string{"github.com/company/*"},
			dependencies: []*domain.Dependency{
				{
					Name:       "github.com/company/user-service",
					Version:    "v1.0.0",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
				nil,
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
			},
			expected: []*domain.Dependency{
				{
					Name:       "github.com/company/user-service",
					Version:    "v1.0.0",
					Ecosystem:  "go-modules",
					IsInternal: true,
				},
				nil,
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Ecosystem:  "go-modules",
					IsInternal: false,
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			classifierInstance := classifier.NewClassifier(tt.internalPatterns)
			result, err := classifierInstance.ClassifyDependencies(ctx, tt.dependencies)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClassifier_EdgeCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("very long dependency name", func(t *testing.T) {
		t.Parallel()
		classifierInstance := classifier.NewClassifier([]string{"github.com/company/*"})
		longName := "github.com/company/" + string(make([]byte, 1000))
		dependency := &domain.Dependency{
			Name:      longName,
			Version:   "v1.0.0",
			Ecosystem: "go-modules",
		}

		result := classifierInstance.IsInternal(ctx, dependency)
		assert.True(t, result)
	})

	t.Run("special characters in pattern", func(t *testing.T) {
		t.Parallel()
		classifierInstance := classifier.NewClassifier([]string{"github.com/company-*/*"})
		dependency := &domain.Dependency{
			Name:      "github.com/company-123/service",
			Version:   "v1.0.0",
			Ecosystem: "go-modules",
		}

		result := classifierInstance.IsInternal(ctx, dependency)
		assert.True(t, result)
	})

	t.Run("unicode characters", func(t *testing.T) {
		t.Parallel()
		classifierInstance := classifier.NewClassifier([]string{"github.com/公司/*"})
		dependency := &domain.Dependency{
			Name:      "github.com/公司/service",
			Version:   "v1.0.0",
			Ecosystem: "go-modules",
		}

		result := classifierInstance.IsInternal(ctx, dependency)
		assert.True(t, result)
	})
}
