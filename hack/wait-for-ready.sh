#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-kterodactyl-system}"
TIMEOUT="${2:-180s}"

echo "Waiting for deployment to be available in namespace $NAMESPACE..."
kubectl wait deployment -l app.kubernetes.io/name=kterodactyl \
  -n "$NAMESPACE" \
  --for=condition=Available \
  --timeout="$TIMEOUT"

echo "Verifying API is accessible at localhost:8080..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "Kterodactyl is ready at http://localhost:8080"
    exit 0
  fi
  sleep 2
done

echo "ERROR: API did not become accessible at localhost:8080 within 60s"
echo "Debug: kubectl get pods -n $NAMESPACE"
kubectl get pods -n "$NAMESPACE"
echo "Debug: kubectl logs -l app.kubernetes.io/name=kterodactyl -n $NAMESPACE --tail=50"
kubectl logs -l app.kubernetes.io/name=kterodactyl -n "$NAMESPACE" --tail=50 2>/dev/null || true
exit 1
