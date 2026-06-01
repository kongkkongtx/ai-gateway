package prompt

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TemplateVariable holds a named variable with a default value.
type TemplateVariable struct {
	Name    string `json:"name" yaml:"name"`
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	Desc    string `json:"desc,omitempty" yaml:"desc,omitempty"`
}

// TemplateVersion represents a versioned snapshot of a template.
type TemplateVersion struct {
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	Variables []TemplateVariable `json:"variables,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
	Comment   string    `json:"comment,omitempty"`
}

// Template is a named prompt template with versioning.
type Template struct {
	ID          string             `json:"id" yaml:"id"`
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Role        string             `json:"role,omitempty" yaml:"role,omitempty"`       // system, user, assistant
	RouteMatch  string             `json:"route_match,omitempty" yaml:"route_match,omitempty"` // glob pattern for routes
	Variables   []TemplateVariable `json:"variables,omitempty" yaml:"variables,omitempty"`
	CurrentVer  int                `json:"current_version" yaml:"current_version"`
	Versions    []TemplateVersion  `json:"versions,omitempty" yaml:"-"`
	CreatedAt   time.Time          `json:"created_at" yaml:"-"`
	UpdatedAt   time.Time          `json:"updated_at" yaml:"-"`
}

// varPattern matches {{variable_name}} or {{ variable_name }} placeholders.
// Config defines prompt template configuration.
type Config struct {
	Enabled bool `yaml:"enabled"`
}

var varPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// Render applies variable values to the template content.
// Returns the rendered string and any unused variables.
func (t *Template) Render(vars map[string]string) (string, error) {
	if len(t.Versions) == 0 {
		return "", fmt.Errorf("template %q has no versions", t.ID)
	}
	latest := t.Versions[len(t.Versions)-1]
	content := latest.Content

	// Build variable map: provided values override defaults
	values := make(map[string]string)
	for _, v := range latest.Variables {
		values[v.Name] = v.Default
	}
	for k, v := range vars {
		if _, ok := values[k]; ok {
			values[k] = v
		}
	}

	// Replace variables
	var errs []string
	result := varPattern.ReplaceAllStringFunc(content, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := values[name]; ok {
			return val
		}
		errs = append(errs, name)
		return match
	})

	if len(errs) > 0 {
		return result, fmt.Errorf("undefined variables: %s", strings.Join(errs, ", "))
	}
	return result, nil
}

// AddVersion creates a new version of the template.
func (t *Template) AddVersion(content string, variables []TemplateVariable, createdBy, comment string) {
	ver := TemplateVersion{
		Version:   t.CurrentVer + 1,
		Content:   content,
		Variables: variables,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		Comment:   comment,
	}
	t.Versions = append(t.Versions, ver)
	t.CurrentVer = ver.Version
	t.UpdatedAt = time.Now()
}

// Manager manages a collection of prompt templates.
type Manager struct {
	mu        sync.RWMutex
	templates map[string]*Template // keyed by ID
}

// NewManager creates a new prompt template manager.
func NewManager() *Manager {
	return &Manager{
		templates: make(map[string]*Template),
	}
}

// Get returns a template by ID.
func (m *Manager) Get(id string) (*Template, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[id]
	return t, ok
}

// List returns all templates.
func (m *Manager) List() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Template, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result
}

// Save creates or updates a template. If the ID is empty, a new one is generated.
func (m *Manager) Save(t *Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t.ID == "" {
		t.ID = fmt.Sprintf("tmpl-%d", time.Now().UnixNano())
	}
	now := time.Now()
	if existing, ok := m.templates[t.ID]; ok {
		t.CreatedAt = existing.CreatedAt
		t.CurrentVer = existing.CurrentVer
		t.Versions = existing.Versions
		t.UpdatedAt = now
	} else {
		t.CreatedAt = now
		t.UpdatedAt = now
		t.CurrentVer = 0
		t.Versions = nil
	}

	// If content is provided, add as a new version
	if t.CurrentVer == 0 && len(t.Versions) == 0 {
		// First save: create version 1 from initial content
		// (content should have been set via a separate call)
	}

	m.templates[t.ID] = t
	return nil
}

// Delete removes a template by ID.
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.templates[id]
	if ok {
		delete(m.templates, id)
	}
	return ok
}

// FindByRouteMatch returns templates whose RouteMatch pattern matches the given model.
func (m *Manager) FindByRouteMatch(model string) []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Template
	for _, t := range m.templates {
		if t.RouteMatch == "" {
			continue
		}
		if matched, _ := filepathMatch(t.RouteMatch, model); matched {
			result = append(result, t)
		}
	}
	return result
}

// filepathMatch is a simple glob matcher.
func filepathMatch(pattern, name string) (bool, error) {
	// Convert glob pattern to regex
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name, nil
	}
	var buf strings.Builder
	buf.WriteString("^")
	for i, part := range parts {
		if i > 0 {
			buf.WriteString(".*")
		}
		buf.WriteString(regexp.QuoteMeta(part))
	}
	buf.WriteString("$")
	return regexp.MatchString(buf.String(), name)
}
