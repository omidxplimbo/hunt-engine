package tools

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func RunWaymore(ctx Context) map[string]string {
	results := make(map[string]string)
	log.Printf(" Running Waymore for %s...\n", ctx.RootDomain)

	waymoreOutputFile := filepath.Join(ctx.TempDir, "waymore.txt")
	_, err := ctx.RunCommand(ctx.TargetID, "waymore", "-i", ctx.RootDomain, "-mode", "U", "-n", "-oU", waymoreOutputFile)
	if err != nil {
		log.Printf("⚠️ Waymore execution failed: %v\n", err)
		return results
	}

	content, err := os.ReadFile(waymoreOutputFile)
	if err != nil {
		log.Printf("⚠️ Could not read Waymore output file: %v\n", err)
		_ = os.Remove(waymoreOutputFile)
		return results
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		u := strings.TrimSpace(line)
		if u != "" {
			results[u] = "waymore"
		}
	}
	log.Printf("✅ Waymore found %d URLs.\n", len(results))
	_ = os.Remove(waymoreOutputFile)

	return results
}
