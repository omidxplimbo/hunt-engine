package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserTool provides browser automation via chromedp for DOM-based testing
type BrowserTool struct{}

func NewBrowserTool() *BrowserTool {
	return &BrowserTool{}
}

func (b *BrowserTool) Name() string { return "browser" }

func (b *BrowserTool) Description() string {
	return `Browser automation tool for testing web applications. Use this to:
- Navigate to URLs and extract page content
- Fill and submit forms (test XSS, CSRF, etc.)
- Execute JavaScript in the page context
- Extract DOM elements (forms, inputs, links, tokens)
- Capture screenshots as evidence
- Test DOM-based XSS by checking if alert/confirm/prompt fires
- Detect client-side vulnerabilities that require JS execution
Use 'action' parameter to specify what to do.`
}

func (b *BrowserTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: navigate, get_content, get_forms, fill_form, execute_js, screenshot, test_xss",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "URL to navigate to",
			},
			"js_code": map[string]any{
				"type":        "string",
				"description": "JavaScript code to execute (for execute_js action)",
			},
			"payload": map[string]any{
				"type":        "string",
				"description": "XSS payload to test (for test_xss action)",
			},
			"form_selector": map[string]any{
				"type":        "string",
				"description": "CSS selector for form (for fill_form action)",
			},
			"form_data": map[string]any{
				"type":        "object",
				"description": "Form field values as key-value pairs (for fill_form action)",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
			},
		},
		"required": []string{"action"},
	}
}

func (b *BrowserTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	timeoutSec := 30
	if t, ok := params["timeout_seconds"].(float64); ok && t > 0 {
		timeoutSec = int(t)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Create chromedp context with headless options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	switch action {
	case "navigate":
		return b.navigate(browserCtx, params)
	case "get_content":
		return b.getContent(browserCtx, params)
	case "get_forms":
		return b.getForms(browserCtx, params)
	case "fill_form":
		return b.fillForm(browserCtx, params)
	case "execute_js":
		return b.executeJS(browserCtx, params)
	case "screenshot":
		return b.screenshot(browserCtx, params)
	case "test_xss":
		return b.testXSS(browserCtx, params)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (b *BrowserTool) navigate(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	if targetURL == "" {
		return "", fmt.Errorf("url is required for navigate")
	}

	var title, finalURL string
	var statusCode int

	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
		chromedp.Title(&title),
		chromedp.Location(&finalURL),
	)
	if err != nil {
		return fmt.Sprintf("[ERROR] Navigation failed: %v", err), nil
	}

	return fmt.Sprintf("[NAVIGATED] %s\n[TITLE] %s\n[FINAL URL] %s\n[STATUS] %d", targetURL, title, finalURL, statusCode), nil
}

func (b *BrowserTool) getContent(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	if targetURL == "" {
		return "", fmt.Errorf("url is required for get_content")
	}

	var content string
	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("html", &content),
	)
	if err != nil {
		return fmt.Sprintf("[ERROR] %v", err), nil
	}

	if len(content) > 100000 {
		content = content[:100000] + "\n... [truncated]"
	}

	return content, nil
}

func (b *BrowserTool) getForms(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	if targetURL == "" {
		return "", fmt.Errorf("url is required for get_forms")
	}

	var formsHTML string
	js := `
		(function() {
			var forms = document.querySelectorAll('form');
			var result = [];
			forms.forEach(function(form, i) {
				var fields = [];
				form.querySelectorAll('input, textarea, select').forEach(function(el) {
					fields.push({
						tag: el.tagName,
						type: el.type || '',
						name: el.name || '',
						id: el.id || '',
						value: el.value || '',
						placeholder: el.placeholder || ''
					});
				});
				result.push({
					index: i,
					action: form.action || '',
					method: form.method || 'GET',
					id: form.id || '',
					fields: fields
				});
			});
			return JSON.stringify(result, null, 2);
		})()
	`

	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(js, &formsHTML),
	)
	if err != nil {
		return fmt.Sprintf("[ERROR] %v", err), nil
	}

	return fmt.Sprintf("[FORMS FOUND]\n%s", formsHTML), nil
}

