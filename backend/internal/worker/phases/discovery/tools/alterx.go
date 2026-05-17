package tools

import (
	"log"
	"strings"
)

func RunAlterx(ctx Context, inputFile, rootDomain string) ([]string, error) {
	if err := ctx.ensureCommandRunner(); err != nil {
		return nil, err
	}

	output, err := ctx.RunCommand(ctx.TargetID, "alterx", "-l", inputFile, "-silent")
	if err != nil {
		return nil, err
	}

	unique := make(map[string]bool)
	var results []string

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		if trimmed != "" && strings.HasSuffix(trimmed, rootDomain) && !unique[trimmed] {
			unique[trimmed] = true
			results = append(results, trimmed)
		}
	}

	log.Printf("✅ Alterx produced %d unique candidates for %s\n", len(results), rootDomain)
	return results, nil
}
