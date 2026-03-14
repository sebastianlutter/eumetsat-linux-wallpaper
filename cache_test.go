package wallpaper

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectNewestCachedImageFallsBackToModTime(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := filepath.Join(tmpDir, "earth_invalid.png")
	newPath := filepath.Join(tmpDir, "earth_2026-03-14_10-00-00.png")
	if err := os.WriteFile(oldPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2026, 3, 14, 9, 0, 0, 0, time.Local)
	newTime := time.Date(2026, 3, 14, 10, 0, 0, 0, time.Local)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	selected, err := selectNewestCachedImage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Path != newPath {
		t.Fatalf("selected %q, want %q", selected.Path, newPath)
	}
}

func TestRotateCacheKeepsHourlyWeeklyAndMonthlyBuckets(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, 3, 14, 15, 0, 0, 0, time.Local)
	keepHourly := []time.Time{
		now.Add(-1 * time.Hour),
		now.Add(-23 * time.Hour),
	}
	keepWeekly := []time.Time{
		now.Add(-10 * 24 * time.Hour),
		now.Add(-17 * 24 * time.Hour),
	}
	dropWeekly := now.Add(-11 * 24 * time.Hour)
	keepMonthly := []time.Time{
		now.AddDate(0, -2, 0),
		now.AddDate(0, -3, 0),
	}
	dropMonthly := now.AddDate(0, -2, -2)

	writeImage := func(ts time.Time) string {
		path := filepath.Join(tmpDir, "earth_"+ts.Format("2006-01-02_15-04-05")+".png")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
		return path
	}

	for _, ts := range keepHourly {
		writeImage(ts)
	}
	for _, ts := range keepWeekly {
		writeImage(ts)
	}
	writeImage(dropWeekly)
	for _, ts := range keepMonthly {
		writeImage(ts)
	}
	dropMonthlyPath := writeImage(dropMonthly)

	logger := NewLogger("debug", io.Discard, io.Discard)
	if err := rotateCache(tmpDir, now, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dropMonthlyPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed", dropMonthlyPath)
	}
}