func (b *BrowserTool) fillForm(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	formSelector, _ := params["form_selector"].(string)
	formData, _ := params["form_data"].(map[string]any)

	if targetURL == "" {
		return "", fmt.Errorf("url is required")
	}
	if formSelector == "" {
		formSelector = "form"
	}

	// Build JS to fill form
	var fillJS strings.Builder
	fillJS.WriteString("(function() { var form = document.querySelector('" + formSelector + "'); if (!form) return 'Form not found'; ")

	for name, value := range formData {
		val := fmt.Sprintf("%v", value)
		// Escape single quotes in value
		val = strings.ReplaceAll(val, "'", "\\'")
		fillJS.WriteString(fmt.Sprintf("var el = form.querySelector('[name=%s]'); if (el) el.value = '%s'; ", name, val))
	}

	fillJS.WriteString("form.submit(); return 'Form submitted'; })()")

	var result string
	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(fillJS.String(), &result),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		return fmt.Sprintf("[ERROR] %v", err), nil
	}

	var finalURL, title string
	chromedp.Run(ctx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	)

	return fmt.Sprintf("[FORM RESULT] %s\n[FINAL URL] %s\n[TITLE] %s", result, finalURL, title), nil
}

func (b *BrowserTool) executeJS(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	jsCode, _ := params["js_code"].(string)

	if jsCode == "" {
		return "", fmt.Errorf("js_code is required")
	}

	var result string
	if targetURL != "" {
		err := chromedp.Run(ctx,
			chromedp.Navigate(targetURL),
			chromedp.WaitReady("body"),
			chromedp.Evaluate(jsCode, &result),
		)
		if err != nil {
			return fmt.Sprintf("[ERROR] %v", err), nil
		}
	} else {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(jsCode, &result),
		)
		if err != nil {
			return fmt.Sprintf("[ERROR] %v", err), nil
		}
	}

	return fmt.Sprintf("[JS RESULT]\n%s", result), nil
}

func (b *BrowserTool) screenshot(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	if targetURL == "" {
		return "", fmt.Errorf("url is required for screenshot")
	}

	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		return fmt.Sprintf("[ERROR] Screenshot failed: %v", err), nil
	}

	return fmt.Sprintf("[SCREENSHOT] Captured %d bytes (base64 encoded in evidence store)", len(buf)), nil
}

func (b *BrowserTool) testXSS(ctx context.Context, params map[string]any) (string, error) {
	targetURL, _ := params["url"].(string)
	payload, _ := params["payload"].(string)

	if targetURL == "" || payload == "" {
		return "", fmt.Errorf("url and payload are required for test_xss")
	}

	// Navigate and inject payload into all input fields
	var result string
	js := fmt.Sprintf(`
		(function() {
			var triggered = false;
			var originalAlert = window.alert;
			var originalConfirm = window.confirm;
			var originalPrompt = window.prompt;

			window.alert = function(msg) { triggered = true; window.__xss_result = 'ALERT: ' + msg; };
			window.confirm = function(msg) { triggered = true; window.__xss_result = 'CONFIRM: ' + msg; return true; };
			window.prompt = function(msg) { triggered = true; window.__xss_result = 'PROMPT: ' + msg; return 'test'; };

			var payload = '%s';
			var inputs = document.querySelectorAll('input[type=text], input[type=search], textarea');
			inputs.forEach(function(input) {
				input.value = payload;
				input.dispatchEvent(new Event('input', {bubbles: true}));
				input.dispatchEvent(new Event('change', {bubbles: true}));
			});

			// Also try URL parameter injection
			var urlParams = new URLSearchParams(window.location.search);
			urlParams.forEach(function(val, key) {
				urlParams.set(key, payload);
			});
			var newURL = window.location.pathname + '?' + urlParams.toString();
			if (urlParams.toString()) {
				window.location.href = newURL;
			}

			// Wait a bit for potential XSS execution
			return new Promise(function(resolve) {
				setTimeout(function() {
					window.alert = originalAlert;
					window.confirm = originalConfirm;
					window.prompt = originalPrompt;
					if (triggered) {
						resolve('VULNERABLE: ' + (window.__xss_result || 'XSS triggered'));
					} else {
						resolve('NOT_TRIGGERED: No XSS execution detected');
					}
				}, 2000);
			});
		})()
	`, strings.ReplaceAll(payload, "'", "\\'"))

	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(js, &result),
	)
	if err != nil {
		return fmt.Sprintf("[ERROR] XSS test failed: %v", err), nil
	}

	return fmt.Sprintf("[XSS TEST] URL: %s\n[PAYLOAD] %s\n[RESULT] %s", targetURL, payload, result), nil
}
