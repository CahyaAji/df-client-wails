package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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
	<-dbReady
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

// UDP listener state
var (
	udpConn      *net.UDPConn
	udpMu        sync.Mutex
	udpListening bool
)

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Keep launch state normal.
	runtime.WindowUnmaximise(ctx)
	// Enforce compact startup size even if the OS/webview profile tries to restore.
	runtime.WindowSetSize(ctx, 320, 710)
	runtime.WindowSetPosition(ctx, 1400, 0)
}

// domReady runs after the webview is ready. On some Windows setups, the
// restored window state can override startup sizing; retry briefly to force normal.
func (a *App) domReady(ctx context.Context) {
	go func() {
		for i := 0; i < 6; i++ {
			time.Sleep(150 * time.Millisecond)
			if runtime.WindowIsMaximised(ctx) {
				runtime.WindowUnmaximise(ctx)
			}
			runtime.WindowSetSize(ctx, 320, 710)
			runtime.WindowSetPosition(ctx, 1440, 0)
		}
	}()
}

// GetMapKey returns the map API key loaded from config.json
func (a *App) GetMapKey() string {
	return appConfig.MapKey
}

// GetConfig returns the full app configuration
func (a *App) GetConfig() AppConfig {
	return appConfig
}

// SetCompassOffset saves compass offset to config.json
func (a *App) SetCompassOffset(value float64) error {
	appConfig.CompassOffset = value
	return saveConfig()
}

// SetGPSLocation saves GPS coordinates to config.json
func (a *App) SetGPSLocation(lat, lng float64) error {
	appConfig.GPSLocation = GPSLocation{Lat: lat, Lng: lng}
	return saveConfig()
}

// SetUTMLocation saves UTM coordinates to config.json
func (a *App) SetUTMLocation(zone, easting, northing, co string) error {
	appConfig.UTMLocation = UTMLocation{Zone: zone, Easting: easting, Northing: northing, Co: co}
	return saveConfig()
}

// ProxyGetRequest performs a GET request to the given URL and returns the response body as a string.
func (a *App) ProxyGetRequest(url string) (string, error) {
	client := http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ProxyPostRequest performs a POST request to the given URL with a JSON body.
func (a *App) ProxyPostRequest(url string, jsonBody string) (string, error) {
	client := http.Client{
		Timeout: 3 * time.Second,
	}
	reqBody := []byte(jsonBody)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ResetConfig resets all user settings to defaults, keeping the map key
func (a *App) ResetConfig() error {
	appConfig.CompassOffset = 0
	appConfig.GPSLocation = GPSLocation{}
	appConfig.UTMLocation = UTMLocation{}
	return saveConfig()
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
	<-dbReady

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
	<-dbReady
	if _, err := db.Exec("DELETE FROM tiles"); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM bookmarks"); err != nil {
		return err
	}
	_, err := db.Exec("VACUUM")
	return err
}

// StartUdpListener starts a UDP listener on the given port and emits received messages as events.
func (a *App) StartUdpListener(port int) string {
	udpMu.Lock()
	defer udpMu.Unlock()

	if udpListening {
		return fmt.Sprintf("Already listening on port %d", port)
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Sprintf("Error resolving address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Sprintf("Error listening: %v", err)
	}

	udpConn = conn
	udpListening = true

	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			raw := strings.TrimSpace(string(buf[:n]))
			fmt.Printf("[UDP] received: %q\n", raw)

			// Try JSON first (device may send structured JSON directly)
			var jsonMsg map[string]any
			if err := json.Unmarshal([]byte(raw), &jsonMsg); err == nil {
				fmt.Printf("[UDP] emitting JSON message: %v\n", jsonMsg)
				runtime.EventsEmit(a.ctx, "udp-message", jsonMsg)
			} else if value, err := strconv.ParseFloat(raw, 64); err == nil {
				// Plain number string
				fmt.Printf("[UDP] emitting number: %v\n", value)
				runtime.EventsEmit(a.ctx, "udp-message", map[string]any{
					"type": "number",
					"data": map[string]any{"value": value},
				})
			} else {
				// Raw string
				fmt.Printf("[UDP] emitting raw: %q\n", raw)
				runtime.EventsEmit(a.ctx, "udp-message", map[string]any{
					"type": "raw",
					"data": map[string]any{"value": raw},
				})
			}
		}
	}()

	return fmt.Sprintf("Listening on port %d", port)
}

// StopUdpListener stops the active UDP listener.
func (a *App) StopUdpListener() string {
	udpMu.Lock()
	defer udpMu.Unlock()

	if !udpListening || udpConn == nil {
		return "Not listening"
	}

	udpConn.Close()
	udpConn = nil
	udpListening = false

	return "Stopped listening"
}

// SendUdpNumber sends a number as a UDP packet to localhost on the given port.
func (a *App) SendUdpNumber(number int, port int) string {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Sprintf("Error resolving address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Sprintf("Error connecting: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(fmt.Sprintf("%d", number)))
	if err != nil {
		return fmt.Sprintf("Error sending: %v", err)
	}

	return fmt.Sprintf("Sent %d", number)
}
