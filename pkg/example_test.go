package pkg_test

import (
	"fmt"
	"log"

	"github.com/lunar-bedrock/socks5-vpn/pkg/proxy"
	"github.com/lunar-bedrock/socks5-vpn/pkg/vpngate"
)

func Example() {
	// Fetch top 3 US VPN servers
	servers, err := vpngate.FetchTopServers("US", 3)
	if err != nil {
		log.Fatal(err)
	}

	for _, s := range servers {
		fmt.Printf("[%s] Score: %d, Speed: %.1f Mbps\n", s.IP, s.Score, s.SpeedMbps())
	}

	// Connect to remote Docker host
	mgr, err := proxy.ConnectWithKeyFile("1.2.3.4:22", "root", "/path/to/key")
	if err != nil {
		log.Fatal(err)
	}
	defer mgr.Close()

	// Start a proxy with the best server's config
	err = mgr.StartProxy(proxy.Config{
		ContainerName: "vpn-us-1",
		BindIP:        "1.2.3.4",
		ListenPort:    1080,
		Username:      "user",
		Password:      "pass",
		VPNConfig:     servers[0].ConfigData,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Check status
	status, _ := mgr.ProxyStatus("vpn-us-1")
	fmt.Println("Status:", status)

	// List all proxies
	proxies, _ := mgr.ListProxies()
	fmt.Println("Proxies:", proxies)

	// Stop proxy
	mgr.StopProxy("vpn-us-1")
}
