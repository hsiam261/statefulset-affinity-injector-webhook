#!/usr/bin/env bash
# Usage: ./generate_crs_conf.sh --namespace <namespace> [--service <service>] [--release <release>]
# At least one of --service or --release must be set.

set -euo pipefail

# Global variables
NAMESPACE=""
SERVICE=""
RELEASE=""
CONF_FILE="csr.conf"


print_usage() {
  echo "Usage: $0 --namespace <namespace> [--service <service>] [--release <release>]"
  echo
  echo "Options:"
  echo "  --namespace   Kubernetes namespace (required)"
  echo "  --service     Service name (optional)"
  echo "  --release     Release name (optional)"
  echo "  -h, --help    Show this help message"
  echo
  echo "At least one of --service or --release must be provided."
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --namespace)
        NAMESPACE="$2"
        shift 2
        ;;
      --service)
        SERVICE="$2"
        shift 2
        ;;
      --release)
        RELEASE="$2"
        shift 2
        ;;
      -h|--help)
        print_usage
        exit 0
        ;;
      *)
        echo "❌ Unknown option: $1"
        print_usage
        exit 1
        ;;
    esac
  done
}

validate_args() {
  if [[ -z "$NAMESPACE" ]]; then
    echo "❌ Error: --namespace is required."
    exit 1
  fi

  if [[ -z "$SERVICE" && -z "$RELEASE" ]]; then
    echo "❌ Error: at least one of --service or --release must be provided."
    exit 1
  fi
}

generate_fullname() {
  local release_name="$1"
  local service_name="$2"
  local fullname=""

  if [[ -n "$service_name" ]]; then
    fullname="$service_name"
  elif [[ "$release_name" == *"statefulset-affinity-injector"* ]]; then
    fullname="$release_name"
  else
    fullname="${release_name}-statefulset-affinity-injector"
  fi

  # Truncate to 63 chars and trim trailing '-'
  fullname="${fullname:0:63}"
  fullname="${fullname%-}"

  echo "$fullname"
}

generate_csr_conf() {
  cat > "$CONF_FILE" <<EOF
# Output file
[ req ]
default_bits       = 2048
prompt             = no
default_md         = sha256
req_extensions     = v3_req
distinguished_name = dn

[ dn ]
CN = "${SERVICE}.${NAMESPACE}.svc"

[ v3_req ]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = ${SERVICE}
DNS.2 = ${SERVICE}.${NAMESPACE}
DNS.3 = ${SERVICE}.${NAMESPACE}.svc
DNS.4 = ${SERVICE}.${NAMESPACE}.svc.cluster.local
DNS.5 = localhost
IP.1  = 127.0.0.1
IP.2  = 0.0.0.0
EOF
}

generate_cert_and_key() {
  # Self-sign the certificate
  # openssl x509 -req -days 365 -in tls.csr -signkey tls.key -out tls.crt
  openssl req -x509 -nodes -days 365 \
    -newkey rsa:2048 \
    -keyout tls.key -out tls.crt \
    -config csr.conf -extensions v3_req
}

main() {
  parse_args "$@"
  validate_args

  echo "Namespace: $NAMESPACE"
  echo "Service: ${SERVICE:-<none>}"
  echo "Release: ${RELEASE:-<none>}"

  SERVICE=$(generate_fullname "$RELEASE" "$SERVICE")

  generate_csr_conf

  echo "✅ Configuration file '${CONF_FILE}' generated successfully."
  cat "$CONF_FILE"

  generate_cert_and_key

}

main "$@"
