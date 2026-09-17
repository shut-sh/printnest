package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"printnest/internal/api"
	"printnest/internal/auth"
	"printnest/internal/cleanup"
	"printnest/internal/config"
	"printnest/internal/db"
	"printnest/internal/jobs"
	"printnest/internal/print"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("upload dir: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()
	if cfg.PasswordSeed != "" {
		if err := auth.EnsurePassword(store, cfg.PasswordSeed); err != nil {
			log.Fatalf("password seed: %v", err)
		}
	} else {
		var cur string
		if err := store.SQL.QueryRow(`SELECT value FROM settings WHERE key='admin_password_hash'`).Scan(&cur); err != nil || cur == "" {
			log.Fatalf("no admin password configured: set PRINTNEST_PASSWORD on first boot")
		}
	}
	if cfg.RetentionMinutes > 0 {
		_ = store.SetSetting("file_retention_minutes", fmt.Sprint(cfg.RetentionMinutes))
	}

	backend := print.NewFailoverBackend(cfg.CUPSHost, cfg.CUPSPort, cfg.CUPSUser)
	jobStore := jobs.NewStore(store, backend, cfg.DataDir, cfg.MaxUploadBytes)

	srv := &api.Server{DB: store, Jobs: jobStore, Backend: backend, MaxBytes: cfg.MaxUploadBytes, DefRet: cfg.RetentionMinutes}
	mux := http.NewServeMux()
	srv.Routes(mux)

	dist := cfg.FrontendDir
	if !filepath.IsAbs(dist) {
		if exe, err := os.Executable(); err == nil {
			cand := filepath.Join(filepath.Dir(exe), "frontend", "dist")
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				dist = cand
			} else if st, err := os.Stat("frontend/dist"); err == nil && st.IsDir() {
				dist = "frontend/dist"
			}
		}
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(dist, filepath.Clean(r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() && p != dist {
			http.ServeFile(w, r, p)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})

	stop := make(chan struct{})
	defer close(stop)
	go cleanup.Start(store, cfg.DataDir, func() int { return store.RetentionMinutes(cfg.RetentionMinutes) }, stop)
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			auth.SweepExpired(store)
		}
	}()

	addr := net.JoinHostPort(cfg.Bind, fmt.Sprint(cfg.Port))
	log.Printf("printnest listening on http://%s (frontend %s)", addr, dist)
	log.Fatal(http.ListenAndServe(addr, mux))
}
