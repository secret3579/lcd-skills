package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	DeviceID string
	Token    string
	HTTPPort string
	UDPPort  string
	Model    string
}

type LCDPayload struct {
	Text  string `json:"text"`
	Size  int    `json:"size,omitempty"`
	Color string `json:"color,omitempty"`
	Title string `json:"title,omitempty"`
}

var (
	startTime = time.Now()
	cfg       Config
)

func main() {
	cfg = Config{
		DeviceID: envOrDefault("DEVICE_ID", "lcd-a3f9c1"),
		Token:    envOrDefault("TOKEN", generateToken()),
		HTTPPort: envOrDefault("HTTP_PORT", "3000"),
		UDPPort:  envOrDefault("UDP_PORT", "49152"),
		Model:    "esp32-st7789-1.14",
	}

	fmt.Println("=== LCD Mock Device ===")
	fmt.Printf("Device ID : %s\n", cfg.DeviceID)
	fmt.Printf("Token     : %s\n", cfg.Token)
	fmt.Printf("HTTP      : http://localhost:%s\n", cfg.HTTPPort)
	fmt.Printf("UDP       : :%s\n", cfg.UDPPort)
	fmt.Println("=======================")
	fmt.Println()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		startHTTP()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		startUDP()
	}()

	wg.Wait()
}

func startHTTP() {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", handleStatus)
	mux.HandleFunc("/lcd", handleLCD)

	log.Printf("[HTTP] Listening on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, mux); err != nil {
		log.Fatalf("[HTTP] Failed to start: %v", err)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[HTTP] GET /status from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"device_id":       cfg.DeviceID,
		"model":           cfg.Model,
		"uptime_seconds":  int(time.Since(startTime).Seconds()),
		"free_heap_bytes": 138240,
		"wifi_rssi":       -42,
	})
}

func handleLCD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[HTTP] POST /lcd from %s", r.RemoteAddr)

	// TODO(v2): re-enable auth checks
	// deviceID := r.Header.Get("X-Device-ID")
	// token := r.Header.Get("X-Token")

	var payload LCDPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[HTTP] 400 — bad JSON: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}

	if payload.Text == "" {
		log.Printf("[HTTP] 400 — missing text field")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing field: text"})
		return
	}

	if len(payload.Text) > 500 {
		log.Printf("[HTTP] 413 — text too long: %d chars", len(payload.Text))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(map[string]any{"error": "text too long", "max": 500})
		return
	}

	renderLCD(payload)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func renderLCD(p LCDPayload) {
	color := p.Color
	if color == "" {
		color = "blue"
	}
	title := p.Title
	if title == "" {
		title = "Claude"
	}
	size := p.Size
	if size == 0 {
		size = 2
	}

	width := 40
	border := strings.Repeat("═", width)

	fmt.Println()
	fmt.Printf("  ╔%s╗\n", border)
	fmt.Printf("  ║ [%s] %-*s ║\n", strings.ToUpper(color), width-len(color)-5, title)
	fmt.Printf("  ╠%s╣\n", border)

	words := strings.Fields(p.Text)
	line := ""
	for _, w := range words {
		if len(line)+len(w)+1 > width-4 {
			fmt.Printf("  ║  %-*s  ║\n", width-4, line)
			line = w
		} else if line == "" {
			line = w
		} else {
			line += " " + w
		}
	}
	if line != "" {
		fmt.Printf("  ║  %-*s  ║\n", width-4, line)
	}

	fmt.Printf("  ╚%s╝\n", border)
	fmt.Printf("  size=%d\n\n", size)
}

func startUDP() {
	addr, err := net.ResolveUDPAddr("udp", ":"+cfg.UDPPort)
	if err != nil {
		log.Fatalf("[UDP] Failed to resolve addr: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("[UDP] Failed to listen: %v", err)
	}
	defer conn.Close()

	log.Printf("[UDP] Listening on :%s", cfg.UDPPort)

	buf := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("[UDP] Read error: %v", err)
			continue
		}

		payload := string(buf[:n])
		log.Printf("[UDP] Received %q from %s", payload, remoteAddr)

		if payload != "AUTONOMOUS_LCD_PROBE?" {
			log.Printf("[UDP] Ignoring unknown probe")
			continue
		}

		reply, _ := json.Marshal(map[string]any{
			"device_id": cfg.DeviceID,
			"http_port": 3000,
			"model":     cfg.Model,
		})

		conn.WriteToUDP(reply, remoteAddr)
		log.Printf("[UDP] Replied to %s", remoteAddr)
	}
}

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
