package cleanup

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"printnest/internal/db"
)

func SweepOnce(store *db.DB, dataDir string, retentionMinutes int) int {
	if retentionMinutes <= 0 {
		retentionMinutes = 30
	}
	ttl := int64(retentionMinutes) * 60
	now := time.Now().Unix()
	cands, err := store.SweepCandidates(now, ttl)
	if err != nil {
		log.Printf("printnest: cleanup query failed: %v", err)
		return 0
	}
	n := 0
	for _, j := range cands {
		dir := filepath.Join(dataDir, "uploads", j.ID)
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("printnest: cleanup %s failed: %v", j.ID, err)
			continue
		}
		j.FileDeletedAt = now
		if err := store.UpdateJob(j); err != nil {
			log.Printf("printnest: cleanup db update %s failed: %v", j.ID, err)
			continue
		}
		n++
	}
	return n
}

func Start(store *db.DB, dataDir string, retentionFn func() int, stop <-chan struct{}) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			SweepOnce(store, dataDir, retentionFn())
		}
	}
}
