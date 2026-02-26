package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	_ "modernc.org/sqlite"
)

//go:embed all:frontend/dist
var assets embed.FS

var db *sql.DB

// dbReady is closed once initDB() finishes — callers block on it before using db.
var dbReady = make(chan struct{})

// GPSLocation holds a geographic coordinate
type GPSLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// UTMLocation holds a UTM coordinate
type UTMLocation struct {
	Zone     string `json:"zone"`
	Easting  string `json:"easting"`
	Northing string `json:"northing"`
	Co       string `json:"co"`
}

// AppConfig holds configurable values saved in config.json
type AppConfig struct {
	MapKey        string      `json:"map_key"`
	CompassOffset float64     `json:"compass_offset"`
	GPSLocation   GPSLocation `json:"gps_location"`
	UTMLocation   UTMLocation `json:"utm_location"`
}

var appConfig AppConfig

func configPath() string {
	ex, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(ex), "config.json")
}

func loadConfig() {
	data, err := os.ReadFile(configPath())
	if err != nil {
		log.Println("config.json not found, using empty config:", err)
		return
	}
	if err := json.Unmarshal(data, &appConfig); err != nil {
		log.Println("Failed to parse config.json:", err)
	}
}

func saveConfig() error {
	data, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0644)
}

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
	db, err = sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-32000)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(268435456)")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	// SQLite is single-writer; cap the pool to 1 open connection to avoid
	// contention overhead and keep idle connections alive to amortize re-open cost.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

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
		title TEXT DEFAULT '',
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

	if _, err := db.Exec("ALTER TABLE bookmarks ADD COLUMN title TEXT DEFAULT ''"); err != nil {
		lower := strings.ToLower(err.Error())
		if !strings.Contains(lower, "duplicate column name") {
			log.Fatal("Failed to add title column:", err)
		}
	}
	// Signal that the DB is fully initialised.
	close(dbReady)
}

func main() {
	// Only loadConfig runs before the window opens — it is fast (one file read).
	loadConfig()

	// initDB opens and migrates the SQLite file concurrently with wails.Run so
	// the window appears immediately instead of waiting for DB setup.
	go initDB()

	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "DF",
		Width:  820,
		Height: 720,
		Windows: &windows.Options{
			// Persist the WebView2 profile so the browser engine is fully cached
			// between launches — this is what makes 'wails dev' feel faster.
			WebviewUserDataPath: filepath.Join(os.Getenv("APPDATA"), "DF"),
		},
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
	<-dbReady // ensure DB is closed cleanly on exit
	db.Close()
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

	// Wait for DB — should already be ready by the time tiles are requested,
	// but guard against very fast requests right after startup.
	<-dbReady

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
