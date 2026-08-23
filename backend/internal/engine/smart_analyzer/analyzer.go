package smart_analyzer

import (
	"fmt"
	"strings"
	"net/http"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

// AnalysisResult نتیجه تحلیل هوشمند مقایسه دو درخواست
type AnalysisResult struct {
	IsVulnerable      bool     `json:"is_vulnerable"`
	ConfidenceScore   float64  `json:"confidence_score"` // 0.0 to 1.0
	VulnerabilityType string   `json:"vulnerability_type"` // IDOR, BOLA, Privilege Escalation
	Evidence          []string `json:"evidence"`
	NextStepHint      string   `json:"next_step_hint"` // پیشنهاد به ایجنت برای گام بعد
}

// IntelligentComparator مقایسه هوشمند پاسخ‌های دو حساب
func IntelligentComparator(resp1, resp2 *http.Response, body1, body2 []byte) *AnalysisResult {
	result := &AnalysisResult{
		ConfidenceScore: 0.0,
		Evidence:        make([]string, 0),
	}

	// 1. بررسی کد وضعیت (Status Code Logic)
	if resp1.StatusCode != resp2.StatusCode {
		result.IsVulnerable = true
		result.ConfidenceScore += 0.4
		result.VulnerabilityType = "IDOR / Access Control Bypass"
		result.Evidence = append(result.Evidence, 
			fmt.Sprintf("Status Code Mismatch: Account A returned %d, Account B returned %d", resp1.StatusCode, resp2.StatusCode))
		
		if resp1.StatusCode == 200 && resp2.StatusCode == 403 {
			result.NextStepHint = "Direct IDOR confirmed. Try extracting sensitive data fields."
		}
	}

	// 2. بررسی محتوای بدنه (Semantic Content Analysis)
	// حتی اگر کد وضعیت یکی باشد (مثلا هر دو 200)، محتوا ممکن است لوکالیزه یا فیلتر شده باشد
	if strings.EqualFold(string(body1), string(body2)) {
		// محتوای کاملا یکسان -> احتمالا امن (یا باگ منطقی عمیق‌تر)
		if !result.IsVulnerable {
			result.NextStepHint = "Responses identical. Try changing Resource ID or HTTP Method."
		}
	} else {
		// محتوای متفاوت است -> تحلیل عمیق‌تر
		diffScore := analyzeContentDiff(body1, body2)
		if diffScore > 0.7 {
			result.IsVulnerable = true
			result.ConfidenceScore += 0.5
			result.VulnerabilityType = "Potential Data Leakage / IDOR"
			result.Evidence = append(result.Evidence, "Significant content difference detected between accounts.")
			result.NextStepHint = "High divergence in response bodies. Check for PII or private keys."
		} else {
			// تفاوت جزئی (مثلا timestamp یا token)
			result.Evidence = append(result.Evidence, "Minor content differences detected (likely dynamic tokens/timestamps).")
			result.NextStepHint = "Differences seem noise-related. Try a different endpoint or parameter."
		}
	}

	// نرمال‌سازی امتیاز اطمینان
	if result.ConfidenceScore > 1.0 {
		result.ConfidenceScore = 1.0
	}

	return result
}

// analyzeContentDiff یک تابع ساده برای تخمین تفاوت محتوایی (قابل توسعه به LLM)
func analyzeContentDiff(b1, b2 []byte) float64 {
	// اینجا می‌توان از الگوریتم‌های Levenshtein یا Diff استفاده کرد
	// برای سادگی فعلا نسبت تفاوت بایت‌ها را حساب می‌کنیم
	if len(b1) == 0 && len(b2) == 0 { return 0.0 }
	if len(b1) == 0 || len(b2) == 0 { return 1.0 }
	
	// شبیه‌سازی امتیاز دهی (در نسخه نهایی از کتابخانه diff استفاده شود)
	if string(b1) == string(b2) { return 0.0 }
	return 0.8 // فرض بر تفاوت زیاد
}

// GenerateHypothesis تولید فرضیه حمله برای ایجنت
func GenerateHypothesis(session *models.TwoAccountSession, lastResult *AnalysisResult) string {
	if lastResult.IsVulnerable {
		return fmt.Sprintf("High confidence IDOR detected on target %d. Recommend immediate exploitation attempt.", session.TargetID)
	}
	
	// اگر هنوز باگی پیدا نشده، ایجنت باید استراتژی عوض کند
	strategies := []string{
		"Try switching from GET to POST with same ID.",
		"Attempt UUID enumeration instead of integer.",
		"Check if 'Admin' role in Account A allows writing to Account B's resource.",
		"Analyze CORS headers for misconfiguration allowing cross-account reads.",
	}
	
	// انتخاب استراتژی تصادفی یا بر اساس تاریخچه (ساده‌سازی شده)
	return strategies[0] 
}
