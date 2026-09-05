package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	ProviderType     string `json:"provider_type"`
	ServerGroupID    string `json:"p2p_server_group_id"`
	PublicIP         string `json:"public_ip"`
	WakeupUDPPort    int    `json:"wakeup_udp_port"`
	PlainTCPPort     int    `json:"plain_tcp_port"`
	DSLKTCPPort      int    `json:"dslk_tcp_port"`
	HealthHTTPAddr   string `json:"health_http_addr"`
	AllowedP2IDPrefix string `json:"allowed_p2pid_prefix"`
}

type Registry struct {
	mu      sync.RWMutex
	devices map[string]DeviceSession
}

type DeviceSession struct {
	P2PID       string    `json:"p2pid"`
	GroupID     string    `json:"group_id"`
	RemoteAddr  string    `json:"remote_addr"`
	LastSeenUTC time.Time `json:"last_seen_utc"`
}

type Request struct {
	Command string `json:"cmd"`
	GroupID string `json:"group_id,omitempty"`
	P2PID   string `json:"p2pid,omitempty"`
	Role    string `json:"role,omitempty"`
}

type Response struct {
	OK          bool           `json:"ok"`
	Error       string         `json:"error,omitempty"`
	Provider    string         `json:"provider_type,omitempty"`
	GroupID     string         `json:"group_id,omitempty"`
	P2PID       string         `json:"p2pid,omitempty"`
	Session     *DeviceSession `json:"session,omitempty"`
	ServerTime  time.Time      `json:"server_time_utc"`
	Compatibility string       `json:"compatibility"`
}

func main() {
	configPath := flag.String("config", "", "path to JSON config")
	flag.Parse()
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	reg := &Registry{devices: make(map[string]DeviceSession)}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 4)
	go func() { errCh <- serveUDP(ctx, cfg, reg, cfg.WakeupUDPPort) }()
	go func() { errCh <- serveTCP(ctx, cfg, reg, cfg.PlainTCPPort, "plain") }()
	go func() { errCh <- serveTCP(ctx, cfg, reg, cfg.DSLKTCPPort, "dslk") }()
	go func() { errCh <- serveHTTP(ctx, cfg, reg) }()

	log.Printf("lenshub compat skeleton started provider=%s group=%s udp=%d tcp=%d dslk=%d health=%s", cfg.ProviderType, cfg.ServerGroupID, cfg.WakeupUDPPort, cfg.PlainTCPPort, cfg.DSLKTCPPort, cfg.HealthHTTPAddr)
	select {
	case <-ctx.Done():
		log.Printf("shutdown requested")
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("server failed: %v", err)
		}
	}
}

func loadConfig(path string) (Config, error) {
	cfg := Config{
		ProviderType: "p2px_ppcs",
		ServerGroupID: "gongshi-test-group-01",
		PublicIP: "47.76.137.198",
		WakeupUDPPort: 12305,
		PlainTCPPort: 12306,
		DSLKTCPPort: 12308,
		HealthHTTPAddr: "127.0.0.1:18080",
		AllowedP2IDPrefix: "PPCS-GSTEST-20260905-",
	}
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, validateConfig(cfg)
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.ProviderType) == "" || strings.TrimSpace(cfg.ServerGroupID) == "" {
		return fmt.Errorf("provider_type and p2p_server_group_id are required")
	}
	for name, port := range map[string]int{"wakeup_udp_port": cfg.WakeupUDPPort, "plain_tcp_port": cfg.PlainTCPPort, "dslk_tcp_port": cfg.DSLKTCPPort} {
		if port <= 0 || port > 65535 {
			return fmt.Errorf("%s out of range: %d", name, port)
		}
	}
	if cfg.HealthHTTPAddr == "" {
		return fmt.Errorf("health_http_addr is required")
	}
	return nil
}

func serveUDP(ctx context.Context, cfg Config, reg *Registry, port int) error {
	addr := net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, 4096)
	go func() { <-ctx.Done(); _ = conn.Close() }()
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil { return ctx.Err() }
			return err
		}
		resp := handlePayload(cfg, reg, remote.String(), bytesToString(buf[:n]))
		out, _ := json.Marshal(resp)
		_, _ = conn.WriteToUDP(append(out, '\n'), remote)
	}
}

func serveTCP(ctx context.Context, cfg Config, reg *Registry, port int, mode string) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil { return err }
	defer ln.Close()
	go func() { <-ctx.Done(); _ = ln.Close() }()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil { return ctx.Err() }
			return err
		}
		go handleTCPConn(cfg, reg, conn, mode)
	}
}

func handleTCPConn(cfg Config, reg *Registry, conn net.Conn, mode string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	s := bufio.NewScanner(conn)
	for s.Scan() {
		resp := handlePayload(cfg, reg, conn.RemoteAddr().String(), s.Text())
		out, _ := json.Marshal(resp)
		_, _ = conn.Write(append(out, '\n'))
	}
}

func serveHTTP(ctx context.Context, cfg Config, reg *Registry) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "provider_type": cfg.ProviderType, "group_id": cfg.ServerGroupID, "server_time_utc": time.Now().UTC()})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		reg.mu.RLock(); count := len(reg.devices); reg.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "registered_devices": count, "compatibility": "skeleton_not_ppcs_wire_compatible"})
	})
	srv := &http.Server{Addr: cfg.HealthHTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) { return context.Canceled }
	return err
}

func handlePayload(cfg Config, reg *Registry, remote string, payload string) Response {
	payload = strings.TrimSpace(payload)
	if payload == "" || strings.EqualFold(payload, "ping") {
		return baseResponse(cfg, true, "")
	}
	var req Request
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return baseResponse(cfg, false, "invalid_json")
	}
	if req.GroupID != "" && req.GroupID != cfg.ServerGroupID {
		return baseResponse(cfg, false, "group_mismatch")
	}
	if req.P2PID != "" && cfg.AllowedP2IDPrefix != "" && !strings.HasPrefix(req.P2PID, cfg.AllowedP2IDPrefix) {
		return baseResponse(cfg, false, "p2pid_prefix_denied")
	}
	switch strings.ToLower(req.Command) {
	case "ping":
		return baseResponse(cfg, true, "")
	case "register", "heartbeat":
		if req.P2PID == "" { return baseResponse(cfg, false, "p2pid_required") }
		sess := DeviceSession{P2PID: req.P2PID, GroupID: cfg.ServerGroupID, RemoteAddr: remote, LastSeenUTC: time.Now().UTC()}
		reg.mu.Lock(); reg.devices[req.P2PID] = sess; reg.mu.Unlock()
		resp := baseResponse(cfg, true, ""); resp.P2PID = req.P2PID; resp.Session = &sess; return resp
	case "lookup":
		if req.P2PID == "" { return baseResponse(cfg, false, "p2pid_required") }
		reg.mu.RLock(); sess, ok := reg.devices[req.P2PID]; reg.mu.RUnlock()
		if !ok { return baseResponse(cfg, false, "device_offline") }
		resp := baseResponse(cfg, true, ""); resp.P2PID = req.P2PID; resp.Session = &sess; return resp
	default:
		return baseResponse(cfg, false, "unknown_cmd")
	}
}

func baseResponse(cfg Config, ok bool, errText string) Response {
	return Response{OK: ok, Error: errText, Provider: cfg.ProviderType, GroupID: cfg.ServerGroupID, ServerTime: time.Now().UTC(), Compatibility: "skeleton_not_ppcs_wire_compatible"}
}

func bytesToString(b []byte) string { return string(b) }
