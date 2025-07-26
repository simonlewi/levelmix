#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}LevelMix GCP Setup Script${NC}"
echo "=========================="

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}gcloud CLI is not installed. Please install it first.${NC}"
    exit 1
fi

# Get project ID
read -p "Enter your GCP Project ID: " PROJECT_ID
read -p "Enter your desired GCP region (default: us-central1): " REGION
REGION=${REGION:-us-central1}

# Set project
echo -e "${YELLOW}Setting up GCP project...${NC}"
gcloud config set project $PROJECT_ID

# Enable required APIs
echo -e "${YELLOW}Enabling required APIs...${NC}"
gcloud services enable \
    container.googleapis.com \
    containerregistry.googleapis.com \
    cloudbuild.googleapis.com \
    compute.googleapis.com \
    redis.googleapis.com \
    secretmanager.googleapis.com

# Create GKE cluster
echo -e "${YELLOW}Creating GKE cluster...${NC}"
gcloud container clusters create levelmix-cluster \
    --zone=$REGION-a \
    --num-nodes=3 \
    --machine-type=n2-standard-2 \
    --enable-autoscaling \
    --min-nodes=3 \
    --max-nodes=10 \
    --enable-autorepair \
    --enable-autoupgrade \
    --release-channel=stable \
    --network=default \
    --enable-ip-alias \
    --enable-stackdriver-kubernetes

# Get cluster credentials
echo -e "${YELLOW}Getting cluster credentials...${NC}"
gcloud container clusters get-credentials levelmix-cluster --zone=$REGION-a

# Reserve static IP
echo -e "${YELLOW}Reserving static IP address...${NC}"
gcloud compute addresses create levelmix-ip --global

# Get the IP address
IP_ADDRESS=$(gcloud compute addresses describe levelmix-ip --global --format="get(address)")
echo -e "${GREEN}Reserved IP address: $IP_ADDRESS${NC}"
echo -e "${YELLOW}Please update your DNS A record for levelmix.io to point to: $IP_ADDRESS${NC}"

# Create Cloud Storage bucket for backups (optional)
echo -e "${YELLOW}Creating backup bucket...${NC}"
gsutil mb -p $PROJECT_ID -c STANDARD -l $REGION gs://levelmix-backups-$PROJECT_ID/

# Set up Cloud Build trigger
echo -e "${YELLOW}Creating Cloud Build trigger...${NC}"
gcloud builds triggers create github \
    --repo-name=levelmix \
    --repo-owner=YOUR_GITHUB_USERNAME \
    --branch-pattern="^main$" \
    --build-config=cloudbuild.yaml \
    --description="Deploy on push to main"

echo -e "${GREEN}Setup complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Update k8s/web-deployment.yaml and k8s/worker-deployment.yaml with your project ID"
echo "2. Create and configure k8s/secret.yaml with your actual secrets"
echo "3. Update your DNS records to point to $IP_ADDRESS"
echo "4. Run ./scripts/deploy.sh to deploy the application"