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
	"syscall"
	"time"

	"github.com/frankichen/ZLMediaKit/lenshub/compatserver/pppp"
)

const compatibilityLevel = "pppp_f1_rendezvous_foundation_not_vendor_validated"

type Config struct {
	ProviderType                         string   `json:"provider_type"`
	ServerGroupID                        string   `json:"p2p_server_group_id"`
	PublicIP                             string   `json:"public_ip"`
	WakeupUDPPort                        int      `json:"wakeup_udp_port"`
	PlainTCPPort                         int      `json:"plain_tcp_port"`
	DSLKTCPPort                          int      `json:"dslk_tcp_port"`
	DiagnosticTCPAddr                    string   `json:"diagnostic_tcp_addr"`
	HealthHTTPAddr                       string   `json:"health_http_addr"`
	AllowedDIDPrefixes                   []string `json:"allowed_did_prefixes"`
	PresenceTTLSeconds                   int      `json:"presence_ttl_seconds"`
	PPPPPSKEnv                           string   `json:"pppp_psk_env,omitempty"`
	UnsafeAllowUnverifiedDIDLoginForTest bool     `json:"unsafe_allow_unverified_did_login_for_test"`
}

type DiagnosticRequest struct {
	Command string `json:"cmd"`
	DID     string `json:"did,omitempty"`
}

type DiagnosticResponse struct {
	OK            bool            `json:"ok"`
	Error         string          `json:"error,omitempty"`
	Provider      string          `json:"provider_type"`
	GroupID       string          `json:"group_id"`
	DID           string          `json:"did,omitempty"`
	Session       *DevicePresence `json:"session,omitempty"`
	Stats         *WireStats      `json:"stats,omitempty"`
	ServerTimeUTC time.Time       `json:"server_time_utc"`
	Compatibility string          `json:"compatibility"`
}

func defaultConfig() Config {
	return Config{
		ProviderType:       "p2px_ppcs",
		ServerGroupID:      "gongshi-test-group-01",
		PublicIP:           "47.76.137.198",
		WakeupUDPPort:      12305,
		PlainTCPPort:       12306,
		DSLKTCPPort:        12308,
		DiagnosticTCPAddr:  "127.0.0.1:18181",
		HealthHTTPAddr:     "127.0.0.1:18180",
		AllowedDIDPrefixes: []string{"PPCS"},
		PresenceTTLSeconds: 90,
	}
}

func (c Config) PresenceTTL() time.Duration {
	return time.Duration(c.PresenceTTLSeconds) * time.Second
}

func main() {
	configPath := flag.String("config", "", "path to JSON config")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	reg := newRegistry()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 3)
	go func() { errCh <- servePPPPUDP(ctx, cfg, reg) }()
	go func() { errCh <- serveDiagnosticTCP(ctx, cfg, reg) }()
	go func() { errCh <- serveHTTP(ctx, cfg, reg) }()

	log.Printf("LensHub PPPP compatibility runtime started provider=%s group=%s pppp_udp=%d reserved_tcp=%d reserved_dslk=%d diag=%s health=%s compatibility=%s", cfg.ProviderType, cfg.ServerGroupID, cfg.WakeupUDPPort, cfg.PlainTCPPort, cfg.DSLKTCPPort, cfg.DiagnosticTCPAddr, cfg.HealthHTTPAddr, compatibilityLevel)
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
	cfg := defaultConfig()
	if path == "" {
		return cfg, validateConfig(cfg)
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
	if err := validateLoopbackTCPAddr(cfg.DiagnosticTCPAddr); err != nil {
		return fmt.Errorf("diagnostic_tcp_addr: %w", err)
	}
	if cfg.PresenceTTLSeconds < 5 || cfg.PresenceTTLSeconds > 3600 {
		return fmt.Errorf("presence_ttl_seconds must be between 5 and 3600")
	}
	for _, prefix := range cfg.AllowedDIDPrefixes {
		if len(strings.TrimSpace(prefix)) != 4 {
			return fmt.Errorf("allowed_did_prefixes entry must be four bytes: %q", prefix)
		}
	}
	return nil
}

