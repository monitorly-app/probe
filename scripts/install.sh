#!/bin/bash

# Script d'installation Monitorly Probe
# Télécharge et installe la dernière version de la probe depuis GitHub

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
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="arm" ;;
    *) echo "❌ Architecture non supportée: $ARCH"; exit 1 ;;
esac

# Détecter l'OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case $OS in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    *) echo "❌ OS non supporté: $OS"; exit 1 ;;
esac

echo "📋 Système détecté: $OS-$ARCH"

# Fonction pour obtenir la dernière version
get_latest_version() {
    # Essayer avec curl d'abord
    if command -v curl >/dev/null 2>&1; then
        curl -s https://api.github.com/repos/monitorly-app/probe/releases/latest | grep '"tag_name"' | cut -d'"' -f4
    # Sinon avec wget
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- https://api.github.com/repos/monitorly-app/probe/releases/latest | grep '"tag_name"' | cut -d'"' -f4
    else
        echo "❌ curl ou wget requis"
        exit 1
    fi
}

# Obtenir la dernière version
echo "🔍 Recherche de la dernière version..."
LATEST_VERSION=$(get_latest_version)

if [ -z "$LATEST_VERSION" ]; then
    echo "❌ Impossible de récupérer la dernière version"
    exit 1
fi

echo "📦 Version trouvée: $LATEST_VERSION"

# Construire l'URL de téléchargement
DOWNLOAD_URL="https://github.com/monitorly-app/probe/releases/download/${LATEST_VERSION}/monitorly-probe-${OS}-${ARCH}"

# Télécharger le binaire
echo "📥 Téléchargement..."
TEMP_FILE="/tmp/monitorly-probe-$$"

if command -v curl >/dev/null 2>&1; then
    curl -L -o "$TEMP_FILE" "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -O "$TEMP_FILE" "$DOWNLOAD_URL"
else
    echo "❌ curl ou wget requis"
    exit 1
fi

# Vérifier que le fichier a été téléchargé
if [ ! -f "$TEMP_FILE" ]; then
    echo "❌ Échec du téléchargement"
    exit 1
fi

# Rendre exécutable et installer
chmod +x "$TEMP_FILE"
sudo mv "$TEMP_FILE" /usr/local/bin/monitorly-probe

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