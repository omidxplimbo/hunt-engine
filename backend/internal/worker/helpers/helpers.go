package helpers

import (
	"os"
	"strconv"
	"strings"
)

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ParseResponseTime(timeStr string) int64 {
	str := strings.TrimSuffix(timeStr, "ms")
	if val, err := strconv.ParseFloat(str, 64); err == nil {
		return int64(val)
	}
	return 0
}

func ShortenString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

func GetBatchSize(defaultBatchSize int) int {
	envStr := os.Getenv("PROBE_BATCH_SIZE")
	if envStr == "" {
		return defaultBatchSize
	}

	size, err := strconv.Atoi(envStr)
	if err != nil || size <= 0 {
		return defaultBatchSize
	}

	return size
}

func GetDNSXBatchSize(defaultBatchSize int) int {
	envStr := os.Getenv("DNSX_BATCH_SIZE")
	if envStr == "" {
		return defaultBatchSize
	}

	size, err := strconv.Atoi(envStr)
	if err != nil || size <= 0 {
		return defaultBatchSize
	}

	return size
}

func MergeUnique(slice1, slice2 []string) []string {
	uniqueMap := make(map[string]bool)

	for _, v := range slice1 {
		uniqueMap[v] = true
	}

	for _, v := range slice2 {
		uniqueMap[v] = true
	}

	final := make([]string, 0, len(uniqueMap))
	for k := range uniqueMap {
		final = append(final, k)
	}

	return final
}
