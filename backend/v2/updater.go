package v2

import (
	"archive/tar"
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
	ts := entry.Time.Format("Jan 02 15:04:05.000")
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
		lvl = entry.Level.String()
	}
	line := fmt.Sprintf("%s %s %s\n", ts, lvl, entry.Message)
	return []byte(line), nil
}

type stderrHook struct{}

func (h *stderrHook) Levels() []log.Level {
	return []log.Level{log.ErrorLevel, log.FatalLevel, log.PanicLevel}
}
func (h *stderrHook) Fire(entry *log.Entry) error {
	line, err := entry.String()
	if err == nil {
		os.Stderr.WriteString(line)
	}
	return nil
}

const (
	DbDir     = "db/v2"
	Timestamp = "db/v2.txt"
)

// Download and extract a v2 tar.gz file with HTTP Basic Auth and If-Modified-Since
func downloadAndExtract(url, destDir, accountID, licenseKey string, lastUpdateTime time.Time, outFileName string) (bool, time.Time, error) {
	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	if accountID != "" && licenseKey != "" {
		req.SetBasicAuth(accountID, licenseKey)
	}
	if !lastUpdateTime.IsZero() {
		req.Header.Set("If-Modified-Since", lastUpdateTime.UTC().Format(http.TimeFormat))
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

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

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to create gzip reader for %s: %w", url, err)
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)

	var found bool
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, time.Time{}, fmt.Errorf("failed to read tar for %s: %w", url, err)
		}
		if strings.HasSuffix(hdr.Name, outFileName) {
			outPath := filepath.Join(destDir, outFileName)
			outFile, err := os.Create(outPath)
			if err != nil {
				return false, time.Time{}, fmt.Errorf("failed to create file %s: %w", outPath, err)
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return false, time.Time{}, fmt.Errorf("failed to extract %s: %w", outFileName, err)
			}
			found = true
			break
		}
	}
	if !found {
		return false, time.Time{}, fmt.Errorf("file %s not found in archive", outFileName)
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

// Main v2 update function with retry queue for failed downloads
func UpdateDatabases(urls map[string]string, accountID, licenseKey string) {
	if err := os.MkdirAll(DbDir, 0755); err != nil {
		log.Errorf("Failed to create v2 db dir: %v", err)
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

	for url, outFileName := range urls {
		updated, modTime, err := downloadAndExtract(url, DbDir, accountID, licenseKey, lastUpdateTime, outFileName)
		if err != nil {
			log.Warnf("v2 download failed for %s: %v. Adding to retry queue.", url, err)
			retryQueue[url] = 1
			continue
		}
		if updated {
			log.Infof("Updated %s", outFileName)
			anyUpdated = true
			if modTime.After(newestModTime) {
				newestModTime = modTime
			}
		}
	}

	if anyUpdated && !newestModTime.IsZero() {
		if err := updateTimestamp(newestModTime); err != nil {
			log.Errorf("Failed to update v2.txt: %v", err)
		}
	}

	if len(retryQueue) > 0 {
		go func() {
			for len(retryQueue) > 0 {
				time.Sleep(3600 * time.Second)
				for url, outFileName := range urls {
					if _, queued := retryQueue[url]; !queued {
						continue
					}
					log.Infof("Retrying v2 download for %s", url)
					updated, modTime, err := downloadAndExtract(url, DbDir, accountID, licenseKey, lastUpdateTime, outFileName)
					if err != nil {
						retryQueue[url]++
						log.Warnf("Retry failed for %s (attempt %d): %v", url, retryQueue[url], err)
						continue
					}
					delete(retryQueue, url)
					if updated && modTime.After(newestModTime) {
						newestModTime = modTime
						if err := updateTimestamp(newestModTime); err != nil {
							log.Errorf("Failed to update v2.txt after retry: %v", err)
						}
					}
				}
			}
		}()
	}
}
