package events

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	isoTimestampPattern    = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ][0-9:.+-Z]+\b`)
	syslogTimestampPattern = regexp.MustCompile(`(?i)\b[a-z]{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\b`)
	hexPattern             = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
	numberPattern          = regexp.MustCompile(`\b\d+\b`)
	spacePattern           = regexp.MustCompile(`\s+`)
)

func Fingerprint(service, line string) string {
	normalized := Normalize(line)
	sum := sha256.Sum256([]byte(strings.TrimSpace(service) + "\x00" + normalized))
	return hex.EncodeToString(sum[:])
}

func Normalize(line string) string {
	line = strings.ToLower(strings.TrimSpace(line))
	line = isoTimestampPattern.ReplaceAllString(line, "<time>")
	line = syslogTimestampPattern.ReplaceAllString(line, "<time>")
	line = hexPattern.ReplaceAllString(line, "<hex>")
	line = numberPattern.ReplaceAllString(line, "<num>")
	line = spacePattern.ReplaceAllString(line, " ")
	return line
}
