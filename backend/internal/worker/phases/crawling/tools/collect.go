package tools

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"strings"
)

func collectFromCommand(ctx Context, inputFile string, sourceLabel string, cmdName string, cmdArgs ...string) map[string]string {
	results := make(map[string]string)
	var output []byte
	var err error

	if cmdName == "waybackurls" || cmdName == "gau" {
		f, err := os.Open(inputFile)
		if err != nil {
			log.Printf("⚠️ Failed to open input file for %s: %v\n", cmdName, err)
			return results
		}
		defer f.Close()

		output, err = ctx.RunCommandWithStdin(ctx.TargetID, f, cmdName, cmdArgs...)
	} else {
		output, err = ctx.RunCommand(ctx.TargetID, cmdName, cmdArgs...)
	}

	if err != nil {
		log.Printf("⚠️ Tool %s failed or killed: %v\n", cmdName, err)
		if len(output) > 0 {
			log.Printf("⚠️ Tool %s output before failure: %s", cmdName, strings.TrimSpace(string(output)))
		}
		return results
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		u := strings.TrimSpace(scanner.Text())
		if u == "" {
			continue
		}
		if _, exists := results[u]; !exists {
			results[u] = sourceLabel
		}
	}

	log.Printf("✅ Tool %s completed with %d URLs", cmdName, len(results))

	return results
}
