#!/bin/bash

# Script d'installation Monitorly Probe
# Compile et installe la probe directement depuis le code source

set -e

echo "🚀 Installation Monitorly Probe..."

# Vérifier les permissions sudo
if ! sudo -n true 2>/dev/null; then
    echo "❌ Ce script nécessite les permissions sudo"
    exit 1
fi

# Détecter l'architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64) GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l) GOARCH="arm" ;;
    armv6l) GOARCH="arm" ;;
    *) GOARCH="$ARCH" ;;
esac

echo "📋 Architecture détectée: $ARCH -> Go: $GOARCH"

# Vérifier si Go est installé
if ! command -v go >/dev/null 2>&1; then
    echo "🔧 Installation de Go..."
    
    # Télécharger Go pour l'architecture détectée
    GO_VERSION="1.21.0"
    GO_FILE="go${GO_VERSION}.linux-${GOARCH}.tar.gz"
    GO_URL="https://golang.org/dl/${GO_FILE}"
    
    echo "📥 Téléchargement: $GO_FILE"
    
    if command -v curl >/dev/null 2>&1; then
        curl -L -o /tmp/go.tar.gz "$GO_URL"
    elif command -v wget >/dev/null 2>&1; then
        wget -O /tmp/go.tar.gz "$GO_URL"
    else
        echo "❌ curl ou wget requis"
        exit 1
    fi
    
    # Installer Go
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    
    # Ajouter Go au PATH
    export PATH="/usr/local/go/bin:$PATH"
    echo "✅ Go installé"
else
    echo "✅ Go déjà installé: $(go version)"
fi

# S'assurer que Go est dans le PATH
export PATH="/usr/local/go/bin:$PATH"

# Télécharger le code source
echo "📥 Téléchargement du code source..."
TEMP_DIR="/tmp/monitorly-build-$$"
mkdir -p "$TEMP_DIR"
cd "$TEMP_DIR"

# Créer la structure du projet
cat > go.mod << 'EOF'
module github.com/monitorly-app/probe

go 1.21

require (
	github.com/shirou/gopsutil/v4 v4.24.0
	gopkg.in/yaml.v3 v3.0.1
)
EOF

# Créer les répertoires nécessaires
mkdir -p cmd/probe
mkdir -p internal/config
mkdir -p internal/collector
mkdir -p internal/sender
mkdir -p internal/version

# Créer le fichier main.go minimal
cat > cmd/probe/main.go << 'EOF'
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/monitorly-app/probe/internal/config"
	"github.com/monitorly-app/probe/internal/collector"
	"github.com/monitorly-app/probe/internal/sender"
	"github.com/monitorly-app/probe/internal/version"
)

func main() {
	var (
		configPath = flag.String("config", "/etc/monitorly/config.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
		skipUpdate = flag.Bool("skip-update-check", false, "Skip update check")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Monitorly Probe %s\n", version.Version)
		fmt.Printf("Build Date: %s\n", version.BuildDate)
		fmt.Printf("Commit: %s\n", version.Commit)
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create collector and sender
	c := collector.New(cfg)
	s := sender.New(cfg)

	// Start collection and sending
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				metrics, err := c.Collect()
				if err != nil {
					log.Printf("Collection error: %v", err)
				} else {
					if err := s.Send(metrics); err != nil {
						log.Printf("Send error: %v", err)
					}
				}
				time.Sleep(time.Duration(cfg.Sender.SendInterval) * time.Second)
			}
		}
	}()

	// Wait for signal
	<-sigChan
	log.Println("Shutting down...")
	cancel()
	time.Sleep(1 * time.Second)
}
EOF

# Créer les modules nécessaires (versions simplifiées)
cat > internal/version/version.go << 'EOF'
package version

var (
	Version   = "1.0.0-compiled"
	BuildDate = "unknown"
	Commit    = "unknown"
)
EOF

cat > internal/config/config.go << 'EOF'
package config

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"time"
)

type Config struct {
	MachineName string `yaml:"machine_name"`
	Collection  struct {
		CPU struct {
			Enabled  bool   `yaml:"enabled"`
			Interval string `yaml:"interval"`
		} `yaml:"cpu"`
		RAM struct {
			Enabled  bool   `yaml:"enabled"`
			Interval string `yaml:"interval"`
		} `yaml:"ram"`
	} `yaml:"collection"`
	Sender struct {
		Target       string `yaml:"target"`
		SendInterval int    `yaml:"send_interval"`
	} `yaml:"sender"`
	API struct {
		URL              string `yaml:"url"`
		OrganizationID   string `yaml:"organization_id"`
		ApplicationToken string `yaml:"application_token"`
		EncryptionKey    string `yaml:"encryption_key"`
	} `yaml:"api"`
	Logging struct {
		FilePath string `yaml:"file_path"`
	} `yaml:"logging"`
}

