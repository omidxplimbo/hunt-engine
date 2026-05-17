package passive

import (
	"log"
	"os"
)

func RunAbuseDB(ctx Context) []string {
	scriptPath := "/root/hunt-engine/backend/scripts/abusedb.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Printf("❌ AbuseDB script not found at %s\n", scriptPath)
		return []string{}
	}

	runner := ctx.RunCombinedCommand
	if runner == nil {
		runner = ctx.RunCommand
	}

	output, err := runner("/bin/bash", scriptPath, ctx.Domain)
	if err != nil {
		log.Printf("❌ AbuseDB Script Error: %v\nOutput: %s\n", err, string(output))
		return []string{}
	}

	results := normalizeLines(output, ctx.Domain)
	if len(results) > 0 {
		log.Printf(" AbuseDB script found %d subdomains for %s\n", len(results), ctx.Domain)
	} else {
		log.Printf("⏩ AbuseDB script produced no results for %s\n", ctx.Domain)
	}

	return results
}
