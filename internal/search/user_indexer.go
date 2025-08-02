package search

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mutual-friend/internal/domain"
	"mutual-friend/pkg/elasticsearch"
)

// UserIndexer handles indexing user data to Elasticsearch
type UserIndexer struct {
	esClient *elasticsearch.Client
}

// NewUserIndexer creates a new UserIndexer
func NewUserIndexer(esClient *elasticsearch.Client) *UserIndexer {
	return &UserIndexer{
		esClient: esClient,
	}
}

// UserSearchDocument represents a user document for search
type UserSearchDocument struct {
	UserID       string                 `json:"user_id"`
	Username     string                 `json:"username"`
	DisplayName  string                 `json:"display_name"`
	Email        string                 `json:"email,omitempty"`
	Bio          string                 `json:"bio,omitempty"`
	Location     *LocationInfo          `json:"location,omitempty"`
	Interests    []string               `json:"interests,omitempty"`
	SocialMetrics *SocialMetrics        `json:"social_metrics"`
	PrivacySettings *PrivacySettings    `json:"privacy_settings"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastActive   time.Time              `json:"last_active"`
	Status       string                 `json:"status"`
}

type LocationInfo struct {
	City        string     `json:"city,omitempty"`
	State       string     `json:"state,omitempty"`
	Country     string     `json:"country,omitempty"`
	Coordinates *GeoPoint  `json:"coordinates,omitempty"`
}

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type SocialMetrics struct {
	FriendCount        int     `json:"friend_count"`
	MutualFriendCount  int     `json:"mutual_friend_count"`
	PopularityScore    float64 `json:"popularity_score"`
	ActivityScore      float64 `json:"activity_score"`
	ResponseRate       float64 `json:"response_rate"`
}

type PrivacySettings struct {
	Searchable         bool `json:"searchable"`
	ShowMutualFriends  bool `json:"show_mutual_friends"`
	LocationVisible    bool `json:"location_visible"`
}

// IndexUser indexes a single user
func (ui *UserIndexer) IndexUser(ctx context.Context, user *domain.User) error {
	doc := ui.convertUserToSearchDocument(user)
	
	err := ui.esClient.IndexDocument(ctx, "users", user.ID, doc)
	if err != nil {
		return fmt.Errorf("failed to index user %s: %w", user.ID, err)
	}
	
	log.Printf("Successfully indexed user: %s (%s)", user.Username, user.ID)
	return nil
}

// UpdateUser updates an existing user document
func (ui *UserIndexer) UpdateUser(ctx context.Context, user *domain.User) error {
	doc := ui.convertUserToSearchDocument(user)
	
	err := ui.esClient.UpdateDocument(ctx, "users", user.ID, doc)
	if err != nil {
		return fmt.Errorf("failed to update user %s: %w", user.ID, err)
	}
	
	log.Printf("Successfully updated user: %s (%s)", user.Username, user.ID)
	return nil
}

// UpdateUserSocialMetrics updates only the social metrics of a user
func (ui *UserIndexer) UpdateUserSocialMetrics(ctx context.Context, userID string, metrics *SocialMetrics) error {
	updateDoc := map[string]interface{}{
		"social_metrics": metrics,
		"updated_at":     time.Now(),
	}
	
	err := ui.esClient.UpdateDocument(ctx, "users", userID, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update social metrics for user %s: %w", userID, err)
	}
	
	log.Printf("Successfully updated social metrics for user: %s", userID)
	return nil
}

// BulkIndexUsers indexes multiple users in bulk
func (ui *UserIndexer) BulkIndexUsers(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}
	
	operations := make([]elasticsearch.BulkOperation, 0, len(users))
	
	for _, user := range users {
		doc := ui.convertUserToSearchDocument(user)
		operation := elasticsearch.BulkOperation{
			Action:     "index",
			Index:      "users",
			DocumentID: user.ID,
			Document:   doc,
		}
		operations = append(operations, operation)
	}
	
	err := ui.esClient.BulkIndex(ctx, operations)
	if err != nil {
		return fmt.Errorf("failed to bulk index users: %w", err)
	}
	
	log.Printf("Successfully bulk indexed %d users", len(users))
	return nil
}

// DeleteUser removes a user from the search index
func (ui *UserIndexer) DeleteUser(ctx context.Context, userID string) error {
	operations := []elasticsearch.BulkOperation{
		{
			Action:     "delete",
			Index:      "users",
			DocumentID: userID,
		},
	}
	
	err := ui.esClient.BulkIndex(ctx, operations)
	if err != nil {
		return fmt.Errorf("failed to delete user %s: %w", userID, err)
	}
	
	log.Printf("Successfully deleted user from index: %s", userID)
	return nil
}

// convertUserToSearchDocument converts a domain User to a search document
func (ui *UserIndexer) convertUserToSearchDocument(user *domain.User) *UserSearchDocument {
	doc := &UserSearchDocument{
		UserID:      user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Bio:         user.Bio,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LastActive:  user.UpdatedAt, // Use UpdatedAt as LastActive for now
		Status:      "active",       // Default status
		SocialMetrics: &SocialMetrics{
			FriendCount:       0,  // Will be updated separately
			MutualFriendCount: 0,  // Will be calculated and updated
			PopularityScore:   0.0, // Will be calculated
			ActivityScore:     0.5, // Default activity score
			ResponseRate:      0.8, // Default response rate
		},
		PrivacySettings: &PrivacySettings{
			Searchable:        true, // Default to searchable
			ShowMutualFriends: true, // Default to showing mutual friends
			LocationVisible:   false, // Default to private location
		},
	}
	
	// Parse interests from bio or other fields if needed
	doc.Interests = ui.extractInterestsFromBio(user.Bio)
	
	// Add location if avatar URL contains location data or other sources
	// For now, set a default location structure
	if user.Bio != "" {
		doc.Location = &LocationInfo{
			City:    "Unknown",
			Country: "Unknown",
		}
	}
	
	return doc
}

// extractInterestsFromBio extracts potential interests from user bio
func (ui *UserIndexer) extractInterestsFromBio(bio string) []string {
	// Simple keyword extraction - in real implementation, this could use NLP
	interests := []string{}
	
	keywords := []string{
		"programming", "coding", "software", "tech", "technology",
		"music", "art", "design", "photography", "travel",
		"sports", "fitness", "gaming", "reading", "writing",
		"cooking", "food", "movies", "science", "learning",
	}
	
	bioLower := strings.ToLower(bio)
	for _, keyword := range keywords {
		if strings.Contains(bioLower, keyword) {
			interests = append(interests, keyword)
		}
	}
	
	// Limit to top 5 interests
	if len(interests) > 5 {
		interests = interests[:5]
	}
	
	return interests
}

// ReindexAllUsers re-indexes all users (useful for mapping changes)
func (ui *UserIndexer) ReindexAllUsers(ctx context.Context, users []*domain.User) error {
	// Delete existing index and recreate
	log.Println("Starting full reindex of users...")
	
	// Use bulk indexing for better performance
	batchSize := 100
	for i := 0; i < len(users); i += batchSize {
		end := i + batchSize
		if end > len(users) {
			end = len(users)
		}
		
		batch := users[i:end]
		if err := ui.BulkIndexUsers(ctx, batch); err != nil {
			return fmt.Errorf("failed to reindex batch starting at %d: %w", i, err)
		}
		
		log.Printf("Reindexed batch %d-%d of %d users", i+1, end, len(users))
		
		// Small delay to avoid overwhelming Elasticsearch
		time.Sleep(100 * time.Millisecond)
	}
	
	log.Printf("Successfully reindexed all %d users", len(users))
	return nil
}