#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}LevelMix Deployment Script${NC}"
echo "========================="

# Check prerequisites
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}kubectl is not installed.${NC}"
    exit 1
fi

if ! command -v docker &> /dev/null; then
    echo -e "${RED}docker is not installed.${NC}"
    exit 1
fi

# Get project ID
PROJECT_ID=$(gcloud config get-value project)
if [ -z "$PROJECT_ID" ]; then
    echo -e "${RED}No GCP project set. Run: gcloud config set project YOUR_PROJECT_ID${NC}"
    exit 1
fi

echo -e "${YELLOW}Using project: $PROJECT_ID${NC}"

# Build and push images
echo -e "${YELLOW}Building Docker images...${NC}"
docker build -t gcr.io/$PROJECT_ID/levelmix-web:latest -f Dockerfile.web .
docker build -t gcr.io/$PROJECT_ID/levelmix-worker:latest -f Dockerfile.worker .

echo -e "${YELLOW}Pushing images to GCR...${NC}"
docker push gcr.io/$PROJECT_ID/levelmix-web:latest
docker push gcr.io/$PROJECT_ID/levelmix-worker:latest

# Update image references in k8s files
echo -e "${YELLOW}Updating Kubernetes manifests...${NC}"
sed -i "s|gcr.io/YOUR_PROJECT_ID|gcr.io/$PROJECT_ID|g" k8s/web-deployment.yaml
sed -i "s|gcr.io/YOUR_PROJECT_ID|gcr.io/$PROJECT_ID|g" k8s/worker-deployment.yaml

# Apply Kubernetes configurations
echo -e "${YELLOW}Applying Kubernetes configurations...${NC}"

# Create namespace
kubectl apply -f k8s/namespace.yaml

# Apply configs and secrets
kubectl apply -f k8s/configmap.yaml

# Check if secret exists
if [ ! -f "k8s/secret.yaml" ]; then
    echo -e "${RED}k8s/secret.yaml not found!${NC}"
    echo "Please create it from k8s/secret-template.yaml with your actual secrets"
    exit 1
fi
kubectl apply -f k8s/secret.yaml

# Deploy Redis
kubectl apply -f k8s/redis.yaml

# Wait for Redis to be ready
echo -e "${YELLOW}Waiting for Redis to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=redis -n levelmix --timeout=300s

# Deploy application
kubectl apply -f k8s/web-deployment.yaml
kubectl apply -f k8s/worker-deployment.yaml

# Apply ingress
kubectl apply -f k8s/ingress.yaml

# Wait for deployments
echo -e "${YELLOW}Waiting for deployments to be ready...${NC}"
kubectl rollout status deployment/levelmix-web -n levelmix
kubectl rollout status deployment/levelmix-worker -n levelmix

# Get ingress IP
echo -e "${GREEN}Deployment complete!${NC}"
echo ""
kubectl get ingress -n levelmix

echo ""
echo -e "${YELLOW}Note: It may take 10-15 minutes for the SSL certificate to be provisioned.${NC}"
echo -e "${YELLOW}Check certificate status with: kubectl describe managedcertificate levelmix-cert -n levelmix${NC}"