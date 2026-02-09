package main

import (
	"database/sql"
	"embed"
	"log"
	"net/http"
	"os"            // Added
	"path/filepath" // Added
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	_ "modernc.org/sqlite"
)

//go:embed all:frontend/dist
var assets embed.FS

var db *sql.DB

func initDB() {
	// 1. Get the path to the current executable
	ex, err := os.Executable()
	if err != nil {
		log.Fatal("Could not get executable path:", err)
	}

	// 2. Resolve the directory of the executable
	// Inside a .app, this will be YourApp.app/Contents/MacOS/
	exPath := filepath.Dir(ex)
	dbPath := filepath.Join(exPath, "map_data.mbtiles")

	// 3. Open the database using the absolute path
	// We use the "file:" prefix for modernc.org/sqlite to ensure path handling is robust
	db, err = sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	query := `
    CREATE TABLE IF NOT EXISTS tiles (
        style TEXT, 
        zoom_level INTEGER, 
        tile_column INTEGER, 
        tile_row INTEGER, 
        tile_data BLOB,
        PRIMARY KEY (style, zoom_level, tile_column, tile_row)
    );
    CREATE TABLE IF NOT EXISTS bookmarks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        style TEXT,
        min_zoom INTEGER,
        max_zoom INTEGER,
        north REAL,
        south REAL,
        east REAL,
        west REAL,
        center_lat REAL,
        center_lng REAL
    );`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
}

func main() {
	initDB()
	defer db.Close()

	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "DF Client",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Check if the request starts with "/tiles/"
					// Format expected: /tiles/{style}/{z}/{x}/{y}.png
					if strings.HasPrefix(r.URL.Path, "/tiles/") {
						handleTileRequest(w, r)
						return
					}

					// Otherwise, just serve the Svelte app
					next.ServeHTTP(w, r)
				})
			},
		},
		OnStartup: app.startup,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// Logic to read from SQLite and send image back to Svelte
func handleTileRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Parse URL: /tiles/normal/14/100/200.png
	parts := strings.Split(r.URL.Path, "/")
	// parts[0]="" parts[1]="tiles" parts[2]="normal" parts[3]="14" parts[4]="100" parts[5]="200.png"

	if len(parts) < 6 {
		http.NotFound(w, r)
		return
	}

	style := parts[2]
	z := parts[3]
	x := parts[4]
	yFilename := parts[5]
	y := strings.TrimSuffix(yFilename, ".png") // Remove .png

	// 2. Query DB
	var tileData []byte
	// Note: MapLibre uses XYZ. Some mbtiles use TMS (flipped Y).
	// For this simple custom server, we save as XYZ and read as XYZ. No flipping needed.
	err := db.QueryRow("SELECT tile_data FROM tiles WHERE style=? AND zoom_level=? AND tile_column=? AND tile_row=?", style, z, x, y).Scan(&tileData)

	if err != nil {
		// If not found in DB, return 404 (MapLibre will handle this gracefully)
		http.NotFound(w, r)
		return
	}

	// 3. Serve Image
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.Write(tileData)
}
