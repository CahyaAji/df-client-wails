package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
)

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
func (a *App) DownloadRegion(minZ, maxZ int, north, south, east, west float64) string {

	// Run in a separate thread so UI doesn't freeze
	go func() {
		// YOUR API KEY HERE
		apiKey := "fB2eDjoDg2nlel5Kw6ym"

		// We will download both "normal" and "hybrid" for the area,
		// or you can choose just one. Let's do Hybrid (Satellite) as it's most important.
		styles := []string{"hybrid", "normal"}

		for _, style := range styles {

			// Determine URL template
			baseUrl := ""
			if style == "hybrid" {
				baseUrl = "https://api.maptiler.com/maps/hybrid/%d/%d/%d.jpg?key=" + apiKey
			} else {
				baseUrl = "https://api.maptiler.com/maps/openstreetmap/%d/%d/%d.jpg?key=" + apiKey
			}

			// Loop Z (Zoom Levels)
			for z := minZ; z <= maxZ; z++ {
				left := long2tile(west, z)
				right := long2tile(east, z)
				top := lat2tile(north, z)
				bottom := lat2tile(south, z)

				// Loop X and Y
				for x := left; x <= right; x++ {
					for y := top; y <= bottom; y++ {

						// Check if we already have it?
						var exists int
						_ = db.QueryRow("SELECT 1 FROM tiles WHERE style=? AND zoom_level=? AND tile_column=? AND tile_row=?", style, z, x, y).Scan(&exists)
						if exists == 1 {
							continue // Skip if already downloaded
						}

						// Download
						url := fmt.Sprintf(baseUrl, z, x, y)
						resp, err := http.Get(url)
						if err != nil {
							fmt.Println("Network error:", url)
							continue
						}

						data, _ := io.ReadAll(resp.Body)
						resp.Body.Close()

						// Save to DB
						if len(data) > 0 {
							_, err = db.Exec("INSERT INTO tiles (style, zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?, ?)", style, z, x, y, data)
							if err != nil {
								fmt.Println("Save error:", err)
							} else {
								// Optional: Print progress
								fmt.Printf("Saved %s: %d/%d/%d\n", style, z, x, y)
							}
						}
					}
				}
			}
		}
		fmt.Println("Download Complete!")
	}()

	return "Download Started in Background..."
}
