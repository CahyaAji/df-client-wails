package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
)

// Bookmark struct for frontend
type Bookmark struct {
	ID        int     `json:"id"`
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
	rows, err := db.Query("SELECT id, style, min_zoom, max_zoom, north, south, east, west, center_lat, center_lng FROM bookmarks ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		err := rows.Scan(&b.ID, &b.Style, &b.MinZoom, &b.MaxZoom, &b.North, &b.South, &b.East, &b.West, &b.CenterLat, &b.CenterLng)
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

func long2tile(lon float64, zoom int) int {
	return int(math.Floor(lon+180.0) / 360.0 * math.Pow(2.0, float64(zoom)))
}

func lat2tile(lat float64, zoom int) int {
	return int(math.Floor((1.0 - math.Log(math.Tan(lat*math.Pi/180.0)+1.0/math.Cos(lat*math.Pi/180.0))/math.Pi) / 2.0 * math.Pow(2.0, float64(zoom))))
}

// --- MAIN DOWNLOAD FUNCTION ---
// Called from Svelte: DownloadRegion(12, 14, -7.0, -7.5, 110.5, 110.0)
func (a *App) DownloadRegion(mode string, minZ, maxZ int, north, south, east, west float64) Bookmark {

	// Limit concurrent downloads to avoid DB lock/race
	const maxConcurrent = 4
	tilesChan := make(chan struct{}, maxConcurrent)

	// Save bookmark for this download (center coordinate)
	centerLat := (north + south) / 2.0
	centerLng := (east + west) / 2.0
	res, _ := db.Exec("INSERT INTO bookmarks (style, min_zoom, max_zoom, north, south, east, west, center_lat, center_lng) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", mode, minZ, maxZ, north, south, east, west, centerLat, centerLng)
	bookmarkID, _ := res.LastInsertId()
	bookmark := Bookmark{
		ID:        int(bookmarkID),
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

	go func() {
		apiKey := "fB2eDjoDg2nlel5Kw6ym"
		style := mode
		baseUrl := ""
		if style == "hybrid" {
			baseUrl = "https://api.maptiler.com/maps/hybrid/%d/%d/%d.jpg?key=" + apiKey
		} else {
			baseUrl = "https://api.maptiler.com/maps/openstreetmap/%d/%d/%d.jpg?key=" + apiKey
		}
		for z := minZ; z <= maxZ; z++ {
			left := long2tile(west, z)
			right := long2tile(east, z)
			top := lat2tile(north, z)
			bottom := lat2tile(south, z)
			for x := left; x <= right; x++ {
				for y := top; y <= bottom; y++ {
					tilesChan <- struct{}{} // acquire
					go func(style string, z, x, y int, baseUrl string) {
						defer func() { <-tilesChan }() // release
						var exists int
						err := db.QueryRow("SELECT 1 FROM tiles WHERE style=? AND zoom_level=? AND tile_column=? AND tile_row=?", style, z, x, y).Scan(&exists)
						if err == nil && exists == 1 {
							return // already downloaded
						}
						url := fmt.Sprintf(baseUrl, z, x, y)
						resp, err := http.Get(url)
						if err != nil {
							fmt.Println("Network error:", url, err)
							return
						}
						data, err := io.ReadAll(resp.Body)
						resp.Body.Close()
						if err != nil {
							fmt.Println("Read error:", url, err)
							return
						}
						if len(data) > 0 {
							_, err = db.Exec("INSERT INTO tiles (style, zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?, ?)", style, z, x, y, data)
							if err != nil {
								fmt.Println("Save error:", err)
							} else {
								fmt.Printf("Saved %s: %d/%d/%d\n", style, z, x, y)
							}
						}
					}(style, z, x, y, baseUrl)
				}
			}
		}
		fmt.Println("Download Complete!")
	}()
	return bookmark
}
