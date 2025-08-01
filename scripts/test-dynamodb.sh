#!/bin/bash

# Test DynamoDB setup script

echo "🚀 Starting DynamoDB setup and test..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Start DynamoDB Local if not running
echo "📦 Starting DynamoDB Local..."
docker-compose up -d dynamodb

# Wait for DynamoDB to be ready
echo "⏳ Waiting for DynamoDB to be ready..."
sleep 5

# Check if DynamoDB is responding
max_attempts=30
attempt=0
while ! curl -s http://localhost:8000 > /dev/null; do
    attempt=$((attempt + 1))
    if [ $attempt -ge $max_attempts ]; then
        echo "❌ DynamoDB failed to start after $max_attempts attempts"
        exit 1
    fi
    echo "   Attempt $attempt/$max_attempts..."
    sleep 2
done

echo "✅ DynamoDB Local is ready!"

# Run the setup program
echo "🔧 Running DynamoDB setup and test..."
cd "$(dirname "$0")/.."
go run cmd/setup/main.go

if [ $? -eq 0 ]; then
    echo "✅ DynamoDB setup and test completed successfully!"
else
    echo "❌ DynamoDB setup failed"
    exit 1
fi

# Optional: Show DynamoDB tables
echo "📋 Listing DynamoDB tables:"
aws dynamodb list-tables --endpoint-url http://localhost:8000 --region us-east-1 2>/dev/null || echo "AWS CLI not available, skipping table list"

echo "🎉 All tests passed!"