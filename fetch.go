package wallpaper

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RemoteMetadata struct {
	Timestamp time.Time
	URL       string
	Filename  string
}

type SelectedImage struct {
	SourcePath    string
	WallpaperPath string
	Timestamp     time.Time
	FromCache     bool
}

func resolveImage(ctx context.Context, cfg Config, session SessionInfo, logger *Logger) (SelectedImage, error) {
	if err := ensureDir(cfg.ImageDir); err != nil {
		return SelectedImage{}, err
	}

	metadata, metadataErr := fetchMetadata(ctx, cfg)
	if metadataErr == nil {
		canonicalPath := filepath.Join(cfg.ImageDir, metadata.Filename)
		if _, err := os.Stat(canonicalPath); err == nil {
			logger.Infof("reusing existing image: %s", canonicalPath)
			return prepareWallpaperImage(ctx, canonicalPath, metadata.Timestamp, cfg, session, logger, true)
		}
		logger.Infof("downloading latest image: %s", metadata.URL)
		if err := downloadAndGenerate(ctx, cfg, metadata, canonicalPath); err == nil {
			return prepareWallpaperImage(ctx, canonicalPath, metadata.Timestamp, cfg, session, logger, false)
		} else {
			logger.Errorf("download failed, falling back to cache: %v", err)
		}
	} else {
		logger.Errorf("metadata fetch failed, falling back to cache: %v", metadataErr)
	}

	cached, err := selectNewestCachedImage(cfg.ImageDir)
	if err != nil {
		return SelectedImage{}, err
	}
	logger.Infof("using cached image: %s", cached.Path)
	return prepareWallpaperImage(ctx, cached.Path, cached.EffectiveTime(), cfg, session, logger, true)
}

func fetchMetadata(ctx context.Context, cfg Config) (RemoteMetadata, error) {
	client := httpClient(cfg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.APIURL, nil)
	if err != nil {
		return RemoteMetadata{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return RemoteMetadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RemoteMetadata{}, fmt.Errorf("unexpected metadata status %s", resp.Status)
	}
	var payload struct {
		Date string `json:"date"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return RemoteMetadata{}, err
	}
	timestamp, err := parseTimestamp(payload.Date)
	if err != nil {
		return RemoteMetadata{}, err
	}
	return RemoteMetadata{
		Timestamp: timestamp,
		URL:       payload.URL,
		Filename:  fmt.Sprintf("earth_%s.png", timestamp.Format("2006-01-02_15-04-05")),
	}, nil
}

func downloadAndGenerate(ctx context.Context, cfg Config, metadata RemoteMetadata, canonicalPath string) error {
	client := httpClient(cfg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadata.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected image status %s", resp.Status)
	}
	imageData, err := jpeg.Decode(resp.Body)
	if err != nil {
		return err
	}
	rendered, err := ProcessSatelliteImage(imageData)
	if err != nil {
		return err
	}
	return writePNGAtomic(canonicalPath, rendered)
}

func httpClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{
		Timeout:   cfg.HTTPTimeout,
		Transport: transport,
	}
}

func prepareWallpaperImage(ctx context.Context, sourcePath string, timestamp time.Time, cfg Config, session SessionInfo, logger *Logger, fromCache bool) (SelectedImage, error) {
	linkTarget := sourcePath
	if cfg.Resolution != "" || cfg.AutoResolution {
		width, height, err := resolveTargetResolution(ctx, cfg, session, logger)
		if err == nil {
			renderDir := filepath.Join(cfg.ImageDir, ".rendered")
			if err := ensureDir(renderDir); err != nil {
				return SelectedImage{}, err
			}
			renderPath := filepath.Join(renderDir, renderedFileName(sourcePath, width, height, cfg.OffsetY))
			if err := renderFittedWallpaper(sourcePath, renderPath, width, height, cfg.OffsetY); err != nil {
				return SelectedImage{}, err
			}
			linkTarget = renderPath
			_ = cleanupRenderedImages(renderDir, renderPath)
		} else {
			logger.Errorf("resolution detection failed, using canonical image: %v", err)
		}
	}
	if err := updateCurrentLink(cfg.CurrentLink, linkTarget); err != nil {
		return SelectedImage{}, err
	}
	return SelectedImage{
		SourcePath:    sourcePath,
		WallpaperPath: cfg.CurrentLink,
		Timestamp:     timestamp,
		FromCache:     fromCache,
	}, nil
}

func renderedFileName(sourcePath string, width, height, offsetY int) string {
	stem := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	return fmt.Sprintf("%s-%dx%d-offset%d.png", stem, width, height, offsetY)
}
