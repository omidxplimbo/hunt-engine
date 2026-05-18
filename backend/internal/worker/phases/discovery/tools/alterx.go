package tools

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunAlterx writes alterx candidates to outputFile and returns the number of
// normalized candidates written. It intentionally avoids capturing the full
// alterx stdout in memory, because large targets can produce millions of
// candidates.
func RunAlterx(ctx Context, inputFile, rootDomain, outputFile string) (int, error) {
	if err := ctx.ensureCommandRunner(); err != nil {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		return 0, err
	}

	rawOutputFile := outputFile + ".raw"
	_ = os.Remove(rawOutputFile)
	_ = os.Remove(outputFile)
	defer os.Remove(rawOutputFile)

	_, err := ctx.RunCommand(ctx.TargetID, "alterx", "-l", inputFile, "-silent", "-o", rawOutputFile)
	if err != nil {
		return 0, err
	}

	count, err := normalizeAlterxOutput(ctx, rawOutputFile, outputFile, rootDomain)
	if err != nil {
		return 0, err
	}

	log.Printf("✅ Alterx produced %d normalized candidates for %s\n", count, rootDomain)
	return count, nil
}

func normalizeAlterxOutput(ctx Context, inputFile, outputFile, rootDomain string) (int, error) {
	in, err := os.Open(inputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	writer := bufio.NewWriterSize(out, 1024*1024)
	defer writer.Flush()

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	const progressEvery = 50000
	startedAt := time.Now()
	lastProgressAt := time.Now()
	rawCount := 0
	count := 0

	ctx.updatePhase("PHASE 1: ALTERX POST-PROCESSING")
	ctx.heartbeat()

	for scanner.Scan() {
		rawCount++

		if ctx.stopped() {
			ctx.updatePhase("PHASE 1: ALTERX POST-PROCESSING STOPPED")
			return count, fmt.Errorf("process killed by user request")
		}

		candidate := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if candidate == "" || !strings.HasSuffix(candidate, rootDomain) {
			continue
		}

		candidate = ctx.normalize(candidate, rootDomain)
		if candidate == "" {
			continue
		}

		if _, err := writer.WriteString(candidate + "\n"); err != nil {
			return count, err
		}
		count++

		if rawCount%progressEvery == 0 || time.Since(lastProgressAt) >= 10*time.Second {
			_ = writer.Flush()
			ctx.heartbeat()
			phase := fmt.Sprintf("PHASE 1: ALTERX POST-PROCESSING (%d UNIQUE / %d RAW)", count, rawCount)
			ctx.updatePhase(phase)
			log.Printf(" Alterx post-processing progress for %s: raw=%d normalized=%d elapsed=%s\n", rootDomain, rawCount, count, time.Since(startedAt).Round(time.Second))
			lastProgressAt = time.Now()
		}
	}

	if err := scanner.Err(); err != nil {
		return count, err
	}

	_ = writer.Flush()
	ctx.heartbeat()
	ctx.updatePhase(fmt.Sprintf("PHASE 1: ALTERX POST-PROCESSING DONE (%d CANDIDATES)", count))
	log.Printf("✅ Alterx post-processing completed for %s: raw=%d normalized=%d elapsed=%s\n", rootDomain, rawCount, count, time.Since(startedAt).Round(time.Second))

	return count, nil
}

func CountLines(filename string) int {
	file, err := os.Open(filename)
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	count := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}

	return count
}
