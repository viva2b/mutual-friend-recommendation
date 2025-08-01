package service

import (
	"context"
	"fmt"
	"log"

	"mutual-friend/internal/domain"
	"mutual-friend/internal/repository"
)

// FriendService handles friend-related business logic
type FriendService struct {
	friendRepo   *repository.FriendRepository
	userRepo     *repository.UserRepository
	eventService *EventService
}

// NewFriendService creates a new friend service
func NewFriendService(
	friendRepo *repository.FriendRepository,
	userRepo *repository.UserRepository,
	eventService *EventService,
) *FriendService {
	return &FriendService{
		friendRepo:   friendRepo,
		userRepo:     userRepo,
		eventService: eventService,
	}
}

// AddFriend adds a friend relationship with event publishing
func (s *FriendService) AddFriend(ctx context.Context, userID, friendID string) error {
	// Validate users exist
	if err := s.validateUsers(ctx, userID, friendID); err != nil {
		return err
	}

	// Check if already friends
	areFriends, err := s.friendRepo.AreFriends(ctx, userID, friendID)
	if err != nil {
		return fmt.Errorf("failed to check existing friendship: %w", err)
	}
	if areFriends {
		return fmt.Errorf("users are already friends")
	}

	// Add friend relationship to database
	if err := s.friendRepo.AddFriend(ctx, userID, friendID); err != nil {
		return fmt.Errorf("failed to add friend relationship: %w", err)
	}

	// Publish events for both directions (async)
	go func() {
		if err := s.eventService.PublishFriendAdded(context.Background(), userID, friendID); err != nil {
			log.Printf("Failed to publish FRIEND_ADDED event for %s -> %s: %v", userID, friendID, err)
		}
		if err := s.eventService.PublishFriendAdded(context.Background(), friendID, userID); err != nil {
			log.Printf("Failed to publish FRIEND_ADDED event for %s -> %s: %v", friendID, userID, err)
		}
	}()

	log.Printf("Friend relationship added: %s <-> %s", userID, friendID)
	return nil
}

// RemoveFriend removes a friend relationship with event publishing
func (s *FriendService) RemoveFriend(ctx context.Context, userID, friendID string) error {
	// Check if they are friends
	areFriends, err := s.friendRepo.AreFriends(ctx, userID, friendID)
	if err != nil {
		return fmt.Errorf("failed to check friendship: %w", err)
	}
	if !areFriends {
		return fmt.Errorf("users are not friends")
	}

	// Remove friend relationship from database
	if err := s.friendRepo.RemoveFriend(ctx, userID, friendID); err != nil {
		return fmt.Errorf("failed to remove friend relationship: %w", err)
	}

	// Publish events for both directions (async)
	go func() {
		if err := s.eventService.PublishFriendRemoved(context.Background(), userID, friendID); err != nil {
			log.Printf("Failed to publish FRIEND_REMOVED event for %s -> %s: %v", userID, friendID, err)
		}
		if err := s.eventService.PublishFriendRemoved(context.Background(), friendID, userID); err != nil {
			log.Printf("Failed to publish FRIEND_REMOVED event for %s -> %s: %v", friendID, userID, err)
		}
	}()

	log.Printf("Friend relationship removed: %s <-> %s", userID, friendID)
	return nil
}

// GetFriends retrieves all friends for a user
func (s *FriendService) GetFriends(ctx context.Context, userID string, limit int, startKey map[string]interface{}) ([]*domain.Friend, map[string]interface{}, error) {
	// Validate user exists
	_, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("user not found: %w", err)
	}

	// Convert startKey to DynamoDB AttributeValue format if needed
	// For now, we'll pass nil for simplicity
	friends, lastKey, err := s.friendRepo.GetFriends(ctx, userID, limit, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get friends: %w", err)
	}

	// Convert lastKey back to interface{} format
	var nextKey map[string]interface{}
	if lastKey != nil {
		nextKey = make(map[string]interface{})
		// This would require proper conversion from DynamoDB AttributeValue
		// For now, we'll keep it simple
	}

	return friends, nextKey, nil
}

// GetFriendCount returns the number of friends for a user
func (s *FriendService) GetFriendCount(ctx context.Context, userID string) (int, error) {
	// Validate user exists
	_, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("user not found: %w", err)
	}

	count, err := s.friendRepo.GetFriendCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get friend count: %w", err)
	}

	return count, nil
}

// AreFriends checks if two users are friends
func (s *FriendService) AreFriends(ctx context.Context, userID, friendID string) (bool, error) {
	return s.friendRepo.AreFriends(ctx, userID, friendID)
}

// GetMutualFriends finds mutual friends between two users
func (s *FriendService) GetMutualFriends(ctx context.Context, userID1, userID2 string) ([]*domain.Friend, error) {
	// Validate users exist
	if err := s.validateUsers(ctx, userID1, userID2); err != nil {
		return nil, err
	}

	mutualFriends, err := s.friendRepo.GetMutualFriends(ctx, userID1, userID2)
	if err != nil {
		return nil, fmt.Errorf("failed to get mutual friends: %w", err)
	}

	return mutualFriends, nil
}

// validateUsers checks if both users exist
func (s *FriendService) validateUsers(ctx context.Context, userID1, userID2 string) error {
	if userID1 == userID2 {
		return fmt.Errorf("cannot perform friend operation with same user")
	}

	// Check if first user exists
	_, err := s.userRepo.GetUser(ctx, userID1)
	if err != nil {
		return fmt.Errorf("user %s not found: %w", userID1, err)
	}

	// Check if second user exists
	_, err = s.userRepo.GetUser(ctx, userID2)
	if err != nil {
		return fmt.Errorf("user %s not found: %w", userID2, err)
	}

	return nil
}