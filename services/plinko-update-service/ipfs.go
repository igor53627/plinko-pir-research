package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

type IPFSPublisher struct {
	client  *shell.Shell
	gateway string
}

func newIPFSPublisher(api, gateway string) (*IPFSPublisher, error) {
	api = strings.TrimSpace(api)
	if api == "" {
		return nil, nil
	}

	s := shell.NewShell(normalizeIPFSAPI(api))
	s.SetTimeout(15 * time.Second)
	
    // Try to contact the IPFS node to ensure it's up
    // If it fails, we just return the error, caller decides if fatal
	if _, err := s.ID(); err != nil {
		return nil, fmt.Errorf("ipfs api unhealthy: %w", err)
	}

	return &IPFSPublisher{
		client:  s,
		gateway: strings.TrimRight(gateway, "/"),
	}, nil
}

func (p *IPFSPublisher) PublishFile(path string) (string, error) {
	if p == nil || p.client == nil {
		return "", fmt.Errorf("ipfs publisher not configured")
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	cid, err := p.client.Add(f, shell.Pin(true), shell.CidVersion(1), shell.RawLeaves(true))
	if err != nil {
		return "", err
	}
	return cid, nil
}

func (p *IPFSPublisher) GatewayURL(cid string) string {
	if p == nil || cid == "" || p.gateway == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", p.gateway, cid)
}

func normalizeIPFSAPI(val string) string {
	trimmed := strings.TrimSpace(val)
	if strings.HasPrefix(trimmed, "/") {
		if hostPort := multiaddrToHostPort(trimmed); hostPort != "" {
			return hostPort
		}
	}
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimSuffix(trimmed, "/api/v0")
	return strings.Trim(trimmed, "/")
}

func multiaddrToHostPort(addr string) string {
	parts := strings.Split(addr, "/")
	var host, port string
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "ip4", "ip6", "dns", "dns4", "dns6":
			if i+1 < len(parts) {
				host = parts[i+1]
				i++
			}
		case "tcp":
			if i+1 < len(parts) {
				port = parts[i+1]
				i++
			}
		}
	}
	if host != "" && port != "" {
		return fmt.Sprintf("%s:%s", host, port)
	}
	return ""
}
