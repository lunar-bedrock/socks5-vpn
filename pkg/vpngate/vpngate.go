package vpngate

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const apiURL = "https://www.vpngate.net/api/iphone/"

type Server struct {
	Hostname    string
	IP          string
	Score       int
	Ping        int
	Speed       int // bytes per second
	Country     string
	CountryCode string
	Sessions    int
	Uptime      int64
	ConfigData  string // raw OpenVPN config
}

func (s Server) SpeedMbps() float64 {
	return float64(s.Speed) / 1_000_000
}

// FetchServers fetches all VPN Gate servers, optionally filtered by country code.
func FetchServers(countryCode string) ([]Server, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch vpngate api: %w", err)
	}
	defer resp.Body.Close()

	var servers []Server
	scanner := bufio.NewScanner(resp.Body)
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#") {
			continue
		}
		
		fields := strings.Split(line, ",")
		if len(fields) < 15 {
			continue
		}
		
		cc := fields[6]
		if countryCode != "" && cc != countryCode {
			continue
		}
		
		score, _ := strconv.Atoi(fields[2])
		ping, _ := strconv.Atoi(fields[3])
		speed, _ := strconv.Atoi(fields[4])
		sessions, _ := strconv.Atoi(fields[7])
		uptime, _ := strconv.ParseInt(fields[8], 10, 64)
		
		configB64 := fields[14]
		if configB64 == "" {
			continue
		}
		
		configBytes, err := base64.StdEncoding.DecodeString(configB64)
		if err != nil {
			continue
		}
		
		config := string(configBytes)
		config = fixCipherConfig(config)
		
		servers = append(servers, Server{
			Hostname:    fields[0],
			IP:          fields[1],
			Score:       score,
			Ping:        ping,
			Speed:       speed,
			Country:     fields[5],
			CountryCode: cc,
			Sessions:    sessions,
			Uptime:      uptime,
			ConfigData:  config,
		})
	}
	
	return servers, scanner.Err()
}

// FetchTopServers returns the top N servers for a country, sorted by score.
func FetchTopServers(countryCode string, count int) ([]Server, error) {
	servers, err := FetchServers(countryCode)
	if err != nil {
		return nil, err
	}
	
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Score > servers[j].Score
	})
	
	if count > len(servers) {
		count = len(servers)
	}
	
	return servers[:count], nil
}

// fixCipherConfig adds data-ciphers for OpenVPN 2.6+ compatibility.
func fixCipherConfig(config string) string {
	if !strings.Contains(config, "cipher AES-128-CBC") {
		return config
	}
	if strings.Contains(config, "data-ciphers") {
		return config
	}
	return strings.Replace(
		config,
		"cipher AES-128-CBC",
		"cipher AES-128-CBC\ndata-ciphers AES-128-CBC:AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305",
		1,
	)
}
