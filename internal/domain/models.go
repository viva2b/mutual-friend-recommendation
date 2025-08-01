package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID          string    `json:"id" dynamodbav:"id"`
	Username    string    `json:"username" dynamodbav:"username"`
	Email       string    `json:"email" dynamodbav:"email"`
	DisplayName string    `json:"display_name" dynamodbav:"display_name"`
	AvatarURL   string    `json:"avatar_url,omitempty" dynamodbav:"avatar_url,omitempty"`
	Bio         string    `json:"bio,omitempty" dynamodbav:"bio,omitempty"`
	CreatedAt   time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Friend represents a friendship relationship
type Friend struct {
	UserID    string    `json:"user_id" dynamodbav:"user_id"`
	FriendID  string    `json:"friend_id" dynamodbav:"friend_id"`
	Status    string    `json:"status" dynamodbav:"status"` // pending, accepted, blocked
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// FriendStatus constants
const (
	FriendStatusPending  = "pending"
	FriendStatusAccepted = "accepted"
	FriendStatusBlocked  = "blocked"
)

// Recommendation represents a friend recommendation
type Recommendation struct {
	UserID           string   `json:"user_id" dynamodbav:"user_id"`
	RecommendedID    string   `json:"recommended_id" dynamodbav:"recommended_id"`
	MutualFriends    []string `json:"mutual_friends" dynamodbav:"mutual_friends"`
	MutualFriendCount int     `json:"mutual_friend_count" dynamodbav:"mutual_friend_count"`
	Score            float64  `json:"score" dynamodbav:"score"`
	CreatedAt        time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// DynamoDBItem represents the structure stored in DynamoDB
type DynamoDBItem struct {
	PK     string    `json:"PK" dynamodbav:"PK"`
	SK     string    `json:"SK" dynamodbav:"SK"`
	GSI1PK string    `json:"GSI1PK,omitempty" dynamodbav:"GSI1PK,omitempty"`
	GSI1SK string    `json:"GSI1SK,omitempty" dynamodbav:"GSI1SK,omitempty"`
	Type   string    `json:"Type" dynamodbav:"Type"`
	Data   interface{} `json:"Data" dynamodbav:"Data"`
	TTL    *int64    `json:"TTL,omitempty" dynamodbav:"TTL,omitempty"`
}

// ItemType constants for Single Table Design
const (
	ItemTypeUser           = "USER"
	ItemTypeFriend         = "FRIEND"
	ItemTypeRecommendation = "RECOMMENDATION"
)

// Key patterns for Single Table Design
func UserPK(userID string) string {
	return "USER#" + userID
}

func UserProfileSK() string {
	return "PROFILE"
}

func FriendSK(friendID string) string {
	return "FRIEND#" + friendID
}

func RecommendationSK() string {
	return "RECOMMENDATIONS"
}

// GSI1 patterns for reverse lookups
func FriendGSI1PK(friendID string) string {
	return "FRIEND#" + friendID
}

func FriendGSI1SK(userID string) string {
	return "USER#" + userID
}

// NewUser creates a new user with generated ID
func NewUser(username, email, displayName string) *User {
	now := time.Now()
	return &User{
		ID:          uuid.New().String(),
		Username:    username,
		Email:       email,
		DisplayName: displayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewFriend creates a new friend relationship
func NewFriend(userID, friendID string) *Friend {
	now := time.Now()
	return &Friend{
		UserID:    userID,
		FriendID:  friendID,
		Status:    FriendStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewRecommendation creates a new recommendation
func NewRecommendation(userID, recommendedID string, mutualFriends []string, score float64) *Recommendation {
	now := time.Now()
	return &Recommendation{
		UserID:           userID,
		RecommendedID:    recommendedID,
		MutualFriends:    mutualFriends,
		MutualFriendCount: len(mutualFriends),
		Score:            score,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}