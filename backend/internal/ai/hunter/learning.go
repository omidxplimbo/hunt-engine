package hunter

import (
	"fmt"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"gorm.io/gorm"
)

// LearningEngine manages the agent's learning from test results
type LearningEngine struct {
	db       *gorm.DB
	targetID uint
	userID   uint
	ownerKey string
}

// NewLearningEngine creates a new learning engine
func NewLearningEngine(db *gorm.DB, targetID, userID uint, ownerKey string) *LearningEngine {
	return &LearningEngine{
		db:       db,
		targetID: targetID,
		userID:   userID,
		ownerKey: ownerKey,
	}
}

// LearnFromResults analyzes evidence and creates learning records
func (l *LearningEngine) LearnFromResults(evidences []Evidence) []LearningInsight {
	insights := make([]LearningInsight, 0)
	
	// Group evidence by type
	byType := make(map[string][]Evidence)
	for _, e := range evidences {
		byType[e.TestType] = append(byType[e.TestType], e)
	}
	
	// Analyze each type
	for testType, typeEvidence := range byType {
		insight := l.analyzeType(testType, typeEvidence)
		if insight != nil {
			insights = append(insights, *insight)
			l.saveLearningRecord(insight)
		}
	}
	
	// Check for creative discoveries
	creativeInsights := l.findCreativePatterns(evidences)
	insights = append(insights, creativeInsights...)
	
	return insights
}

// LearningInsight represents what the agent learned
type LearningInsight struct {
	Type        string   `json:"type"`        // success, failure, pattern, creative
	BugClass    string   `json:"bug_class"`
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"`
	NextSteps   []string `json:"next_steps"`
	ShouldSpawn []string `json:"should_spawn"` // Other bug classes to test
}

// analyzeType analyzes evidence of a specific type
func (l *LearningEngine) analyzeType(testType string, evidences []Evidence) *LearningInsight {
	confirmed := 0
	total := len(evidences)
	
	for _, e := range evidences {
		if e.Status == "confirmed" {
			confirmed++
		}
	}
	
	if total == 0 {
		return nil
	}
	
	insight := &LearningInsight{
		BugClass: testType,
	}
	
	if confirmed > 0 {
		insight.Type = "success"
		insight.Confidence = float64(confirmed) / float64(total)
		insight.Description = fmt.Sprintf("Found %d confirmed %s vulnerabilities out of %d tests", confirmed, testType, total)
		insight.NextSteps = []string{
			fmt.Sprintf("Generate PoC report for %s findings", testType),
			"Test for vulnerability chaining",
			"Check for similar patterns on other endpoints",
		}
	} else {
		insight.Type = "failure"
		insight.Confidence = 0.1
		insight.Description = fmt.Sprintf("No %s vulnerabilities found in %d tests", testType, total)
		insight.NextSteps = []string{
			"Try different payloads",
			"Expand URL discovery",
			"Test with authenticated context",
		}
	}
	
	return insight
}

// findCreativePatterns looks for patterns that suggest other vulnerability types
func (l *LearningEngine) findCreativePatterns(evidences []Evidence) []LearningInsight {
	insights := make([]LearningInsight, 0)
	
	for _, e := range evidences {
		if e.Result == nil {
			continue
		}
		
		// If we see certain patterns, suggest other tests
		body := strings.ToLower(e.Result.ResponseBody)
		
		// If we see API endpoints, suggest IDOR testing
		if strings.Contains(body, "/api/") || strings.Contains(body, "application/json") {
			insights = append(insights, LearningInsight{
				Type:        "creative",
				BugClass:    "idor",
				Description: "API endpoints detected - IDOR testing recommended",
				Confidence:  0.6,
				ShouldSpawn: []string{"idor"},
				NextSteps:   []string{"Test API endpoints for IDOR with different user contexts"},
			})
		}
		
		// If we see forms, suggest CSRF/XSS testing
		if strings.Contains(body, "<form") || strings.Contains(body, "method=\"post\"") {
			insights = append(insights, LearningInsight{
				Type:        "creative",
				BugClass:    "csrf",
				Description: "Forms detected - CSRF testing recommended",
				Confidence:  0.5,
				ShouldSpawn: []string{"csrf"},
				NextSteps:   []string{"Test forms for CSRF token validation"},
			})
		}
		
		// If we see file upload, suggest file upload testing
		if strings.Contains(body, "upload") || strings.Contains(body, "multipart") {
			insights = append(insights, LearningInsight{
				Type:        "creative",
				BugClass:    "file_upload",
				Description: "File upload detected - upload bypass testing recommended",
				Confidence:  0.7,
				ShouldSpawn: []string{"file_upload"},
				NextSteps:   []string{"Test file upload for extension bypass, content-type bypass"},
			})
		}
		
		// If we see redirects, suggest open redirect testing
		if e.Result.Redirected || e.Result.ResponseStatus == 301 || e.Result.ResponseStatus == 302 {
			insights = append(insights, LearningInsight{
				Type:        "creative",
				BugClass:    "open_redirect",
				Description: "Redirect detected - open redirect testing recommended",
				Confidence:  0.6,
				ShouldSpawn: []string{"open_redirect"},
				NextSteps:   []string{"Test redirect parameters for external redirect"},
			})
		}
	}
	
	return insights
}

// saveLearningRecord saves a learning insight to the database
func (l *LearningEngine) saveLearningRecord(insight *LearningInsight) {
	record := models.OperatorLearningRecord{
		UserID:    l.userID,
		OwnerKey:  l.ownerKey,
		Scope:     "target",
		Source:    "hunter_agent",
		Status:    "active",
		TargetID:  &l.targetID,
		Title:     fmt.Sprintf("[%s] %s", insight.Type, insight.BugClass),
		Summary:   insight.Description,
		BugClass:  insight.BugClass,
		Confidence: int(insight.Confidence * 100),
	}
	
	l.db.Create(&record)
}

// GetRelevantSkills returns skills relevant to the current strategy
func (l *LearningEngine) GetRelevantSkills(bugClasses []string) []string {
	var relevant []string
	
	// Get past learning records for this target
	var records []models.OperatorLearningRecord
	l.db.Where("target_id = ? AND status = ?", l.targetID, "active").
		Order("created_at DESC").Limit(10).Find(&records)
	
	// Add bug classes from past learning
	for _, r := range records {
		if r.BugClass != "" {
			relevant = append(relevant, r.BugClass)
		}
	}
	
	// Add requested bug classes
	relevant = append(relevant, bugClasses...)
	
	// Deduplicate
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, bc := range relevant {
		if !seen[bc] {
			seen[bc] = true
			result = append(result, bc)
		}
	}
	
	return result
}

// ShouldSpawnSubAgent checks if we should spawn a sub-agent for a different bug class
func (l *LearningEngine) ShouldSpawnSubAgent(insights []LearningInsight) []string {
	var spawn []string
	
	for _, insight := range insights {
		if insight.Type == "creative" && insight.Confidence > 0.5 {
			spawn = append(spawn, insight.ShouldSpawn...)
		}
	}
	
	return spawn
}
