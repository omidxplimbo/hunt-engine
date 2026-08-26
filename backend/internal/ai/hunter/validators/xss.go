package validators

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// XSSResult holds the outcome of a browser-based XSS validation
type XSSResult struct {
	Executed     bool    `json:"executed"`      // Did a dialog fire?
	DialogType   string  `json:"dialog_type"`   // alert/confirm/prompt
	DialogContent string `json:"dialog_content"`
	Screenshot   []byte  `json:"-"`
	Confidence   float64 `json:"confidence"`
	Detail       string  `json:"detail"`
}

// ValidateXSSInBrowser navigates to the URL with the payload injected into the
// named parameter and reports whether JavaScript actually executed.
// This is REAL exploit validation — not string matching. Dialogs are detected
// via CDP's javascriptDialogOpening event (works even when the page overrides
// nothing) and auto-dismissed so headless chrome never blocks.
func ValidateXSSInBrowser(ctx context.Context, targetURL, paramName, payload string) (*XSSResult, error) {
	result := &XSSResult{}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// Listen for real native dialogs fired by payload execution.
	// The callback only records and signals (calling Run from inside the
	// event loop deadlocks); a watcher goroutine performs the dismissal.
	var mu sync.Mutex
	var gotDialog *page.EventJavascriptDialogOpening
	dialogSeen := make(chan struct{}, 1)

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		if e, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			mu.Lock()
			if gotDialog == nil {
				gotDialog = e
			}
			mu.Unlock()
			select {
			case dialogSeen <- struct{}{}:
			default:
			}
		}
	})

	// Watcher: as soon as a dialog appears, dismiss it so the page unblocks.
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-dialogSeen:
				dctx, dcancel := context.WithTimeout(browserCtx, 2*time.Second)
				_ = chromedp.Run(dctx, chromedp.ActionFunc(func(actx context.Context) error {
					return page.HandleJavaScriptDialog(false).Do(actx)
				}))
				dcancel()
			}
		}
	}()

	testURL := injectParam(targetURL, paramName, payload)

	err := chromedp.Run(browserCtx,
		chromedp.Navigate(testURL),
		chromedp.Sleep(5*time.Second),
		chromedp.FullScreenshot(&result.Screenshot, 80),
	)

	mu.Lock()
	dialogEvent := gotDialog
	mu.Unlock()

	if err != nil && dialogEvent == nil {
		result.Detail = fmt.Sprintf("browser error: %v", err)
		return result, nil
	}

	if dialogEvent != nil {
		result.Executed = true
		result.DialogType = strings.ToLower(dialogEvent.Type.String())
		result.DialogContent = dialogEvent.Message
		result.Confidence = 0.99
		result.Detail = fmt.Sprintf("JavaScript executed in real browser at %s (%s)", testURL, result.DialogType)
	} else {
		result.Confidence = 0.1
		result.Detail = fmt.Sprintf("No JS execution detected at %s — likely encoded or CSP-blocked", testURL)
	}

	return result, nil
}

// injectParam adds/replaces a query parameter with the given value
func injectParam(rawURL, param, value string) string {
	if param == "" || !strings.Contains(rawURL, "?") && !strings.Contains(rawURL, param+"=") {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		return rawURL + sep + param + "=" + urlEncode(value)
	}
	// naive replace of existing param
	idx := strings.Index(rawURL, param+"=")
	if idx == -1 {
		return rawURL + "&" + param + "=" + urlEncode(value)
	}
	end := strings.Index(rawURL[idx:], "&")
	if end == -1 {
		return rawURL[:idx] + param + "=" + urlEncode(value)
	}
	return rawURL[:idx] + param + "=" + urlEncode(value) + rawURL[idx+end:]
}

func urlEncode(s string) string {
	replacer := strings.NewReplacer(
		" ", "%20", "\"", "%22", "<", "%3C", ">", "%3E",
		"'", "%27", "#", "%23", "&", "%26",
	)
	return replacer.Replace(s)
}
