package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a loaded skill with its metadata and content
type Skill struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Category    string   `json:"category" yaml:"category"`
	BugClass    string   `json:"bug_class" yaml:"bug_class"`
	Triggers    []string `json:"triggers" yaml:"triggers"`
	Content     string   `json:"content"` // The full markdown content
}

// SkillLoader loads skills from markdown files
type SkillLoader struct {
	skillsDir string
	skills    map[string]*Skill
}

// NewSkillLoader creates a new skill loader
func NewSkillLoader(skillsDir string) *SkillLoader {
	return &SkillLoader{
		skillsDir: skillsDir,
		skills:    make(map[string]*Skill),
	}
}

// LoadAll loads all skills from the skills directory
func (l *SkillLoader) LoadAll() error {
	return filepath.Walk(l.skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		
		skill, err := l.loadSkill(path)
		if err != nil {
			return fmt.Errorf("failed to load skill %s: %w", path, err)
		}
		
		l.skills[skill.Name] = skill
		return nil
	})
}

// loadSkill loads a single skill from a markdown file
func (l *SkillLoader) loadSkill(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	content := string(data)
	skill := &Skill{
		Content: content,
	}
	
	// Parse YAML frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			frontmatter := strings.TrimSpace(parts[0])
			if err := yaml.Unmarshal([]byte(frontmatter), skill); err != nil {
				return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
			}
			skill.Content = strings.TrimSpace(parts[1])
		}
	}
	
	if skill.Name == "" {
		// Use filename as name
		skill.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	
	return skill, nil
}

// Get returns a skill by name
func (l *SkillLoader) Get(name string) (*Skill, bool) {
	s, ok := l.skills[name]
	return s, ok
}

// GetAll returns all loaded skills
func (l *SkillLoader) GetAll() map[string]*Skill {
	return l.skills
}

// GetByBugClass returns skills matching a bug class
func (l *SkillLoader) GetByBugClass(bugClass string) []*Skill {
	var result []*Skill
	for _, s := range l.skills {
		if strings.EqualFold(s.BugClass, bugClass) {
			result = append(result, s)
		}
	}
	return result
}

// GetRelevant returns up to maxSkills skills relevant to the given context
func (l *SkillLoader) GetRelevant(bugClasses []string, maxSkills int) []*Skill {
	var result []*Skill
	seen := make(map[string]bool)
	
	// First, match by bug class
	for _, bc := range bugClasses {
		for _, s := range l.skills {
			if len(result) >= maxSkills {
				return result
			}
			if strings.EqualFold(s.BugClass, bc) && !seen[s.Name] {
				result = append(result, s)
				seen[s.Name] = true
			}
		}
	}
	
	// Then, match by triggers
	for _, bc := range bugClasses {
		for _, s := range l.skills {
			if len(result) >= maxSkills {
				return result
			}
			if seen[s.Name] {
				continue
			}
			for _, trigger := range s.Triggers {
				if strings.Contains(strings.ToLower(bc), strings.ToLower(trigger)) {
					result = append(result, s)
					seen[s.Name] = true
					break
				}
			}
		}
	}
	
	return result
}

// ToJSON returns all skills as JSON
func (l *SkillLoader) ToJSON() (string, error) {
	skills := make([]*Skill, 0, len(l.skills))
	for _, s := range l.skills {
		skills = append(skills, s)
	}
	b, err := json.MarshalIndent(skills, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
