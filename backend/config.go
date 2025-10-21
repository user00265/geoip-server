package backend

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultUpdateInterval is the default interval in seconds for checking/downloading databases.
	DefaultUpdateInterval = 86400 // 24 hours
	// DefaultGeoIPv1URLs is a comma-separated list of default GeoIP v1 database URLs.
	DefaultGeoIPv1URLs = "https://mailfud.org/geoip-legacy/GeoIP.dat.gz,https://mailfud.org/geoip-legacy/GeoIPv6.dat.gz,https://mailfud.org/geoip-legacy/GeoIPCity.dat.gz,https://mailfud.org/geoip-legacy/GeoIPCityv6.dat.gz,https://mailfud.org/geoip-legacy/GeoIPASNum.dat.gz,https://mailfud.org/geoip-legacy/GeoIPASNumv6.dat.gz,https://mailfud.org/geoip-legacy/GeoIPISP.dat.gz,https://mailfud.org/geoip-legacy/GeoIPISPv6.dat.gz,https://mailfud.org/geoip-legacy/GeoIPOrg.dat.gz,https://mailfud.org/geoip-legacy/GeoIPOrgv6.dat.gz"
)

// Config holds the application configuration.
type Config struct {
	UpdateInterval      time.Duration
	GeoIPv1URLs         []string
	MaxMindAccountID    string
	MaxMindLicenseKey   string
	DatabaseDir         string
	V1TimestampFile     string
	V2TimestampFile     string
	V1DBsDir            string
	V2DBsDir            string
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	// Load UpdateInterval
	updateIntervalSeconds := DefaultUpdateInterval
	if envInterval := os.Getenv("UPDATE_INTERVAL"); envInterval != "" {
		if interval, err := strconv.Atoi(envInterval); err == nil && interval > 0 {
			updateIntervalSeconds = interval
		} else {
			log.Printf("Warning: Invalid UPDATE_INTERVAL '%s'. Using default %d seconds.", envInterval, DefaultUpdateInterval)
		}
	}
	updateInterval := time.Duration(updateIntervalSeconds) * time.Second

	// Load GeoIPv1URLs
	geoIPv1URLs := []string{}
	if envURLs := os.Getenv("GEOIPV1_URLS"); envURLs != "" {
		// Split by comma and trim whitespace
		for _, url := range splitAndTrim(envURLs, ",") {
			if url != "" {
				geoIPv1URLs = append(geoIPv1URLs, url)
			}
		}
	}
	if len(geoIPv1URLs) == 0 {
		// Split by comma and trim whitespace for default URLs
		for _, url := range splitAndTrim(DefaultGeoIPv1URLs, ",") {
			if url != "" {
				geoIPv1URLs = append(geoIPv1URLs, url)
			}
		}
	}

	// Load MaxMind credentials
	maxMindAccountID := os.Getenv("MAXMIND_ACCOUNT_ID")
	maxMindLicenseKey := os.Getenv("MAXMIND_LICENSE_KEY")

	// Refuse to start v2 update without credentials
	if maxMindAccountID == "" || maxMindLicenseKey == "" {
		log.Println("Warning: MAXMIND_ACCOUNT_ID and MAXMIND_LICENSE_KEY are required for v2 database updates. V2 updates will be skipped.")
	}

	// Define directories and timestamp files
	dbDir := "db"
	v1TimestampFile := "v1.txt"
	v2TimestampFile := "v2.txt"
	v1DBsDir := dbDir + "/v1"
	v2DBsDir := dbDir + "/v2"

	// Ensure directories exist
	if err := os.MkdirAll(v1DBsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create v1 database directory %s: %w", v1DBsDir, err)
	}
	if err := os.MkdirAll(v2DBsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create v2 database directory %s: %w", v2DBsDir, err)
	}

	return &Config{
		UpdateInterval:      updateInterval,
		GeoIPv1URLs:         geoIPv1URLs,
		MaxMindAccountID:    maxMindAccountID,
		MaxMindLicenseKey:   maxMindLicenseKey,
		DatabaseDir:         dbDir,
		V1TimestampFile:     v1TimestampFile,
		V2TimestampFile:     v2TimestampFile,
		V1DBsDir:            v1DBsDir,
		V2DBsDir:            v2DBsDir,
	}, nil
}

// splitAndTrim splits a string by a separator and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range strings.Split(s, sep) {
		result = append(result, strings.TrimSpace(item))
	}
	return result
}
