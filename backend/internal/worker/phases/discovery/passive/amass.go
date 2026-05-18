package passive

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAmassTimeoutSeconds = 900

func amassTimeout() time.Duration {
	value := strings.TrimSpace(os.Getenv("AMASS_TIMEOUT_SECONDS"))
	if value == "" {
		return time.Duration(defaultAmassTimeoutSeconds) * time.Second
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("⚠️ Invalid AMASS_TIMEOUT_SECONDS=%q. Using default %d seconds.\n", value, defaultAmassTimeoutSeconds)
		return time.Duration(defaultAmassTimeoutSeconds) * time.Second
	}

	if seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

func runAmassCommand(ctx Context, timeout time.Duration, args []string) ([]byte, error) {
	if ctx.RunCombinedCommandWithTimeout != nil {
		return ctx.RunCombinedCommandWithTimeout(timeout, "amass", args...)
	}
	if ctx.RunCombinedCommand != nil {
		return ctx.RunCombinedCommand("amass", args...)
	}
	if ctx.RunCommand != nil {
		return ctx.RunCommand("amass", args...)
	}
	return nil, nil
}

func parseAmassLines(ctx Context, data []byte, seen map[string]struct{}, results *[]string) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Ignore common banner/usage/log lines from Amass v5.
		lower := strings.ToLower(line)
		if strings.Contains(lower, "owasp amass") ||
			strings.HasPrefix(lower, "usage:") ||
			strings.Contains(lower, "discord") ||
			strings.HasPrefix(lower, "error:") ||
			strings.HasPrefix(lower, "failed ") ||
			strings.HasPrefix(lower, "[") {
			continue
		}

		value := NormalizeSubdomain(line, ctx.Domain)
		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		*results = append(*results, value)
	}
}

func parseAmassFile(ctx Context, path string, seen map[string]struct{}, results *[]string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		value := NormalizeSubdomain(scanner.Text(), ctx.Domain)
		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		*results = append(*results, value)
	}
}

// RunAmass runs OWASP Amass in passive mode and returns discovered subdomains.
//
// Amass v4 and v5 have different CLI/output behavior:
// - v4-style commands support output files via -o.
// - v5.x may reject older flags and often behaves better when stdout is captured.
// This runner tries v4-compatible file mode first, then v5-compatible stdout modes.
func RunAmass(ctx Context) []string {
	timeout := amassTimeout()

	outputFile, err := os.CreateTemp("", "hunt-amass-*.txt")
	if err != nil {
		log.Printf("⚠️ Failed to create Amass output file for %s: %v\n", ctx.Domain, err)
		return nil
	}

	outputPath := outputFile.Name()
	_ = outputFile.Close()
	defer os.Remove(outputPath)

	log.Printf(" Starting AMASS passive enumeration for %s (timeout=%s)...\n", ctx.Domain, timeout)

	seen := make(map[string]struct{})
	results := make([]string, 0)

	type attempt struct {
		name      string
		args      []string
		parseFile bool
	}

	attempts := []attempt{
		{
			name: "v4 file output",
			args: []string{
				"enum",
				"-passive",
				"-norecursive",
				"-noalts",
				"-d", ctx.Domain,
				"-o", outputPath,
			},
			parseFile: true,
		},
		{
			name: "v5 passive stdout",
			args: []string{
				"enum",
				"-passive",
				"-d", ctx.Domain,
			},
			parseFile: false,
		},
		{
			name: "v5 default stdout",
			args: []string{
				"enum",
				"-d", ctx.Domain,
			},
			parseFile: false,
		},
		{
			name: "v5 subs stdout",
			args: []string{
				"subs",
				"-d", ctx.Domain,
			},
			parseFile: false,
		},
	}

	for _, attempt := range attempts {
		log.Printf(" Running AMASS attempt for %s: %s\n", ctx.Domain, attempt.name)

		output, commandErr := runAmassCommand(ctx, timeout, attempt.args)

		if attempt.parseFile {
			parseAmassFile(ctx, outputPath, seen, &results)
		} else {
			parseAmassLines(ctx, output, seen, &results)
		}

		if commandErr == nil {
			log.Printf("✅ AMASS attempt succeeded for %s using %s. Results so far: %d\n", ctx.Domain, attempt.name, len(results))
			break
		}

		outputText := strings.TrimSpace(string(output))
		if outputText != "" {
			log.Printf("⚠️ AMASS output for %s using %s:\n%s\n", ctx.Domain, attempt.name, outputText)
		}

		if strings.Contains(commandErr.Error(), "process killed by user request") {
			log.Printf("⏹️ AMASS stopped for %s\n", ctx.Domain)
			return nil
		}

		if strings.Contains(commandErr.Error(), "timed out") {
			log.Printf("⚠️ AMASS timed out for %s after %s using %s. Continuing with partial output.\n", ctx.Domain, timeout, attempt.name)
			break
		}

		log.Printf("⚠️ AMASS attempt failed for %s using %s: %v\n", ctx.Domain, attempt.name, commandErr)
	}

	log.Printf("✅ AMASS found %d domains for %s\n", len(results), ctx.Domain)

	return results
}
