package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Bookmark struct for frontend
type Bookmark struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	Style     string  `json:"style"`
	MinZoom   int     `json:"min_zoom"`
	MaxZoom   int     `json:"max_zoom"`
	North     float64 `json:"north"`
	South     float64 `json:"south"`
	East      float64 `json:"east"`
	West      float64 `json:"west"`
	CenterLat float64 `json:"center_lat"`
	CenterLng float64 `json:"center_lng"`
}

// ListBookmarks returns all bookmarks
func (a *App) ListBookmarks() ([]Bookmark, error) {
	rows, err := db.Query("SELECT id, title, style, min_zoom, max_zoom, north, south, east, west, center_lat, center_lng FROM bookmarks ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		err := rows.Scan(&b.ID, &b.Title, &b.Style, &b.MinZoom, &b.MaxZoom, &b.North, &b.South, &b.East, &b.West, &b.CenterLat, &b.CenterLng)
		if err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, nil
}

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetMapKey returns the map API key loaded from config.json
func (a *App) GetMapKey() string {
	return appConfig.MapKey
}

func long2tile(lon float64, zoom int) int {
	return int(math.Floor(((lon + 180.0) / 360.0) * math.Pow(2.0, float64(zoom))))
}

func lat2tile(lat float64, zoom int) int {
	return int(math.Floor((1.0 - math.Log(math.Tan(lat*math.Pi/180.0)+1.0/math.Cos(lat*math.Pi/180.0))/math.Pi) / 2.0 * math.Pow(2.0, float64(zoom))))
}

// --- MAIN DOWNLOAD FUNCTION ---
// Called from Svelte: DownloadRegion("Title", 12, 14, -7.0, -7.5, 110.5, 110.0)
func (a *App) DownloadRegion(mode string, title string, minZ, maxZ int, north, south, east, west float64) Bookmark {

	// Limit concurrent downloads to avoid DB lock/race
	const maxConcurrent = 4

	// Save bookmark for this download (center coordinate)
	centerLat := (north + south) / 2.0
	centerLng := (east + west) / 2.0
	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = fmt.Sprintf("%s download %d-%d", mode, minZ, maxZ)
	}
	res, _ := db.Exec("INSERT INTO bookmarks (title, style, min_zoom, max_zoom, north, south, east, west, center_lat, center_lng) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", cleanTitle, mode, minZ, maxZ, north, south, east, west, centerLat, centerLng)
	bookmarkID, _ := res.LastInsertId()
	bookmark := Bookmark{
		ID:        int(bookmarkID),
		Title:     cleanTitle,
		Style:     mode,
		MinZoom:   minZ,
		MaxZoom:   maxZ,
		North:     north,
		South:     south,
		East:      east,
		West:      west,
		CenterLat: centerLat,
		CenterLng: centerLng,
	}

	go func(b Bookmark, style string, minZoom, maxZoom int, n, s, e, w float64) {
		tilesChan := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup
		var errMu sync.Mutex
		var hadError bool
		var firstErr string
		recordError := func(msg string) {
			errMu.Lock()
			defer errMu.Unlock()
			if !hadError {
				hadError = true
				firstErr = msg
			}
		}

		apiKey := appConfig.MapKey
		baseUrl := ""
		if style == "hybrid" {
			baseUrl = "https://api.maptiler.com/maps/hybrid/%d/%d/%d.jpg?key=" + apiKey
		} else {
			baseUrl = "https://api.maptiler.com/maps/openstreetmap/%d/%d/%d.jpg?key=" + apiKey
		}
		for z := minZoom; z <= maxZoom; z++ {
			left := long2tile(w, z)
			right := long2tile(e, z)
			top := lat2tile(n, z)
			bottom := lat2tile(s, z)
			for x := left; x <= right; x++ {
				for y := top; y <= bottom; y++ {
					tilesChan <- struct{}{}
					wg.Add(1)
					go func(style string, zoomLevel, tileX, tileY int, urlTemplate string) {
						defer func() {
							<-tilesChan
							wg.Done()
						}()
						var exists int
						err := db.QueryRow("SELECT 1 FROM tiles WHERE style=? AND zoom_level=? AND tile_column=? AND tile_row=?", style, zoomLevel, tileX, tileY).Scan(&exists)
						if err == nil && exists == 1 {
							return
						}
						url := fmt.Sprintf(urlTemplate, zoomLevel, tileX, tileY)
						resp, err := http.Get(url)
						if err != nil {
							recordError(fmt.Sprintf("network error: %v", err))
							return
						}
						data, err := io.ReadAll(resp.Body)
						resp.Body.Close()
						if err != nil {
							recordError(fmt.Sprintf("read error: %v", err))
							return
						}
						if len(data) > 0 {
							_, err = db.Exec("INSERT INTO tiles (style, zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?, ?)", style, zoomLevel, tileX, tileY, data)
							if err != nil {
								recordError(fmt.Sprintf("save error: %v", err))
								return
							}
							fmt.Printf("Saved %s: %d/%d/%d\n", style, zoomLevel, tileX, tileY)
						}
					}(style, z, x, y, baseUrl)
				}
			}
		}
		wg.Wait()
		status := "complete"
		message := fmt.Sprintf("%s ready", b.Title)
		if hadError {
			status = "error"
			if firstErr != "" {
				message = firstErr
			} else {
				message = "Some tiles failed to download"
			}
		}
		runtime.EventsEmit(a.ctx, "download-status", map[string]any{
			"bookmarkId": b.ID,
			"title":      b.Title,
			"status":     status,
			"message":    message,
		})
		fmt.Println("Download Complete!")
	}(bookmark, mode, minZ, maxZ, north, south, east, west)
	return bookmark
}

// ClearDownloads removes all cached tiles and bookmarks from the database.
func (a *App) ClearDownloads() error {
	if _, err := db.Exec("DELETE FROM tiles"); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM bookmarks"); err != nil {
		return err
	}
	_, err := db.Exec("VACUUM")
	return err
}
