package events

import (
	"encoding/json"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	// Friend events
	EventTypeFriendAdded   EventType = "FRIEND_ADDED"
	EventTypeFriendRemoved EventType = "FRIEND_REMOVED"
	
	// User events
	EventTypeUserCreated EventType = "USER_CREATED"
	EventTypeUserUpdated EventType = "USER_UPDATED"
	EventTypeUserDeleted EventType = "USER_DELETED"
	
	// Recommendation events
	EventTypeRecommendationGenerated EventType = "RECOMMENDATION_GENERATED"
	EventTypeRecommendationUpdated   EventType = "RECOMMENDATION_UPDATED"
)

// BaseEvent represents the base structure for all events
type BaseEvent struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// FriendEvent represents friend-related events
type FriendEvent struct {
	BaseEvent
	UserID   string `json:"user_id"`
	FriendID string `json:"friend_id"`
	Status   string `json:"status,omitempty"`
}

// UserEvent represents user-related events
type UserEvent struct {
	BaseEvent
	UserID      string `json:"user_id"`
	Username    string `json:"username,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// RecommendationEvent represents recommendation-related events
type RecommendationEvent struct {
	BaseEvent
	UserID              string   `json:"user_id"`
	RecommendedUserID   string   `json:"recommended_user_id"`
	MutualFriends       []string `json:"mutual_friends,omitempty"`
	MutualFriendCount   int      `json:"mutual_friend_count"`
	Score              float64  `json:"score"`
}

// NewFriendAddedEvent creates a new friend added event
func NewFriendAddedEvent(userID, friendID, source string) *FriendEvent {
	return &FriendEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeFriendAdded,
			Source:    source,
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		UserID:   userID,
		FriendID: friendID,
		Status:   "accepted",
	}
}

// NewFriendRemovedEvent creates a new friend removed event
func NewFriendRemovedEvent(userID, friendID, source string) *FriendEvent {
	return &FriendEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeFriendRemoved,
			Source:    source,
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		UserID:   userID,
		FriendID: friendID,
	}
}

// NewUserCreatedEvent creates a new user created event
func NewUserCreatedEvent(userID, username, email, displayName, source string) *UserEvent {
	return &UserEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeUserCreated,
			Source:    source,
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		UserID:      userID,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
	}
}

// NewRecommendationGeneratedEvent creates a new recommendation generated event
func NewRecommendationGeneratedEvent(userID, recommendedUserID string, mutualFriends []string, score float64, source string) *RecommendationEvent {
	return &RecommendationEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeRecommendationGenerated,
			Source:    source,
			Timestamp: time.Now(),
			Version:   "1.0",
			Data:      make(map[string]interface{}),
		},
		UserID:            userID,
		RecommendedUserID: recommendedUserID,
		MutualFriends:     mutualFriends,
		MutualFriendCount: len(mutualFriends),
		Score:            score,
	}
}

// ToJSON converts event to JSON bytes
func (e *BaseEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToJSON converts friend event to JSON bytes
func (e *FriendEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToJSON converts user event to JSON bytes
func (e *UserEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToJSON converts recommendation event to JSON bytes  
func (e *RecommendationEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// GetRoutingKey returns the routing key for the event
func (e *FriendEvent) GetRoutingKey() string {
	switch e.Type {
	case EventTypeFriendAdded:
		return "friend.added"
	case EventTypeFriendRemoved:
		return "friend.removed"
	default:
		return "friend.unknown"
	}
}

// GetRoutingKey returns the routing key for the event
func (e *UserEvent) GetRoutingKey() string {
	switch e.Type {
	case EventTypeUserCreated:
		return "user.created"
	case EventTypeUserUpdated:
		return "user.updated"
	case EventTypeUserDeleted:
		return "user.deleted"
	default:
		return "user.unknown"
	}
}

// GetRoutingKey returns the routing key for the event
func (e *RecommendationEvent) GetRoutingKey() string {
	switch e.Type {
	case EventTypeRecommendationGenerated:
		return "recommendation.generated"
	case EventTypeRecommendationUpdated:
		return "recommendation.updated"
	default:
		return "recommendation.unknown"
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}