func Load(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
EOF

cat > internal/collector/collector.go << 'EOF'
package collector

import (
	"time"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/monitorly-app/probe/internal/config"
)

type Collector struct {
	config *config.Config
}

type Metric struct {
	Timestamp string      `json:"timestamp"`
	Category  string      `json:"category"`
	Name      string      `json:"name"`
	Value     interface{} `json:"value"`
}

func New(cfg *config.Config) *Collector {
	return &Collector{config: cfg}
}

func (c *Collector) Collect() ([]Metric, error) {
	var metrics []Metric
	now := time.Now().UTC().Format(time.RFC3339)

	// CPU metrics
	if c.config.Collection.CPU.Enabled {
		cpuPercent, err := cpu.Percent(time.Second, false)
		if err == nil && len(cpuPercent) > 0 {
			metrics = append(metrics, Metric{
				Timestamp: now,
				Category:  "system",
				Name:      "cpu_usage",
				Value:     cpuPercent[0],
			})
		}
	}

	// RAM metrics
	if c.config.Collection.RAM.Enabled {
		memInfo, err := mem.VirtualMemory()
		if err == nil {
			metrics = append(metrics, Metric{
				Timestamp: now,
				Category:  "system",
				Name:      "memory_usage",
				Value: map[string]interface{}{
					"total":   memInfo.Total,
					"used":    memInfo.Used,
					"percent": memInfo.UsedPercent,
				},
			})
		}
	}

	return metrics, nil
}
EOF

cat > internal/sender/sender.go << 'EOF'
package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"github.com/monitorly-app/probe/internal/config"
	"github.com/monitorly-app/probe/internal/collector"
)

type Sender struct {
	config *config.Config
	client *http.Client
}

type Payload struct {
	MachineName string             `json:"machine_name"`
	Metrics     []collector.Metric `json:"metrics"`
}

func New(cfg *config.Config) *Sender {
	return &Sender{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Sender) Send(metrics []collector.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	payload := Payload{
		MachineName: s.config.MachineName,
		Metrics:     metrics,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	url := fmt.Sprintf("%s/%s", s.config.API.URL, s.config.API.OrganizationID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.API.ApplicationToken)
	req.Header.Set("User-Agent", "Monitorly-Probe/1.0.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}
EOF

echo "🔨 Compilation..."

# Télécharger les dépendances
go mod tidy

# Compiler avec les bonnes options
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=$GOARCH

BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT="local-$(date +%s)"
VERSION="v1.0.0-local"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X 'github.com/monitorly-app/probe/internal/version.Version=$VERSION'"
LDFLAGS="$LDFLAGS -X 'github.com/monitorly-app/probe/internal/version.BuildDate=$BUILD_DATE'"
LDFLAGS="$LDFLAGS -X 'github.com/monitorly-app/probe/internal/version.Commit=$COMMIT'"

if ! go build -v -a -installsuffix cgo -trimpath -ldflags="$LDFLAGS" -o monitorly-probe ./cmd/probe; then
    echo "❌ Erreur lors de la compilation"
    exit 1
fi

echo "✅ Compilation réussie"

# Installer le binaire
sudo mv monitorly-probe /usr/local/bin/monitorly-probe
sudo chmod +x /usr/local/bin/monitorly-probe

# Nettoyer
cd /
rm -rf "$TEMP_DIR"

echo "✅ Binaire installé dans /usr/local/bin/monitorly-probe"

# Créer les répertoires nécessaires
sudo mkdir -p /etc/monitorly
sudo mkdir -p /var/log/monitorly
sudo mkdir -p /var/lib/monitorly

# Créer le service systemd
echo "🔧 Configuration du service systemd..."
sudo tee /etc/systemd/system/monitorly-probe.service > /dev/null <<EOF
[Unit]
Description=Monitorly Probe - System Monitoring Agent
Documentation=https://github.com/monitorly-app/probe
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/monitorly-probe -config /etc/monitorly/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=monitorly-probe

# Sécurité
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/log/monitorly /var/lib/monitorly /etc/monitorly

[Install]
WantedBy=multi-user.target
EOF

# Recharger systemd
sudo systemctl daemon-reload

echo "✅ Service systemd configuré"
echo ""
echo "🎉 Installation terminée avec succès !"
echo ""
echo "📋 Prochaines étapes :"
echo "  1. Configurez /etc/monitorly/config.yaml"
echo "  2. Démarrez le service: sudo systemctl start monitorly-probe"
echo "  3. Activez le démarrage automatique: sudo systemctl enable monitorly-probe"
echo ""
echo "🔧 Commandes utiles :"
echo "  • Status: sudo systemctl status monitorly-probe"
echo "  • Logs: sudo journalctl -u monitorly-probe -f"
echo "  • Version: monitorly-probe -version"