package analytics

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxLogLine = 8192

var combinedRe = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "([A-Z]+) ([^" ]+)(?: [^"]*)?" (\d{3}) (\d+|-)\s+"[^"]*" "([^"]*)"\s*$`)

// ParseCombined reads one Combined Log Format line.
// The returned event has no latency. The path query string is already removed.
func ParseCombined(line string) (Event, bool) {
	line = strings.TrimRight(line, "\r")
	if line == "" || len(line) > maxLogLine {
		return Event{}, false
	}
	match := combinedRe.FindStringSubmatch(line)
	if match == nil {
		return Event{}, false
	}
	at, err := time.Parse("02/Jan/2006:15:04:05 -0700", match[2])
	if err != nil {
		return Event{}, false
	}
	status, err := strconv.Atoi(match[5])
	if err != nil {
		return Event{}, false
	}
	var nbytes int64
	if match[6] != "-" {
		nbytes, err = strconv.ParseInt(match[6], 10, 64)
		if err != nil || nbytes < 0 {
			return Event{}, false
		}
	}
	return Event{
		At:        at.UTC(),
		Project:   "proxy",
		App:       "proxy",
		Path:      SanitizePath(match[4]),
		Status:    status,
		Bytes:     nbytes,
		RemoteIP:  match[1],
		UserAgent: match[7],
	}, true
}

// SafeLogPath accepts an absolute file path from operator configuration.
// Relative paths and any path containing ".." are refused. A missing file is an error.
func SafeLogPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("access log tail is off")
	}
	if strings.Contains(path, "..") || strings.ContainsRune(path, 0) {
		return "", errors.New("access log path is invalid")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("access log path must be absolute")
	}
	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil || info.IsDir() {
		return "", errors.New("access log is not available")
	}
	return clean, nil
}

// Follow reads new lines from an access log. It starts at the end of the file,
// so existing history is not imported. path comes from operator configuration.
func Follow(ctx context.Context, path string, rec *Recorder, logger *log.Logger) {
	clean, err := SafeLogPath(path)
	if err != nil {
		if logger != nil {
			logger.Printf("access log: %v", err)
		}
		return
	}
	file, err := os.Open(clean)
	if err != nil {
		if logger != nil {
			logger.Printf("access log: unavailable")
		}
		return
	}
	defer file.Close()
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return
	}
	if logger != nil {
		logger.Printf("access log: tailing new lines")
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var partial string
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, statErr := file.Stat()
			if statErr != nil {
				continue
			}
			if info.Size() < offset {
				offset = 0
			}
			if info.Size() == offset {
				continue
			}
			buf := make([]byte, info.Size()-offset)
			n, readErr := file.ReadAt(buf, offset)
			if n > 0 {
				offset += int64(n)
				partial += string(buf[:n])
				for {
					i := strings.IndexByte(partial, '\n')
					if i < 0 {
						break
					}
					line := partial[:i]
					partial = partial[i+1:]
					ev, ok := ParseCombined(line)
					if !ok || rec == nil {
						continue
					}
					if err := rec.Record(ctx, ev); err != nil && logger != nil {
						logger.Printf("access log: record failed")
					}
				}
				if len(partial) > maxLogLine {
					partial = ""
				}
			}
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				continue
			}
		}
	}
}
