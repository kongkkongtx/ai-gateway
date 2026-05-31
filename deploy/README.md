# Kubernetes Deployment

## Quick Start

`ash
# 1. Set your DeepSeek API key
kubectl create secret generic ai-gateway-secrets \
  --from-literal=deepseek_api_key='sk-your-key-here'

# 2. Deploy
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/hpa.yaml

# 3. Check status
kubectl get pods -l app=ai-gateway
kubectl get svc ai-gateway

# 4. Test
kubectl port-forward svc/ai-gateway 8080:8080
curl http://localhost:8080/admin/health
`

## Configuration

The gateway configuration is stored in a ConfigMap (configmap.yaml).
Route and upstream changes can be made by editing the ConfigMap:

`ash
kubectl edit configmap ai-gateway-config
`

The gateway's config file watcher will detect the ConfigMap change and hot-reload automatically (within ~30s for K8s ConfigMap propagation + 500ms debounce).

## Scaling

The HPA (hpa.yaml) automatically scales between 2 and 10 replicas based on CPU (70%) and memory (80%) utilization.

Manual scaling:

`ash
kubectl scale deployment ai-gateway --replicas=5
`

## Exposing Externally

To expose the gateway outside the cluster, uncomment and configure ingress.yaml:

`ash
kubectl apply -f deploy/ingress.yaml
`

## Production Checklist

- [ ] Replace placeholder API key in secret.yaml or create via kubectl create secret
- [ ] Configure proper resource limits in deployment.yaml
- [ ] Set up Prometheus monitoring to scrape /metrics
- [ ] Configure external secrets management (Vault, External Secrets Operator)
- [ ] Add network policies to restrict inbound traffic
- [ ] Configure PodDisruptionBudget for HA