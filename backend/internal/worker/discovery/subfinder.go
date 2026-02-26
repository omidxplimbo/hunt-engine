package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
	"gopkg.in/yaml.v3"
)

// subfinderProviderKeys is the default set of providers emitted by subfinder v2.11.0
// when it generates /root/.config/subfinder/provider-config.yaml
var subfinderProviderKeys = []string{
	"alienvault",
	"bevigil",
	"bufferover",
	"builtwith",
	"c99",
	"censys",
	"certspotter",
	"chaos",
	"chinaz",
	"digitalyama",
	"dnsdb",
	"dnsdumpster",
	"dnsrepo",
	"domainsproject",
	"driftnet",
	"facebook",
	"fofa",
	"fullhunt",
	"github",
	"intelx",
	"leakix",
	"merklemap",
	"netlas",
	"onyphe",
	"profundis",
	"pugrecon",
	"quake",
	"redhuntlabs",
	"robtex",
	"rsecloud",
	"securitytrails",
	"shodan",
	"threatbook",
	"virustotal",
	"whoisxmlapi",
	"windvane",
	"zoomeyeapi",
}

// writeSubfinderProviderConfigFile builds a per-user provider-config.yaml for subfinder and writes it
// into the current target temp directory. Returns empty string if the user has no provider entries.
func WriteSubfinderProviderConfigFile(targetID uint, userID uint) (string, error) {
	if userID == 0 {
		return "", nil
	}

	var rows []models.SubfinderProviderConfig
	if err := database.DB.
		Where("user_id = ?", userID).
		Order("provider asc").
		Find(&rows).Error; err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}

	tempDir, _, err := utils.GetTargetTempDir(targetID)
	if err != nil {
		return "", err
	}

	// Start from subfinder defaults so unknown providers don't break anything and we keep a stable schema.
	cfg := make(map[string]interface{}, len(subfinderProviderKeys)+len(rows))
	for _, k := range subfinderProviderKeys {
		cfg[k] = []interface{}{}
	}

	for _, r := range rows {
		p := strings.ToLower(strings.TrimSpace(r.Provider))
		if p == "" {
			continue
		}
		var entries []interface{}
		if strings.TrimSpace(r.Entries) != "" {
			_ = json.Unmarshal([]byte(r.Entries), &entries)
		}
		if entries == nil {
			entries = []interface{}{}
		}
		cfg[p] = entries
	}

	yml, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}

	outPath := filepath.Join(tempDir, fmt.Sprintf("subfinder_provider-config_u%d.yaml", userID))
	if err := os.WriteFile(outPath, yml, 0600); err != nil {
		return "", err
	}
	return outPath, nil
}
