package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"mutual-friend/internal/domain"
)

// UserRepository handles user-related database operations
type UserRepository struct {
	client *DynamoDBClient
}

// NewUserRepository creates a new user repository
func NewUserRepository(client *DynamoDBClient) *UserRepository {
	return &UserRepository{
		client: client,
	}
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User) error {
	// Create DynamoDB item for user profile
	item := domain.DynamoDBItem{
		PK:   domain.UserPK(user.ID),
		SK:   domain.UserProfileSK(),
		Type: domain.ItemTypeUser,
		Data: user,
	}

	// Convert to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal user item: %w", err)
	}

	// Put item in DynamoDB
	_, err = r.client.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.client.GetTableName()),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"), // Prevent overwrite
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser retrieves a user by ID
func (r *UserRepository) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	// Get item from DynamoDB
	result, err := r.client.GetClient().GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.client.GetTableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: domain.UserProfileSK()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Unmarshal DynamoDB item
	var item domain.DynamoDBItem
	err = attributevalue.UnmarshalMap(result.Item, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user item: %w", err)
	}

	// Convert data to User struct
	userData, ok := item.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid user data format")
	}

	var user domain.User
	err = attributevalue.UnmarshalMap(convertInterfaceToAttributeValue(userData), &user)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (r *UserRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	user.UpdatedAt = time.Now()

	// Create DynamoDB item
	item := domain.DynamoDBItem{
		PK:   domain.UserPK(user.ID),
		SK:   domain.UserProfileSK(),
		Type: domain.ItemTypeUser,
		Data: user,
	}

	// Convert to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal user item: %w", err)
	}

	// Update item in DynamoDB
	_, err = r.client.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.client.GetTableName()),
		Item:                av,
		ConditionExpression: aws.String("attribute_exists(PK)"), // Ensure user exists
	})
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user
func (r *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	// Delete user profile
	_, err := r.client.GetClient().DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.client.GetTableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: domain.UserProfileSK()},
		},
		ConditionExpression: aws.String("attribute_exists(PK)"), // Ensure user exists
	})
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// ListUsers retrieves all users (for testing purposes)
func (r *UserRepository) ListUsers(ctx context.Context, limit int) ([]*domain.User, error) {
	// Query users with pagination
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.client.GetTableName()),
		FilterExpression: aws.String("#type = :type"),
		ExpressionAttributeNames: map[string]string{
			"#type": "Type",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":type": &types.AttributeValueMemberS{Value: domain.ItemTypeUser},
		},
	}

	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}

	result, err := r.client.GetClient().Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	var users []*domain.User
	for _, item := range result.Items {
		var dbItem domain.DynamoDBItem
		err = attributevalue.UnmarshalMap(item, &dbItem)
		if err != nil {
			continue // Skip invalid items
		}

		userData, ok := dbItem.Data.(map[string]interface{})
		if !ok {
			continue
		}

		var user domain.User
		err = attributevalue.UnmarshalMap(convertInterfaceToAttributeValue(userData), &user)
		if err != nil {
			continue
		}

		users = append(users, &user)
	}

	return users, nil
}

// Helper function to convert interface{} to AttributeValue map
func convertInterfaceToAttributeValue(data map[string]interface{}) map[string]types.AttributeValue {
	result := make(map[string]types.AttributeValue)
	
	for key, value := range data {
		av, err := attributevalue.Marshal(value)
		if err == nil {
			result[key] = av
		}
	}
	
	return result
}