func servePPPPUDP(ctx context.Context, cfg Config, reg *Registry) error {
	addr := net.UDPAddr{IP: net.IPv4zero, Port: cfg.WakeupUDPPort}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() { <-ctx.Done(); _ = conn.Close() }()

	var psk string
	if cfg.PPPPPSKEnv != "" {
		psk = os.Getenv(cfg.PPPPPSKEnv)
		if psk == "" {
			log.Printf("PPPP PSK env %s is not set; only plain F1 packets will be accepted", cfg.PPPPPSKEnv)
		}
	}

	buf := make([]byte, 64*1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		reg.noteReceive()
		decoded, err := decodeWireDatagram(buf[:n], psk)
		if err != nil {
			reg.noteParseError()
			continue
		}
		out := handleWirePacket(cfg, reg, remote, decoded)
		var sent uint64
		for _, datagram := range out {
			wire, err := datagram.packet.MarshalBinary()
			if err != nil {
				continue
			}
			if datagram.obfuscate {
				if psk == "" {
					continue
				}
				wire = pppp.Obfuscate(psk, wire)
			}
			if _, err := conn.WriteToUDP(wire, datagram.to); err == nil {
				sent++
			}
		}
		reg.noteSent(sent)
	}
}

// TCP 12306/12308 are reserved for actual CS2 TCP/DSLK framing and are not
// opened until that framing is implemented. Diagnostics bind loopback only.
func serveDiagnosticTCP(ctx context.Context, cfg Config, reg *Registry) error {
	ln, err := net.Listen("tcp", cfg.DiagnosticTCPAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	go func() { <-ctx.Done(); _ = ln.Close() }()
	log.Printf("loopback diagnostic TCP listener addr=%s", cfg.DiagnosticTCPAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go handleDiagnosticConn(cfg, reg, conn)
	}
}

func validateLoopbackTCPAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("must use an explicit loopback IP")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("invalid port")
	}
	return nil
}

func handleDiagnosticConn(cfg Config, reg *Registry, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	s := bufio.NewScanner(conn)
	for s.Scan() {
		resp := handleDiagnosticPayload(cfg, reg, s.Text())
		out, _ := json.Marshal(resp)
		_, _ = conn.Write(append(out, '\n'))
	}
}

func handleDiagnosticPayload(cfg Config, reg *Registry, payload string) DiagnosticResponse {
	base := func(ok bool, errText string) DiagnosticResponse {
		return DiagnosticResponse{OK: ok, Error: errText, Provider: cfg.ProviderType, GroupID: cfg.ServerGroupID, ServerTimeUTC: time.Now().UTC(), Compatibility: compatibilityLevel}
	}
	payload = strings.TrimSpace(payload)
	if payload == "" || strings.EqualFold(payload, "ping") {
		return base(true, "")
	}
	var req DiagnosticRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return base(false, "invalid_json")
	}
	switch strings.ToLower(req.Command) {
	case "ping":
		return base(true, "")
	case "stats":
		_, stats := reg.snapshot()
		resp := base(true, "")
		resp.Stats = &stats
		return resp
	case "lookup":
		did, err := pppp.ParseDID(req.DID)
		if err != nil {
			return base(false, "invalid_did")
		}
		presence, ok := reg.lookup(did)
		if !ok {
			return base(false, "device_offline")
		}
		resp := base(true, "")
		resp.DID = did.String()
		resp.Session = &presence
		return resp
	default:
		return base(false, "unknown_cmd")
	}
}

func serveHTTP(ctx context.Context, cfg Config, reg *Registry) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "provider_type": cfg.ProviderType, "group_id": cfg.ServerGroupID,
			"wire_protocol": "pppp_f1", "compatibility": compatibilityLevel,
			"server_time_utc": time.Now().UTC(),
		})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		count, stats := reg.snapshot()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "registered_devices": count, "wire_protocol": "pppp_f1",
			"direct_rendezvous": true, "pppp_relay": false, "tcp_fallback": false,
			"unverified_login_test_mode": cfg.UnsafeAllowUnverifiedDIDLoginForTest,
			"compatibility":              compatibilityLevel, "stats": stats,
		})
	})
	srv := &http.Server{Addr: cfg.HealthHTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return context.Canceled
	}
	return err
}
