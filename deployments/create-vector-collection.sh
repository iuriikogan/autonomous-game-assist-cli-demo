#!/bin/bash
# Script to create a secure, serverless Vector Search 2.0 Collection with Gemini Auto-Embeddings 2.

set -euo pipefail

PROJECT_ID="${GCP_PROJECT:-develop-491110}"
LOCATION="${GCP_LOCATION:-us-central1}"
ENV="${ENV:-dev}"
COLLECTION_ID="${PROJECT_ID}-${ENV}-${LOCATION}-gameassist-collection"

echo "Creating Vector Search 2.0 Collection..."
echo "Project: ${PROJECT_ID}"
echo "Location: ${LOCATION}"
echo "Collection ID: ${COLLECTION_ID}"

# Define Data Schema
DATA_SCHEMA='{
  "type": "object",
  "properties": {
    "path": {
      "type": "string"
    },
    "type": {
      "type": "string"
    },
    "description": {
      "type": "string"
    }
  },
  "required": ["path", "type", "description"]
}'

# Define Vector Schema with Gemini Auto-Embeddings 2
VECTOR_SCHEMA='{
  "asset_embedding": {
    "denseVector": {
      "dimensions": 3072,
      "vertexEmbeddingConfig": {
        "modelId": "gemini-embedding-2-preview",
        "textTemplate": "Asset Path: {path}\nType: {type}\nDescription: {description}",
        "taskType": "RETRIEVAL_DOCUMENT"
      }
    }
  }
}'

# Call gcloud beta vector-search to create the collection asynchronously
gcloud beta vector-search collections create "${COLLECTION_ID}" \
  --project="${PROJECT_ID}" \
  --location="${LOCATION}" \
  --description="OpenWorldRPG structural semantic code and blueprint asset collection" \
  --data-schema="${DATA_SCHEMA}" \
  --vector-schema="${VECTOR_SCHEMA}" \
  --labels="environment=${ENV},owner=ikogan,cost-center=gaming-assist-ai,managed-by=vector-indexer"

echo "Collection creation long running operation triggered successfully."
echo "You can verify collection state using:"
echo "gcloud beta vector-search collections describe ${COLLECTION_ID} --location=${LOCATION} --project=${PROJECT_ID}"
