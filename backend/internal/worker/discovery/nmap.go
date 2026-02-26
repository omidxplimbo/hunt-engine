
package discovery

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

type CommandExecutor func(targetID uint, name string, args ...string) ([]byte, error)

// RunNmapScan performs a port scan on live, non-CDN assets and updates them in the database.
func RunNmapScan(targetID uint, inputFile string, executor CommandExecutor) {
	// The logic from runPortScanForLiveNoCDNAssets starts here.
	var assets []models.Asset
	if err := database.DB.Model(&models.Asset{}).
		Where("target_id = ?", targetID).
		Where("is_live = ?", true).
		Where("jsonb_array_length(dnsx_ip) > 0").
		Where("(cdn_name IS NULL OR cdn_name = '' OR cdn_name = 'null')").
		Where("cdncheck = ?", false).
		Where("(cdncheck_name IS NULL OR cdncheck_name = '' OR cdncheck_name = 'null')").
		Find(&assets).Error; err != nil {
		log.Printf("❌ Port scan: failed to fetch eligible assets: %v\n", err)
		return
	}

	if len(assets) == 0 {
		log.Printf("ℹ️ Port scan: no eligible assets (live + dnsx + no CDN).\n")
		return
	}

	ipSet := make(map[string]struct{})
	for _, a := range assets {
		if a.DnsxIP == "" || a.DnsxIP == "[]" || a.DnsxIP == "null" {
			continue
		}
		var ips []string
		if err := json.Unmarshal([]byte(a.DnsxIP), &ips); err != nil {
			continue
		}
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			if strings.Contains(ip, ":") { // Skip IPv6 for now
				continue
			}
			ipSet[ip] = struct{}{}
		}
	}

	if len(ipSet) == 0 {
		log.Printf("ℹ️ Port scan: no IPv4 IPs extracted from dnsx.\n")
		return
	}

	ipList := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ipList = append(ipList, ip)
	}
	sort.Strings(ipList)

	results, err := runNmapTopPorts(targetID, ipList, inputFile, executor)
	if err != nil {
		if err.Error() == "process killed by user request" {
			return
		}
		log.Printf("⚠️ Port scan: nmap finished with error: %v\n", err)
	}

	updated := 0
	for i := range assets {
		a := &assets[i]
		var ips []string
		_ = json.Unmarshal([]byte(a.DnsxIP), &ips)
		m := make(map[string][]int)
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip == "" || strings.Contains(ip, ":") {
				continue
			}
			if ports, ok := results[ip]; ok && len(ports) > 0 {
				m[ip] = ports
			}
		}

		b, _ := json.Marshal(m)
		openPortsJSON := string(b)
		if openPortsJSON == "" || openPortsJSON == "null" {
			openPortsJSON = "{}"
		}

		if a.OpenPorts != openPortsJSON {
			if err := database.DB.Model(&models.Asset{}).Where("id = ?", a.ID).Update("open_ports", openPortsJSON).Error; err == nil {
				updated++
			}
		}
	}

	if updated > 0 {
		cache.IncrementTargetVersion(targetID)
	}

	log.Printf("✅ Port scan completed. Updated %d assets with open ports.\n", updated)
}

func runNmapTopPorts(targetID uint, ips []string, inputFile string, executor CommandExecutor) (map[string][]int, error) {
	results := make(map[string][]int)
	if len(ips) == 0 {
		return results, nil
	}
	if err := utils.WriteSliceToFile(inputFile, ips); err != nil {
		return results, err
	}


	out, err := executor(targetID, "nmap",
		"-n", "-Pn",
		"-sT",
		"--open",
		"--top-ports", "1000",
		"-T4",
		"--max-retries", "2",
		"--host-timeout", "45s",
		"--min-rate", "300",
		"-iL", inputFile,
		"-oG", "-",
	)
	if len(out) > 0 {
		results = parseNmapGreppable(out)
	}
	return results, err
}

func parseNmapGreppable(out []byte) map[string][]int {
	results := make(map[string][]int)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "Host:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ip := parts[1]
		if ip == "" {
			continue
		}
		portsIdx := strings.Index(line, "Ports:")
		if portsIdx == -1 {
			continue
		}
		portsPart := strings.TrimSpace(line[portsIdx+len("Ports:"):])
		if portsPart == "" {
			continue
		}
		if i := strings.Index(portsPart, "Ignored"); i != -1 {
			portsPart = strings.TrimSpace(portsPart[:i])
		}
		if i := strings.Index(portsPart, "OS:"); i != -1 {
			portsPart = strings.TrimSpace(portsPart[:i])
		}
		portItems := strings.Split(portsPart, ",")
		portSet := make(map[int]struct{})
		for _, item := range portItems {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			s := strings.Split(item, "/")
			if len(s) < 2 {
				continue
			}
			if s[1] != "open" {
				continue
			}
			p, err := strconv.Atoi(s[0])
			if err != nil || p <= 0 {
				continue
			}
			portSet[p] = struct{}{}
		}
		if len(portSet) == 0 {
			continue
		}
		ports := make([]int, 0, len(portSet))
		for p := range portSet {
			ports = append(ports, p)
		}
		sort.Ints(ports)
		results[ip] = ports
	}
	return results
}

