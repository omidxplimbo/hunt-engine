package passive

import (
	"bufio"
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

// RunAmass runs OWASP Amass in passive mode and returns discovered subdomains.
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

	args := []string{
		"enum",
		"-passive",
		"-norecursive",
		"-noalts",
		"-d", ctx.Domain,
		"-o", outputPath,
	}

	var commandErr error
	if ctx.RunCombinedCommandWithTimeout != nil {
		_, commandErr = ctx.RunCombinedCommandWithTimeout(timeout, "amass", args...)
	} else if ctx.RunCombinedCommand != nil {
		_, commandErr = ctx.RunCombinedCommand("amass", args...)
	} else if ctx.RunCommand != nil {
		_, commandErr = ctx.RunCommand("amass", args...)
	}

	if commandErr != nil {
		if strings.Contains(commandErr.Error(), "process killed by user request") {
			log.Printf("⏹️ AMASS stopped for %s\n", ctx.Domain)
			return nil
		}

		if strings.Contains(commandErr.Error(), "timed out") {
			log.Printf("⚠️ AMASS timed out for %s after %s. Continuing with partial output.\n", ctx.Domain, timeout)
		} else {
			log.Printf("⚠️ AMASS failed for %s: %v. Continuing with partial output if available.\n", ctx.Domain, commandErr)
		}
	}

	file, err := os.Open(outputPath)
	if err != nil {
		log.Printf("⚠️ Failed to open Amass output for %s: %v\n", ctx.Domain, err)
		return nil
	}
	defer file.Close()

	seen := make(map[string]struct{})
	results := make([]string, 0)

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
		results = append(results, value)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ Failed while reading Amass output for %s: %v\n", ctx.Domain, err)
	}

	log.Printf("✅ AMASS found %d domains for %s\n", len(results), ctx.Domain)

	return results
}
