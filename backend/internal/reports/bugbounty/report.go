package reports

import (
	"fmt"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// BugBountyReport represents a standard bug bounty report
type BugBountyReport struct {
	Title           string    `json:"title"`
	Severity        string    `json:"severity"`
	CWEID           string    `json:"cwe_id"`
	Summary         string    `json:"summary"`
	AffectedURL     string    `json:"affected_url"`
	StepsToReproduce []string `json:"steps_to_reproduce"`
	PoC             string    `json:"poc"`
	Impact          string    `json:"impact"`
	Remediation     string    `json:"remediation"`
	Evidence        []EvidenceEntry `json:"evidence"`
	GeneratedAt     time.Time `json:"generated_at"`
	Target          string    `json:"target"`
}

// EvidenceEntry represents a piece of evidence in the report
type EvidenceEntry struct {
	Description string `json:"description"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
	Screenshot  string `json:"screenshot,omitempty"`
}

// GenerateReport generates a bug bounty report from Hunter Agent evidence
func GenerateReport(evidences []hunter.Evidence, target string) *BugBountyReport {
	// Find the best confirmed evidence
	var bestEvidence *hunter.Evidence
	for i, e := range evidences {
		if e.Status == "confirmed" && e.Confidence > 0.5 {
			if bestEvidence == nil || e.Confidence > bestEvidence.Confidence {
				bestEvidence = &evidences[i]
			}
		}
	}
	
	if bestEvidence == nil {
		return nil
	}
	
	report := &BugBountyReport{
		GeneratedAt: time.Now().UTC(),
		Target:      target,
	}
	
	// Generate based on vulnerability type
	switch bestEvidence.TestType {
	case "xss":
		report = generateXSSReport(bestEvidence, target)
	case "sqli":
		report = generateSQLiReport(bestEvidence, target)
	case "idor":
		report = generateIDORReport(bestEvidence, target)
	default:
		report = generateGenericReport(bestEvidence, target)
	}
	
	// Add all confirmed evidence
	for _, e := range evidences {
		if e.Status == "confirmed" {
			report.Evidence = append(report.Evidence, EvidenceEntry{
				Description: fmt.Sprintf("%s test on parameter '%s'", e.TestType, e.Parameter),
				Request:     fmt.Sprintf("%s %s", e.Result.RequestMethod, e.Result.RequestURL),
				Response:    truncate(e.Result.ResponseBody, 500),
			})
		}
	}
	
	return report
}

func generateXSSReport(e *hunter.Evidence, target string) *BugBountyReport {
	severity := "Medium"
	if e.Confidence > 0.8 {
		severity = "High"
	}
	
	return &BugBountyReport{
		Title:    fmt.Sprintf("Reflected XSS in %s parameter on %s", e.Parameter, target),
		Severity: severity,
		CWEID:    "CWE-79",
		Summary: fmt.Sprintf(
			"A reflected Cross-Site Scripting (XSS) vulnerability was identified in the '%s' parameter on %s. "+
				"The application reflects user-supplied input without proper sanitization, allowing an attacker "+
				"to execute arbitrary JavaScript in the context of the victim's browser.",
			e.Parameter, e.Target),
		AffectedURL: e.Target,
		StepsToReproduce: []string{
			fmt.Sprintf("1. Navigate to %s", e.Target),
			fmt.Sprintf("2. Inject the payload into the '%s' parameter", e.Parameter),
			fmt.Sprintf("3. Payload: %s", e.Payload),
			"4. Observe that the payload is reflected and executed in the response",
		},
		PoC: e.PoC,
		Impact: "An attacker could steal session cookies, redirect users to malicious sites, " +
			"deface the application, or perform actions on behalf of the victim.",
		Remediation: "1. Implement input validation and output encoding\n" +
			"2. Use Content Security Policy (CSP) headers\n" +
			"3. Sanitize all user input before rendering in HTML context\n" +
			"4. Use frameworks that automatically escape output",
	}
}

func generateSQLiReport(e *hunter.Evidence, target string) *BugBountyReport {
	severity := "Critical"
	
	return &BugBountyReport{
		Title:    fmt.Sprintf("SQL Injection in %s parameter on %s", e.Parameter, target),
		Severity: severity,
		CWEID:    "CWE-89",
		Summary: fmt.Sprintf(
			"A SQL Injection vulnerability was identified in the '%s' parameter on %s. "+
				"The application incorporates user input into SQL queries without proper parameterization.",
			e.Parameter, e.Target),
		AffectedURL: e.Target,
		StepsToReproduce: []string{
			fmt.Sprintf("1. Navigate to %s", e.Target),
			fmt.Sprintf("2. Inject SQL payload into '%s' parameter", e.Parameter),
			fmt.Sprintf("3. Payload: %s", e.Payload),
			"4. Observe SQL error or differential response indicating injection",
		},
		PoC: e.PoC,
		Impact: "An attacker could extract sensitive data from the database, modify or delete data, " +
			"potentially execute operating system commands, and gain unauthorized access.",
		Remediation: "1. Use parameterized queries / prepared statements\n" +
			"2. Implement input validation\n" +
			"3. Apply principle of least privilege for database accounts\n" +
			"4. Use an ORM with built-in protection",
	}
}

func generateIDORReport(e *hunter.Evidence, target string) *BugBountyReport {
	severity := "High"
	
	return &BugBountyReport{
		Title:    fmt.Sprintf("IDOR on %s", target),
		Severity: severity,
		CWEID:    "CWE-639",
		Summary: fmt.Sprintf(
			"An Insecure Direct Object Reference (IDOR) vulnerability was identified on %s. "+
				"The application allows access to resources belonging to other users by manipulating object identifiers.",
			e.Target),
		AffectedURL: e.Target,
		StepsToReproduce: []string{
			"1. Log in as User A and access a resource",
			"2. Note the resource ID in the URL or request",
			"3. Log in as User B",
			fmt.Sprintf("4. Access the same resource with User A's ID: %s", e.Target),
			"5. Observe that User B can access User A's data",
		},
		PoC: e.PoC,
		Impact: "An attacker could access, modify, or delete other users' data, " +
			"leading to privacy violations and potential account takeover.",
		Remediation: "1. Implement proper authorization checks\n" +
			"2. Use indirect references (mapped IDs)\n" +
			"3. Verify resource ownership before serving data\n" +
			"4. Implement rate limiting on enumeration attempts",
	}
}

func generateGenericReport(e *hunter.Evidence, target string) *BugBountyReport {
	return &BugBountyReport{
		Title:    fmt.Sprintf("Security Finding: %s on %s", e.TestType, target),
		Severity: "Medium",
		CWEID:    "CWE-200",
		Summary: fmt.Sprintf(
			"A security vulnerability of type '%s' was identified on %s.",
			e.TestType, e.Target),
		AffectedURL: e.Target,
		StepsToReproduce: []string{
			fmt.Sprintf("1. Navigate to %s", e.Target),
			fmt.Sprintf("2. Test parameter '%s' with payload: %s", e.Parameter, e.Payload),
			"3. Observe the vulnerability indicator in the response",
		},
		PoC: e.PoC,
		Impact: "This vulnerability could lead to unauthorized access or data exposure.",
		Remediation: "1. Implement input validation\n2. Apply output encoding\n3. Follow security best practices",
	}
}

// ToMarkdown converts the report to Markdown format
func (r *BugBountyReport) ToMarkdown() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("# %s\n\n", r.Title))
	sb.WriteString(fmt.Sprintf("**Severity:** %s\n", r.Severity))
	sb.WriteString(fmt.Sprintf("**CWE:** %s\n", r.CWEID))
	sb.WriteString(fmt.Sprintf("**Target:** %s\n", r.Target))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05")))
	
	sb.WriteString("## Summary\n\n")
	sb.WriteString(r.Summary + "\n\n")
	
	sb.WriteString("## Affected URL\n\n")
	sb.WriteString(fmt.Sprintf("`%s`\n\n", r.AffectedURL))
	
	sb.WriteString("## Steps to Reproduce\n\n")
	for _, step := range r.StepsToReproduce {
		sb.WriteString(step + "\n")
	}
	sb.WriteString("\n")
	
	if r.PoC != "" {
		sb.WriteString("## Proof of Concept\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.PoC + "\n")
		sb.WriteString("```\n\n")
	}
	
	sb.WriteString("## Impact\n\n")
	sb.WriteString(r.Impact + "\n\n")
	
	sb.WriteString("## Remediation\n\n")
	sb.WriteString(r.Remediation + "\n\n")
	
	if len(r.Evidence) > 0 {
		sb.WriteString("## Evidence\n\n")
		for i, e := range r.Evidence {
			sb.WriteString(fmt.Sprintf("### Evidence %d: %s\n\n", i+1, e.Description))
			if e.Request != "" {
				sb.WriteString("**Request:**\n```\n" + e.Request + "\n```\n\n")
			}
			if e.Response != "" {
				sb.WriteString("**Response (excerpt):**\n```\n" + e.Response + "\n```\n\n")
			}
		}
	}
	
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
