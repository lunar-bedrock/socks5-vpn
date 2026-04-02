package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/txthinking/socks5"
)

type config struct {
	ListenAddr string
	BindIP     string
	Username   string
	Password   string
	TCPTimeout int
	UDPTimeout int
	HealthAddr string
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	bindIP, err := resolveBindIP(cfg.ListenAddr, cfg.BindIP)
	if err != nil {
		log.Fatalf("resolve SOCKS5 bind IP: %v", err)
	}

	server, err := socks5.NewClassicServer(cfg.ListenAddr, bindIP, cfg.Username, cfg.Password, cfg.TCPTimeout, cfg.UDPTimeout)
	if err != nil {
		log.Fatalf("create SOCKS5 server: %v", err)
	}

	healthSrv := startHealthServer(cfg.HealthAddr)
	defer shutdownHTTPServer(healthSrv, "health")

	errCh := make(chan error, 1)
	go func() {
		log.Printf("starting SOCKS5 VPN gateway listen_addr=%s bind_ip=%s tcp_timeout=%ds udp_timeout=%ds auth_enabled=%t",
			cfg.ListenAddr,
			bindIP,
			cfg.TCPTimeout,
			cfg.UDPTimeout,
			cfg.Username != "" || cfg.Password != "",
		)
		errCh <- server.ListenAndServe(nil)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("SOCKS5 server exited: %v", err)
		}
	case sig := <-sigCh:
		log.Printf("received signal=%s, shutting down", sig)
		if err := server.Shutdown(); err != nil {
			log.Printf("SOCKS5 shutdown error: %v", err)
		}
	}
}

func loadConfig() (config, error) {
	cfg := config{
		ListenAddr: firstNonEmpty(strings.TrimSpace(os.Getenv("SOCKS5_LISTEN_ADDR")), ":1080"),
		BindIP:     strings.TrimSpace(os.Getenv("SOCKS5_BIND_IP")),
		Username:   strings.TrimSpace(os.Getenv("SOCKS5_USERNAME")),
		Password:   os.Getenv("SOCKS5_PASSWORD"),
		TCPTimeout: envInt("SOCKS5_TCP_TIMEOUT_SECONDS", 30),
		UDPTimeout: envInt("SOCKS5_UDP_TIMEOUT_SECONDS", 60),
		HealthAddr: firstNonEmpty(strings.TrimSpace(os.Getenv("SOCKS5_HEALTH_ADDR")), ":8080"),
	}

	if _, _, err := net.SplitHostPort(cfg.ListenAddr); err != nil {
		return config{}, err
	}
	if cfg.TCPTimeout < 0 {
		return config{}, errors.New("SOCKS5_TCP_TIMEOUT_SECONDS must be >= 0")
	}
	if cfg.UDPTimeout < 0 {
		return config{}, errors.New("SOCKS5_UDP_TIMEOUT_SECONDS must be >= 0")
	}
	return cfg, nil
}

func resolveBindIP(listenAddr, configured string) (string, error) {
	if configured != "" {
		if ip := net.ParseIP(configured); ip == nil {
			return "", errors.New("SOCKS5_BIND_IP must be a literal IP address")
		}
		return configured, nil
	}

	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", err
	}
	switch host {
	case "", "0.0.0.0", "::":
		return "", errors.New("SOCKS5_BIND_IP is required when SOCKS5_LISTEN_ADDR uses a wildcard host")
	default:
		return host, nil
	}
}

func startHealthServer(addr string) *http.Server {
	if strings.TrimSpace(addr) == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("health server error: %v", err)
		}
	}()
	return srv
}

func shutdownHTTPServer(srv *http.Server, name string) {
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("%s server shutdown error: %v", name, err)
	}
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
