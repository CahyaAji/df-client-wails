package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/protomaps/go-pmtiles/pmtiles"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	_ "modernc.org/sqlite"
)

//go:embed all:frontend/dist
var assets embed.FS

// Font glyph PBFs for offline label rendering (road/place names), served at
// /fonts/{fontstack}/{range}.pbf. Sourced from the openmaptiles/fonts
// gh-pages build (Open Sans Regular, Apache-2.0 licensed — see
// assets/fonts/LICENSE.txt) so text labels work fully offline instead of
// depending on a public glyph CDN.
//
//go:embed assets/fonts
var fontAssets embed.FS

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
	VectorMapPath string      `json:"vector_map_path"`
	CompassOffset float64     `json:"compass_offset"`
	OffsetUhf     float64     `json:"offsetUhf"`
	OffsetVhf     float64     `json:"offsetVhf"`
	GPSLocation   GPSLocation `json:"gps_location"`
	UTMLocation   UTMLocation `json:"utm_location"`
}

var appConfig AppConfig

// Vector (PMTiles) tile server state. Only one archive is active at a time;
// selecting a new file swaps pmServer/pmArchive under pmMu. The pmtiles.Server
// spins up a background cache goroutine via Start() that never voluntarily
// exits — when swapping files the old goroutine is simply abandoned (it just
// blocks forever on an unused channel), which is an acceptable trade-off for
// a desktop app where the user rarely swaps files within a single session.
var (
	pmMu      sync.RWMutex
	pmServer  *pmtiles.Server
	pmArchive string
)

// loadVectorMap opens the given .pmtiles file and makes it the active vector
// tile source, replacing any previously loaded archive.
func loadVectorMap(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("pmtiles file not accessible: %w", err)
	}

	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	bucket := pmtiles.NewFileBucket(dir)
	logger := log.New(os.Stdout, "[pmtiles] ", log.LstdFlags)
	srv, err := pmtiles.NewServerWithBucket(bucket, "", logger, 64, "")
	if err != nil {
		return err
	}
	srv.Start()

	pmMu.Lock()
	pmServer = srv
	pmArchive = base
	pmMu.Unlock()
	log.Printf("[pmtiles] loaded vector map %q (archive=%q, dir=%q)", path, base, dir)
	return nil
}

func configPath() string {
	// If wails.json exists in the current working directory, we are likely in development mode
	if _, err := os.Stat("wails.json"); err == nil {
		return "config.json"
	}

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

	// Restore the previously selected vector (PMTiles) map, if any, without
	// blocking window startup.
	if appConfig.VectorMapPath != "" {
		go func() {
			if err := loadVectorMap(appConfig.VectorMapPath); err != nil {
				log.Println("Failed to reload previously selected PMTiles file:", err)
			}
		}()
	}

	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Cakranusa DF",
		MinWidth:  314,
		MinHeight: 500,
		Width:     320,
		Height:    700,
		// Force a normal window on startup instead of inheriting a maximized state.
		WindowStartState: options.Normal,
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
		Windows: &windows.Options{
			// Persist the WebView2 profile so the browser engine is fully cached
			// between launches — this is what makes 'wails dev' feel faster.
			WebviewUserDataPath: filepath.Join(os.Getenv("APPDATA"), "DF"),
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// --- CORS headers for development ---
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					if r.Method == "OPTIONS" {
						w.WriteHeader(http.StatusOK)
						return
					}

					// Check if the request starts with "/tiles/"
					// Format expected: /tiles/{style}/{z}/{x}/{y}.png
					if strings.HasPrefix(r.URL.Path, "/tiles/") {
						handleTileRequest(w, r)
						return
					}

					// Vector (PMTiles) OSM base map.
					// Format expected: /vtiles/{z}/{x}/{y}.mvt
					if strings.HasPrefix(r.URL.Path, "/vtiles/") {
						handleVectorTileRequest(w, r)
						return
					}

					// Font glyphs for offline vector map labels.
					// Format expected: /fonts/{fontstack}/{range}.pbf
					if strings.HasPrefix(r.URL.Path, "/fonts/") {
						handleFontRequest(w, r)
						return
					}

					// Otherwise, just serve the Svelte app
					next.ServeHTTP(w, r)
				})
			},
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
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

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.Write(tileData)
}

// handleVectorTileRequest serves vector (MVT/PBF) tiles from the currently
// active PMTiles archive, selected via App.SelectVectorMapFile.
// Format expected: /vtiles/{z}/{x}/{y}.mvt
func handleVectorTileRequest(w http.ResponseWriter, r *http.Request) {
	pmMu.RLock()
	srv := pmServer
	archive := pmArchive
	pmMu.RUnlock()

	if srv == nil {
		log.Printf("[vtiles] %s -> no vector map loaded", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// r.URL.Path = /vtiles/{z}/{x}/{y}.mvt -> archive-relative path
	rest := strings.TrimPrefix(r.URL.Path, "/vtiles/")
	path := "/" + archive + "/" + rest

	status, headers, data := srv.Get(r.Context(), path)

	// The pmtiles library reports the tile's on-disk compression via the
	// Content-Encoding header, relying on the CLIENT to transparently
	// decompress it (as a normal browser fetch would). Wails' webview
	// serves this content through its own custom "wails://" URL scheme
	// handler rather than a plain HTTP load, and that path does NOT
	// perform automatic Content-Encoding decompression — MapLibre would
	// otherwise receive raw gzip bytes and fail with "Unable to parse the
	// tile ... expected a valid PBF". So we decompress it ourselves here
	// and drop the header, sending plain, already-decoded PBF bytes.
	if enc := headers["Content-Encoding"]; enc == "gzip" && status == http.StatusOK {
		if gr, err := gzip.NewReader(bytes.NewReader(data)); err == nil {
			if decoded, err := io.ReadAll(gr); err == nil {
				data = decoded
			} else {
				log.Printf("[vtiles] gzip read error for %s: %v", r.URL.Path, err)
			}
			gr.Close()
		} else {
			log.Printf("[vtiles] gzip reader error for %s: %v", r.URL.Path, err)
		}
		delete(headers, "Content-Encoding")
	}

	for k, v := range headers {
		w.Header().Set(k, v)
	}
	// Only cache successful tile responses. Caching error/404 responses is
	// dangerous here: a tile that legitimately 404s before a file is loaded
	// (or before it's fully re-indexed after a swap) would otherwise be
	// remembered as "missing" by the webview's HTTP cache forever, even
	// after the correct archive starts serving that tile successfully.
	if status == http.StatusOK {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	} else {
		w.Header().Set("Cache-Control", "no-store")
		log.Printf("[vtiles] %s -> archive=%s path=%s status=%d", r.URL.Path, archive, path, status)
	}
	w.WriteHeader(status)
	w.Write(data)
}

// handleFontRequest serves embedded font glyph PBFs for offline label
// rendering. Format expected: /fonts/{fontstack}/{range}.pbf
// (fontstack may itself contain spaces, e.g. "Open Sans Regular", but
// never contains "/", so the *last* slash always separates it from the
// "{start}-{end}.pbf" range filename.)
func handleFontRequest(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/fonts/")
	idx := strings.LastIndex(rest, "/")
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	fontstack := rest[:idx]
	rangeFile := rest[idx+1:]

	data, err := fontAssets.ReadFile("assets/fonts/" + fontstack + "/" + rangeFile)
	if err != nil {
		log.Printf("[fonts] %s -> not found (fontstack=%q range=%q): %v", r.URL.Path, fontstack, rangeFile, err)
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Write(data)
}
