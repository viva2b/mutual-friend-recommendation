package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	
	appconfig "mutual-friend/pkg/config"
)

// DynamoDBClient wraps AWS DynamoDB client
type DynamoDBClient struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBClient creates a new DynamoDB client
func NewDynamoDBClient(cfg *appconfig.Config) (*DynamoDBClient, error) {
	// Create AWS config for local DynamoDB
	awsConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.DynamoDB.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if cfg.DynamoDB.Endpoint != "" {
					return aws.Endpoint{
						URL:           cfg.DynamoDB.Endpoint,
						SigningRegion: cfg.DynamoDB.Region,
					}, nil
				}
				// Return default endpoint for production
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			})),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"dummy", "dummy", "")), // DynamoDB Local doesn't require real credentials
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(awsConfig)

	return &DynamoDBClient{
		client:    client,
		tableName: cfg.DynamoDB.TableName,
	}, nil
}

// CreateTable creates the main table with Single Table Design
func (d *DynamoDBClient) CreateTable(ctx context.Context) error {
	// Check if table already exists
	_, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.tableName),
	})
	if err == nil {
		// Table already exists
		return nil
	}

	// Create table with Single Table Design
	createTableInput := &dynamodb.CreateTableInput{
		TableName: aws.String(d.tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("PK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("SK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("GSI1PK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("GSI1SK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("PK"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("SK"),
				KeyType:       types.KeyTypeRange,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("GSI1"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("GSI1PK"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("GSI1SK"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	}

	_, err = d.client.CreateTable(ctx, createTableInput)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Wait for table to be created
	waiter := dynamodb.NewTableExistsWaiter(d.client)
	err = waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.tableName),
	}, 30) // 30 seconds timeout for local
	if err != nil {
		return fmt.Errorf("failed waiting for table to be created: %w", err)
	}

	return nil
}

// TestConnection tests the connection to DynamoDB
func (d *DynamoDBClient) TestConnection(ctx context.Context) error {
	_, err := d.client.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		return fmt.Errorf("failed to connect to DynamoDB: %w", err)
	}
	return nil
}

// GetClient returns the underlying DynamoDB client
func (d *DynamoDBClient) GetClient() *dynamodb.Client {
	return d.client
}

// GetTableName returns the table name
func (d *DynamoDBClient) GetTableName() string {
	return d.tableName
}