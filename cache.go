package wallpaper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type CachedImage struct {
	Path      string
	Timestamp time.Time
	HasStamp  bool
	ModTime   time.Time
}

func (c CachedImage) EffectiveTime() time.Time {
	if c.HasStamp {
		return c.Timestamp
	}
	return c.ModTime
}

func selectNewestCachedImage(imageDir string) (CachedImage, error) {
	cached, err := listCachedImages(imageDir)
	if err != nil {
		return CachedImage{}, err
	}
	if len(cached) == 0 {
		return CachedImage{}, errors.New("no cached images available")
	}
	sort.Slice(cached, func(i, j int) bool {
		return cached[i].EffectiveTime().After(cached[j].EffectiveTime())
	})
	return cached[0], nil
}

func listCachedImages(imageDir string) ([]CachedImage, error) {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cached []CachedImage
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == filepath.Base(defaultCurrentLink(imageDir)) || filepath.Ext(name) != ".png" || !strings.HasPrefix(name, "earth_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		timestamp, ok := parseImageTimestampFromName(name)
		cached = append(cached, CachedImage{
			Path:      filepath.Join(imageDir, name),
			Timestamp: timestamp,
			HasStamp:  ok,
			ModTime:   info.ModTime(),
		})
	}
	return cached, nil
}

func defaultCurrentLink(imageDir string) string {
	return filepath.Join(imageDir, "current.png")
}

func updateCurrentLink(linkPath, target string) error {
	if err := ensureDir(filepath.Dir(linkPath)); err != nil {
		return err
	}
	absTarget := resolvePath(target)
	tmpLink := linkPath + ".tmp"
	_ = os.Remove(tmpLink)
	if err := os.Symlink(absTarget, tmpLink); err != nil {
		return err
	}
	if err := os.Rename(tmpLink, linkPath); err != nil {
		_ = os.Remove(tmpLink)
		return err
	}
	return nil
}

func cleanupRenderedImages(renderDir, keepPath string) error {
	entries, err := os.ReadDir(renderDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(renderDir, entry.Name())
		if path == keepPath {
			continue
		}
		_ = os.Remove(path)
	}
	return nil
}

func rotateCache(imageDir string, now time.Time, logger *Logger) error {
	cached, err := listCachedImages(imageDir)
	if err != nil {
		return err
	}
	type keeper struct {
		Cache CachedImage
		Keep  bool
	}
	items := make([]keeper, 0, len(cached))
	for _, image := range cached {
		items = append(items, keeper{Cache: image})
	}

	weekly := make(map[string]int)
	monthly := make(map[string]int)
	for i, item := range items {
		age := now.Sub(item.Cache.EffectiveTime())
		if age <= 24*time.Hour {
			items[i].Keep = true
			continue
		}
		if age <= 30*24*time.Hour {
			year, week := item.Cache.EffectiveTime().ISOWeek()
			key := fmt.Sprintf("%04d-%02d", year, week)
			current, found := weekly[key]
			if !found || items[current].Cache.EffectiveTime().Before(item.Cache.EffectiveTime()) {
				if found {
					items[current].Keep = false
				}
				items[i].Keep = true
				weekly[key] = i
			}
			continue
		}
		key := item.Cache.EffectiveTime().Format("2006-01")
		current, found := monthly[key]
		if !found || items[current].Cache.EffectiveTime().Before(item.Cache.EffectiveTime()) {
			if found {
				items[current].Keep = false
			}
			items[i].Keep = true
			monthly[key] = i
		}
	}

	for _, item := range items {
		if item.Keep {
			continue
		}
		logger.Debugf("removing old image: %s", item.Cache.Path)
		_ = os.Remove(item.Cache.Path)
	}
	return nil
}
