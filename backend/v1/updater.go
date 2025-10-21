package v1

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&dxStyleFormatter{})
	log.SetOutput(os.Stdout)
	log.AddHook(&stderrHook{})
}

type dxStyleFormatter struct{}

func (f *dxStyleFormatter) Format(entry *log.Entry) ([]byte, error) {
	// Timestamp: Oct 20 19:46:52.713
	ts := entry.Time.Format("Jan 02 15:04:05.000")
	// Level: 3-letter uppercase
	var lvl string
	switch entry.Level {
	case log.InfoLevel:
		lvl = "NOT"
	case log.WarnLevel:
		lvl = "WRN"
	case log.ErrorLevel:
		lvl = "ERR"
	case log.FatalLevel:
		lvl = "CRT"
	case log.PanicLevel:
		lvl = "PAN"
	case log.DebugLevel:
		lvl = "DBG"
	case log.TraceLevel:
		lvl = "TRC"
	default:
		lvl = strings.ToUpper(entry.Level.String())
	}
	// Message
	line := fmt.Sprintf("%s %s %s\n", ts, lvl, entry.Message)
	return []byte(line), nil
}

type stderrHook struct{}

func (h *stderrHook) Levels() []log.Level {
	return []log.Level{log.ErrorLevel, log.FatalLevel, log.PanicLevel}
}

func (h *stderrHook) Fire(entry *log.Entry) error {
	// Write error/crit/panic logs to STDERR
	line, err := entry.String()
	if err == nil {
		os.Stderr.WriteString(line)
	}
	return nil
}

const (
	DbDir     = "db/v1"
	Timestamp = "db/v1.txt"
)

// Download and decompress a single v1 file, using If-Modified-Since and Last-Modified
func downloadAndDecompress(url string, destDir string, lastUpdateTime time.Time) (bool, time.Time, error) {
	client := &http.Client{Timeout: 300 * time.Second}
	progressParts := strings.Split(url, "/")
	progressGzName := progressParts[len(progressParts)-1]
	progressOutName := strings.TrimSuffix(progressGzName, ".gz")
	log.Debugf("Preparing request for %s", progressOutName)

	// User agent variables (to be set by CI/build)
	var version = "dev"     // set via -ldflags in CI
	var commit = "a1a1a1a1" // set via -ldflags in CI
	userAgent := fmt.Sprintf("geoip-server/%s-%s (+https://github.com/user00265/geoip-server)", version, commit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("Request creation failed for %s: %v", progressOutName, err)
		return false, time.Time{}, fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)
	if !lastUpdateTime.IsZero() {
		req.Header.Set("If-Modified-Since", lastUpdateTime.UTC().Format(http.TimeFormat))
		log.Debugf("Set If-Modified-Since for %s: %s", progressOutName, lastUpdateTime.UTC().Format(http.TimeFormat))
	}

	log.Infof("Downloading %s...", progressOutName)
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Download failed for %s: %v", progressOutName, err)
		return false, time.Time{}, fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()
	log.Debugf("Response received for %s: status %d", progressOutName, resp.StatusCode)

	if resp.StatusCode == http.StatusNotModified {
		return false, lastUpdateTime, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, time.Time{}, fmt.Errorf("bad status downloading %s: %s", url, resp.Status)
	}

	var newModTime time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		t, err := time.Parse(http.TimeFormat, lm)
		if err == nil {
			newModTime = t
		}
	}
	if newModTime.IsZero() {
		newModTime = time.Now().UTC()
	}

	parts := strings.Split(url, "/")
	gzName := parts[len(parts)-1]
	outName := strings.TrimSuffix(gzName, ".gz")
	outPath := filepath.Join(destDir, outName)

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to create gzip reader for %s: %w", url, err)
	}
	defer gzReader.Close()

	outFile, err := os.Create(outPath)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to create file %s: %w", outPath, err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, gzReader)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to decompress %s: %w", url, err)
	}

	return true, newModTime, nil
}

func updateTimestamp(ts time.Time) error {
	unixTs := fmt.Sprintf("%d", ts.Unix())
	return os.WriteFile(Timestamp, []byte(unixTs), 0644)
}

func parseUnixTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0).UTC(), nil
}

// Main v1 update function with retry queue for failed downloads
func UpdateDatabases(urls []string) {
	if err := os.MkdirAll(DbDir, 0755); err != nil {
		log.Errorf("Failed to create v1 db dir: %v", err)
		return
	}

	var lastUpdateTime time.Time
	if data, err := os.ReadFile(Timestamp); err == nil {
		ts, err := parseUnixTimestamp(string(data))
		if err == nil {
			lastUpdateTime = ts
		}
	}

	var newestModTime time.Time
	anyUpdated := false
	retryQueue := make(map[string]int)

	for _, url := range urls {
		parts := strings.Split(url, "/")
		gzName := parts[len(parts)-1]
		outName := strings.TrimSuffix(gzName, ".gz")
		log.Infof("Checking update for %s", outName)
		updated, modTime, err := downloadAndDecompress(url, DbDir, lastUpdateTime)
		if err != nil {
			log.Warnf("Download failed for %s: %v. Adding to retry queue.", outName, err)
			retryQueue[url] = 1
			continue
		}
		if updated {
			log.Infof("Updated %s", outName)
			anyUpdated = true
			if modTime.After(newestModTime) {
				newestModTime = modTime
			}
		} else {
			log.Debugf("No update needed for %s", outName)
		}
	}

	if anyUpdated && !newestModTime.IsZero() {
		if err := updateTimestamp(newestModTime); err != nil {
			log.Errorf("Failed to update v1.txt: %v", err)
		}
	}

	if len(retryQueue) > 0 {
		go func() {
			for len(retryQueue) > 0 {
				time.Sleep(3600 * time.Second)
				for url := range retryQueue {
					log.Infof("Retrying v1 download for %s", url)
					updated, modTime, err := downloadAndDecompress(url, DbDir, lastUpdateTime)
					if err != nil {
						retryQueue[url]++
						log.Warnf("Retry failed for %s (attempt %d): %v", url, retryQueue[url], err)
						continue
					}
					delete(retryQueue, url)
					if updated && modTime.After(newestModTime) {
						newestModTime = modTime
						if err := updateTimestamp(newestModTime); err != nil {
							log.Errorf("Failed to update v1.txt after retry: %v", err)
						}
					}
				}
			}
		}()
	}
}
