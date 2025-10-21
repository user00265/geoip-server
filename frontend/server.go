package frontend

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	var lvl, color string
	switch entry.Level {
	case log.InfoLevel:
		lvl = "NOT"
		color = "\033[32m" // Green
	case log.WarnLevel:
		lvl = "WRN"
		color = "\033[33m" // Yellow
	case log.ErrorLevel:
		lvl = "ERR"
		color = "\033[31m" // Red
	case log.FatalLevel:
		lvl = "CRT"
		color = "\033[35m" // Magenta
	case log.PanicLevel:
		lvl = "PAN"
		color = "\033[41m\033[97m" // White on Red background
	case log.DebugLevel:
		lvl = "DBG"
		color = "\033[36m" // Cyan
	case log.TraceLevel:
		lvl = "TRC"
		color = "\033[34m" // Blue
	default:
		lvl = entry.Level.String()
		color = ""
	}
	reset := "\033[0m"
	line := fmt.Sprintf("%s %s%s%s %s\n", ts, color, lvl, reset, entry.Message)
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

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	DatabaseDir string // Base directory where v1 and v2 databases are stored.
}

// NewServer creates a new HTTP server instance.

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(recorder, r)
		elapsed := time.Since(start)
		status := recorder.status
		// Choose log level based on status
		var logFunc func(string, ...interface{})
		switch {
		case status >= 500:
			logFunc = log.Errorf
		case status >= 400:
			logFunc = log.Warnf
		default:
			logFunc = log.Infof
		}
		var durStr string
		ns := elapsed.Nanoseconds()
		ms := elapsed.Milliseconds()
		s := elapsed.Seconds()
		switch {
		case ns < 1_000_000:
			durStr = fmt.Sprintf("%dns", ns)
		case ms < 1000:
			durStr = fmt.Sprintf("%dms", ms)
		default:
			durStr = fmt.Sprintf("%.3fs", s)
		}
		// Format: Oct 20 17:09:18.717 <LEVEL> GET /path - <status> (<duration>) - <remoteAddr>
		// Remove port from remote address
		remoteIP := r.RemoteAddr
		if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
			remoteIP = remoteIP[:idx]
		}
		logFunc(
			"%s %s - %d (%s) - %s",
			r.Method,
			r.URL.Path,
			status,
			durStr,
			remoteIP,
		)
	})
}

func NewServer(cfg *ServerConfig) *http.Server {
	mux := http.NewServeMux()

	// ...existing code...

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanedPath := filepath.Clean(r.URL.Path)
		// Remove leading slash (lint: S1017)
		cleanedPath = strings.TrimPrefix(cleanedPath, "/")
		// Block path traversal
		if cleanedPath == "." || cleanedPath == "" || strings.Contains(cleanedPath, "..") {
			http.NotFound(w, r)
			return
		}
		// Only allow known filenames
		allowedV1 := map[string]bool{
			"GeoIP.dat": true, "GeoIPv6.dat": true, "GeoIPCity.dat": true, "GeoIPCityv6.dat": true,
			"GeoIPASNum.dat": true, "GeoIPASNumv6.dat": true, "GeoIPISP.dat": true, "GeoIPISPv6.dat": true,
			"GeoIPOrg.dat": true, "GeoIPOrgv6.dat": true,
		}
		allowedV2 := map[string]bool{
			"GeoLite2-ASN.mmdb": true, "GeoLite2-City.mmdb": true, "GeoLite2-Country.mmdb": true,
		}
		var requestedFilePath string
		if allowedV1[cleanedPath] {
			requestedFilePath = filepath.Join("db/v1", cleanedPath)
		} else if allowedV2[cleanedPath] {
			requestedFilePath = filepath.Join("db/v2", cleanedPath)
		} else {
			log.Debugf("Unknown or disallowed file: %s", cleanedPath)
			http.NotFound(w, r)
			return
		}
		log.Debugf("Looking for file: %s", requestedFilePath)
		fileInfo, err := os.Stat(requestedFilePath)
		if err == nil && fileInfo.IsDir() {
			http.NotFound(w, r)
			return
		}
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("If-Modified-Since") != "" {
			if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil {
				modTime := fileInfo.ModTime().UTC().Truncate(time.Second)
				parsedTime := t.UTC().Truncate(time.Second)
				if modTime.Equal(parsedTime) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
		w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))
		http.ServeFile(w, r, requestedFilePath)
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "7502"
	}
	addr := "0.0.0.0:" + port

	server := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Infof("Starting GeoIP Server on port %s", port)
	return server
}
