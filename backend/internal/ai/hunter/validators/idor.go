package validators

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// IDORResult captures the outcome of a two-account comparison
type IDORResult struct {
	Vulnerable    bool    `json:"vulnerable"`
	Confidence    float64 `json:"confidence"`
	StatusA       int     `json:"status_a"`
	StatusB       int     `json:"status_b"`
	BodyHashA     string  `json:"body_hash_a"`
	BodyHashB     string  `json:"body_hash_b"`
	Similarity    float64 `json:"similarity"` // 0..1, 1 = identical
	LeakedFields  []string `json:"leaked_fields,omitempty"`
	Detail        string  `json:"detail"`
}

// ValidateIDOR sends the same request as account A and account B and decides
// whether B received A's data (or vice versa) beyond generic page similarity.
func ValidateIDOR(clientA, clientB *http.Client, targetURL string) (*IDORResult, error) {
	res := &IDORResult{}

	respA, err := clientA.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("account A request failed: %w", err)
	}
	defer respA.Body.Close()
	bodyA := readLimited(respA)
	res.StatusA = respA.StatusCode

	respB, err := clientB.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("account B request failed: %w", err)
	}
	defer respB.Body.Close()
	bodyB := readLimited(respB)
	res.StatusB = respB.StatusCode

	res.BodyHashA = hashBody(bodyA)
	res.BodyHashB = hashBody(bodyB)
	res.Similarity = jaccardSimilarity(tokenize(bodyA), tokenize(bodyB))

	switch {
	case res.StatusA == 200 && res.StatusB == 200 && res.Similarity > 0.98 && res.BodyHashA != res.BodyHashB:
		// Both authorized but near-identical bodies with different hashes:
		// likely only a CSRF token/nonce differs — weak signal.
		res.Confidence = 0.3
		res.Detail = "Both accounts got 200 with nearly identical content (likely benign differences)"
	case res.StatusA == 200 && res.StatusB == 200 && res.Similarity < 0.6:
		// Very different content for the same resource is suspicious when the
		// URL references an object owned by A.
		res.Vulnerable = true
		res.Confidence = 0.7
		res.Detail = "Different content returned to second account for same object URL"
		res.LeakedFields = findSensitiveKeys(bodyB)
	default:
		res.Confidence = 0.1
		res.Detail = fmt.Sprintf("Statuses A=%d B=%d, similarity=%.2f", res.StatusA, res.StatusB, res.Similarity)
	}

	return res, nil
}

func readLimited(resp *http.Response) string {
	buf := make([]byte, 512*1024)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}

func hashBody(b string) string {
	h := sha256.Sum256([]byte(b))
	return hex.EncodeToString(h[:])[:16]
}

func tokenize(s string) map[string]bool {
	tokens := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '"' || r == '{' || r == '}'
	}) {
		if len(f) > 3 { // skip tiny tokens/noise
			tokens[f] = true
		}
	}
	return tokens
}

func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	union := map[string]bool{}
	for t := range a {
		union[t] = true
		if b[t] {
			inter++
		}
	}
	for t := range b {
		union[t] = true
	}
	if len(union) == 0 {
		return 1
	}
	return float64(inter) / float64(len(union))
}

// findSensitiveKeys looks for personal-data keys in JSON-ish bodies
func findSensitiveKeys(body string) []string {
	sensitive := []string{"email", "phone", "address", "ssn", "national_id", "api_key", "token"}
	var found []string
	lower := strings.ToLower(body)
	for _, k := range sensitive {
		if strings.Contains(lower, `"`+k+`"`) || strings.Contains(lower, k+":") {
			found = append(found, k)
		}
	}
	return found
}
