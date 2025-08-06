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

// FriendRepository handles friend-related database operations
type FriendRepository struct {
	client *DynamoDBClient
}

// NewFriendRepository creates a new friend repository
func NewFriendRepository(client *DynamoDBClient) *FriendRepository {
	return &FriendRepository{
		client: client,
	}
}

// AddFriend creates a bidirectional friend relationship
func (r *FriendRepository) AddFriend(ctx context.Context, userID, friendID string) error {
	now := time.Now()

	// Create friend relationship for userID -> friendID
	friend1 := &domain.Friend{
		UserID:    userID,
		FriendID:  friendID,
		Status:    domain.FriendStatusAccepted,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create friend relationship for friendID -> userID (bidirectional)
	friend2 := &domain.Friend{
		UserID:    friendID,
		FriendID:  userID,
		Status:    domain.FriendStatusAccepted,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create DynamoDB items
	item1 := domain.DynamoDBItem{
		PK:     domain.UserPK(userID),
		SK:     domain.FriendSK(friendID),
		GSI1PK: domain.FriendGSI1PK(friendID),
		GSI1SK: domain.FriendGSI1SK(userID),
		Type:   domain.ItemTypeFriend,
		Data:   friend1,
	}

	item2 := domain.DynamoDBItem{
		PK:     domain.UserPK(friendID),
		SK:     domain.FriendSK(userID),
		GSI1PK: domain.FriendGSI1PK(userID),
		GSI1SK: domain.FriendGSI1SK(friendID),
		Type:   domain.ItemTypeFriend,
		Data:   friend2,
	}

	// Convert to DynamoDB attribute values
	av1, err := attributevalue.MarshalMap(item1)
	if err != nil {
		return fmt.Errorf("failed to marshal friend item 1: %w", err)
	}

	av2, err := attributevalue.MarshalMap(item2)
	if err != nil {
		return fmt.Errorf("failed to marshal friend item 2: %w", err)
	}

	// Use TransactWriteItems for atomicity
	_, err = r.client.GetClient().TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName:           aws.String(r.client.GetTableName()),
					Item:                av1,
					ConditionExpression: aws.String("attribute_not_exists(PK)"),
				},
			},
			{
				Put: &types.Put{
					TableName:           aws.String(r.client.GetTableName()),
					Item:                av2,
					ConditionExpression: aws.String("attribute_not_exists(PK)"),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add friend relationship: %w", err)
	}

	return nil
}

// RemoveFriend removes a bidirectional friend relationship
func (r *FriendRepository) RemoveFriend(ctx context.Context, userID, friendID string) error {
	// Use TransactWriteItems to remove both directions atomically
	_, err := r.client.GetClient().TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Delete: &types.Delete{
					TableName: aws.String(r.client.GetTableName()),
					Key: map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
						"SK": &types.AttributeValueMemberS{Value: domain.FriendSK(friendID)},
					},
					ConditionExpression: aws.String("attribute_exists(PK)"),
				},
			},
			{
				Delete: &types.Delete{
					TableName: aws.String(r.client.GetTableName()),
					Key: map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: domain.UserPK(friendID)},
						"SK": &types.AttributeValueMemberS{Value: domain.FriendSK(userID)},
					},
					ConditionExpression: aws.String("attribute_exists(PK)"),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to remove friend relationship: %w", err)
	}

	return nil
}

// GetFriends retrieves all friends for a user
func (r *FriendRepository) GetFriends(ctx context.Context, userID string, limit int, startKey map[string]types.AttributeValue) ([]*domain.Friend, map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.client.GetTableName()),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "FRIEND#"},
		},
	}

	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}

	if startKey != nil {
		input.ExclusiveStartKey = startKey
	}

	result, err := r.client.GetClient().Query(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get friends: %w", err)
	}

	var friends []*domain.Friend
	for _, item := range result.Items {
		var dbItem domain.DynamoDBItem
		err = attributevalue.UnmarshalMap(item, &dbItem)
		if err != nil {
			continue // Skip invalid items
		}

		friendData, ok := dbItem.Data.(map[string]interface{})
		if !ok {
			continue
		}

		var friend domain.Friend
		err = attributevalue.UnmarshalMap(convertInterfaceToAttributeValue(friendData), &friend)
		if err != nil {
			continue
		}

		friends = append(friends, &friend)
	}

	return friends, result.LastEvaluatedKey, nil
}

// GetFriendCount returns the number of friends for a user
func (r *FriendRepository) GetFriendCount(ctx context.Context, userID string) (int, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.client.GetTableName()),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "FRIEND#"},
		},
		Select: types.SelectCount,
	}

	result, err := r.client.GetClient().Query(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get friend count: %w", err)
	}

	return int(result.Count), nil
}

// AreFriends checks if two users are friends
func (r *FriendRepository) AreFriends(ctx context.Context, userID, friendID string) (bool, error) {
	result, err := r.client.GetClient().GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.client.GetTableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: domain.FriendSK(friendID)},
		},
	})
	if err != nil {
		return false, fmt.Errorf("failed to check friendship: %w", err)
	}

	return result.Item != nil, nil
}

// GetMutualFriends finds mutual friends between two users
func (r *FriendRepository) GetMutualFriends(ctx context.Context, userID1, userID2 string) ([]*domain.Friend, error) {
	// Get friends of user1
	friends1, _, err := r.GetFriends(ctx, userID1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get friends of user1: %w", err)
	}

	// Get friends of user2
	friends2, _, err := r.GetFriends(ctx, userID2, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get friends of user2: %w", err)
	}

	// Find mutual friends
	friendMap1 := make(map[string]*domain.Friend)
	for _, friend := range friends1 {
		friendMap1[friend.FriendID] = friend
	}

	var mutualFriends []*domain.Friend
	for _, friend := range friends2 {
		if _, exists := friendMap1[friend.FriendID]; exists {
			mutualFriends = append(mutualFriends, friend)
		}
	}

	return mutualFriends, nil
}

// SaveRecommendations saves user recommendations to DynamoDB
func (r *FriendRepository) SaveRecommendations(ctx context.Context, userID string, recommendations []domain.Recommendation) error {
	// Create UserRecommendations object
	userRecs := &domain.UserRecommendations{
		UserID:          userID,
		Recommendations: recommendations,
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	}

	// Create DynamoDB item
	item := domain.DynamoDBItem{
		PK:   domain.UserPK(userID),
		SK:   domain.RecommendationSK(),
		Type: domain.ItemTypeRecommendation,
		Data: userRecs,
		TTL:  aws.Int64(userRecs.ExpiresAt.Unix()), // Set TTL for automatic expiration
	}

	// Convert to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal recommendations: %w", err)
	}

	// Put item to DynamoDB
	_, err = r.client.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.client.GetTableName()),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to save recommendations: %w", err)
	}

	return nil
}

// GetRecommendations retrieves user recommendations from DynamoDB
func (r *FriendRepository) GetRecommendations(ctx context.Context, userID string) (*domain.UserRecommendations, error) {
	result, err := r.client.GetClient().GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.client.GetTableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: domain.UserPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: domain.RecommendationSK()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}

	if result.Item == nil {
		return nil, nil // No recommendations found
	}

	var dbItem domain.DynamoDBItem
	err = attributevalue.UnmarshalMap(result.Item, &dbItem)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	// Extract recommendations data
	recData, ok := dbItem.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid recommendations data format")
	}

	var userRecs domain.UserRecommendations
	err = attributevalue.UnmarshalMap(convertInterfaceToAttributeValue(recData), &userRecs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal recommendations: %w", err)
	}

	return &userRecs, nil
}