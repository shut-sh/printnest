package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Bind             string
	Port             int
	DataDir          string
	UploadDir        string
	DBPath           string
	FrontendDir      string
	CUPSHost         string
	CUPSPort         int
	CUPSUser         string
	IPPTimeoutSecs   int
	PasswordSeed     string
	RetentionMinutes int
	MaxUploadBytes   int64
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiOr(env string, def int) int {
	v, err := strconv.Atoi(getenv(env, ""))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func Load() Config {
	dataDir := getenv("DATA_DIR", "/var/lib/printnest")
	maxMB := atoiOr("MAX_UPLOAD_MB", 50)
	return Config{
		Bind:             getenv("BIND", "127.0.0.1"),
		Port:             atoiOr("PORT", 8080),
		DataDir:          dataDir,
		UploadDir:        filepath.Join(dataDir, "uploads"),
		DBPath:           filepath.Join(dataDir, "printnest.db"),
		FrontendDir:      getenv("FRONTEND_DIR", "frontend/dist"),
		CUPSHost:         getenv("CUPS_HOST", "localhost"),
		CUPSPort:         atoiOr("CUPS_PORT", 631),
		CUPSUser:         getenv("CUPS_USER", "printnest"),
		IPPTimeoutSecs:   atoiOr("IPP_TIMEOUT_SECONDS", 25),
		PasswordSeed:     os.Getenv("PRINTNEST_PASSWORD"),
		RetentionMinutes: atoiOr("FILE_RETENTION_MINUTES", 30),
		MaxUploadBytes:   int64(maxMB) << 20,
	}
}
