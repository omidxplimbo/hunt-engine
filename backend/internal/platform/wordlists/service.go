package wordlists

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

const (
	DefaultStorageDir       = "/data/wordlists"
	DefaultMaxFileSizeBytes = int64(10 * 1024 * 1024)
	DefaultMaxTotalBytes    = int64(100 * 1024 * 1024)
)

type StoredFile struct {
	DisplayName string
	StoredName  string
	Path        string
	SizeBytes   int64
	SHA256      string
	Lines       int64
}

func RootDir() string {
	raw := strings.TrimSpace(os.Getenv("WORDLISTS_STORAGE_DIR"))
	if raw == "" {
		return DefaultStorageDir
	}
	return raw
}

func UserDir(userID uint) string {
	return filepath.Join(RootDir(), "users", fmt.Sprintf("%d", userID))
}

func EnsureUserDir(userID uint) error {
	return os.MkdirAll(UserDir(userID), 0750)
}

func DeleteUserDir(userID uint) error {
	if userID == 0 {
		return fmt.Errorf("invalid user id")
	}
	return os.RemoveAll(UserDir(userID))
}

func SafeDisplayName(name string) (string, error) {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("wordlist name is required")
	}

	if strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid wordlist name")
	}

	if strings.ToLower(filepath.Ext(name)) != ".txt" {
		return "", fmt.Errorf("only .txt wordlists are allowed")
	}

	if len(name) > 255 {
		return "", fmt.Errorf("wordlist name is too long")
	}

	return name, nil
}

func NewStoredName(displayName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(displayName))
	if ext == "" {
		ext = ".txt"
	}

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(displayName), filepath.Ext(displayName))
	base = sanitizeBase(base)
	if base == "" {
		base = "wordlist"
	}

	return fmt.Sprintf("%s-%s-%s%s", time.Now().UTC().Format("20060102150405"), hex.EncodeToString(buf), base, ext), nil
}

func PathForStoredName(userID uint, storedName string) (string, error) {
	storedName = filepath.Base(strings.TrimSpace(storedName))
	if storedName == "" || storedName == "." || storedName == "/" {
		return "", fmt.Errorf("invalid stored name")
	}

	path := filepath.Join(UserDir(userID), storedName)
	cleanUserDir := filepath.Clean(UserDir(userID)) + string(os.PathSeparator)
	cleanPath := filepath.Clean(path)

	if !strings.HasPrefix(cleanPath, cleanUserDir) {
		return "", fmt.Errorf("invalid wordlist path")
	}

	return cleanPath, nil
}

func EffectiveLimits(user models.User) (maxFile int64, maxTotal int64, unlimited bool) {
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		return 0, 0, true
	}

	maxFile = user.WordlistMaxFileSizeBytes
	if maxFile <= 0 {
		maxFile = DefaultMaxFileSizeBytes
	}

	maxTotal = user.WordlistMaxTotalSizeBytes
	if maxTotal <= 0 {
		maxTotal = DefaultMaxTotalBytes
	}

	return maxFile, maxTotal, false
}

func CurrentTotalSize(userID uint) (int64, error) {
	var total int64
	err := database.DB.Model(&models.UserWordlist{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(size_bytes), 0)").
		Scan(&total).Error
	return total, err
}

func CheckQuota(currentTotal int64, incomingSize int64, maxFile int64, maxTotal int64, unlimited bool) error {
	if unlimited {
		return nil
	}

	if maxFile > 0 && incomingSize > maxFile {
		return fmt.Errorf("file exceeds max file size limit")
	}

	if maxTotal > 0 && currentTotal+incomingSize > maxTotal {
		return fmt.Errorf("upload exceeds total wordlist storage limit")
	}

	return nil
}

func SaveReader(userID uint, displayName string, r io.Reader, maxFileBytes int64) (*StoredFile, error) {
	displayName, err := SafeDisplayName(displayName)
	if err != nil {
		return nil, err
	}

	if err := EnsureUserDir(userID); err != nil {
		return nil, err
	}

	storedName, err := NewStoredName(displayName)
	if err != nil {
		return nil, err
	}

	finalPath, err := PathForStoredName(userID, storedName)
	if err != nil {
		return nil, err
	}

	tmpPath := finalPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	size, lines, copyErr := copyCountHash(out, r, hasher, maxFileBytes)
	closeErr := out.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, closeErr
	}
	if size <= 0 {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("empty wordlist is not allowed")
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	return &StoredFile{
		DisplayName: displayName,
		StoredName:  storedName,
		Path:        finalPath,
		SizeBytes:   size,
		SHA256:      hex.EncodeToString(hasher.Sum(nil)),
		Lines:       lines,
	}, nil
}

func RemoveFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return os.Remove(path)
}

func copyCountHash(dst io.Writer, src io.Reader, hasher hash.Hash, maxBytes int64) (int64, int64, error) {
	reader := bufio.NewReaderSize(src, 64*1024)
	writer := io.MultiWriter(dst, hasher)

	var size int64
	var lines int64

	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			size += int64(len(chunk))
			if maxBytes > 0 && size > maxBytes {
				return size, lines, fmt.Errorf("file exceeds max file size limit")
			}

			if chunk[len(chunk)-1] == '\n' {
				lines++
			}

			if _, writeErr := writer.Write(chunk); writeErr != nil {
				return size, lines, writeErr
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return size, lines, err
		}
	}

	return size, lines, nil
}

func sanitizeBase(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder

	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
			continue
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}

	out := strings.Trim(b.String(), "-_.")
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}
