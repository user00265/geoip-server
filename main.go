package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/user00265/geoip-server/backend"
	v1 "github.com/user00265/geoip-server/backend/v1"
	v2 "github.com/user00265/geoip-server/backend/v2"
	"github.com/user00265/geoip-server/frontend"
)

// DXClusterGoAPI logrus formatter and hook
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

func main() {
	log.SetFormatter(&dxStyleFormatter{})
	log.SetOutput(os.Stdout)
	log.AddHook(&stderrHook{})

	// Set log level from LOGLEVEL env, default warn
	lvl := os.Getenv("LOGLEVEL")
	switch strings.ToLower(lvl) {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn", "warning", "":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.WarnLevel)
	}

	// Healthcheck CLI command
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		fmt.Println("OK")
		os.Exit(0)
	}

	cfg, err := backend.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	go v1.UpdateDatabases(cfg.GeoIPv1URLs)

	if cfg.MaxMindAccountID != "" && cfg.MaxMindLicenseKey != "" {
		v2Urls := map[string]string{
			"https://download.maxmind.com/geoip/databases/GeoLite2-ASN/download?suffix=tar.gz":     "GeoLite2-ASN.mmdb",
			"https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz":    "GeoLite2-City.mmdb",
			"https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz": "GeoLite2-Country.mmdb",
		}
		go v2.UpdateDatabases(v2Urls, cfg.MaxMindAccountID, cfg.MaxMindLicenseKey)
	} else {
		log.Warn("MAXMIND_ACCOUNT_ID and MAXMIND_LICENSE_KEY are required for v2 database updates. V2 updates will be skipped.")
		log.Warn("Skipping v2 database updates due to missing credentials.")
	}

	serverCfg := &frontend.ServerConfig{
		DatabaseDir: cfg.DatabaseDir,
	}
	server := frontend.NewServer(serverCfg)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to run: %v", err)
	}
}
