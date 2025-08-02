package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"mutual-friend/internal/cache"
	"mutual-friend/internal/domain"
	"mutual-friend/internal/repository"
	cacheTypes "mutual-friend/pkg/cache"
)

// FriendService handles friend-related business logic
type FriendService struct {
	friendRepo   *repository.FriendRepository
	userRepo     *repository.UserRepository
	eventService *EventService
	cache        cacheTypes.Cache
}

// NewFriendService creates a new friend service
func NewFriendService(
	friendRepo *repository.FriendRepository,
	userRepo *repository.UserRepository,
	eventService *EventService,
	cacheService cacheTypes.Cache,
) *FriendService {
	return &FriendService{
		friendRepo:   friendRepo,
		userRepo:     userRepo,
		eventService: eventService,
		cache:        cacheService,
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

	// Invalidate cache for both users
	s.invalidateUserCache(ctx, userID)
	s.invalidateUserCache(ctx, friendID)

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

	// Invalidate cache for both users
	s.invalidateUserCache(ctx, userID)
	s.invalidateUserCache(ctx, friendID)

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

// GetFriends retrieves all friends for a user with caching
func (s *FriendService) GetFriends(ctx context.Context, userID string, limit int, startKey map[string]interface{}) ([]*domain.Friend, map[string]interface{}, error) {
	// Validate user exists
	_, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("user not found: %w", err)
	}

	// Cache only the first page (when startKey is nil) for simplicity
	if startKey == nil {
		// Try to get from cache first
		cacheKey := cacheTypes.CacheKeyUserFriends.Format(userID)
		
		var cachedResult struct {
			Friends []*domain.Friend           `json:"friends"`
			NextKey map[string]interface{}     `json:"next_key"`
		}
		
		if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil {
			if err := json.Unmarshal([]byte(cachedData), &cachedResult); err == nil {
				log.Printf("Cache hit for user friends: %s", userID)
				return cachedResult.Friends, cachedResult.NextKey, nil
			}
			log.Printf("Failed to unmarshal cached friends data: %v", err)
		}
	}

	// Cache miss or pagination - get from database
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

	// Cache the result only for the first page
	if startKey == nil {
		cacheResult := struct {
			Friends []*domain.Friend           `json:"friends"`
			NextKey map[string]interface{}     `json:"next_key"`
		}{
			Friends: friends,
			NextKey: nextKey,
		}
		
		if resultJSON, err := json.Marshal(cacheResult); err == nil {
			cacheKey := cacheTypes.CacheKeyUserFriends.Format(userID)
			ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyUserFriends)
			
			if err := s.cache.Set(ctx, cacheKey, string(resultJSON), ttl); err != nil {
				log.Printf("Failed to cache friends for user %s: %v", userID, err)
			} else {
				log.Printf("Cached friends for user: %s", userID)
			}
		}
	}

	return friends, nextKey, nil
}

// GetFriendCount returns the number of friends for a user with caching
func (s *FriendService) GetFriendCount(ctx context.Context, userID string) (int, error) {
	// Validate user exists
	_, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("user not found: %w", err)
	}

	// Try to get from cache first
	cacheKey := cacheTypes.CacheKeyFriendCount.Format(userID)
	
	if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil {
		var count int
		if err := json.Unmarshal([]byte(cachedData), &count); err == nil {
			log.Printf("Cache hit for friend count: %s", userID)
			return count, nil
		}
		log.Printf("Failed to unmarshal cached friend count: %v", err)
	}

	// Cache miss - get from database
	count, err := s.friendRepo.GetFriendCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get friend count: %w", err)
	}

	// Cache the result
	if countJSON, err := json.Marshal(count); err == nil {
		ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyFriendCount)
		if err := s.cache.Set(ctx, cacheKey, string(countJSON), ttl); err != nil {
			log.Printf("Failed to cache friend count for user %s: %v", userID, err)
		} else {
			log.Printf("Cached friend count for user: %s", userID)
		}
	}

	return count, nil
}

// AreFriends checks if two users are friends with caching
func (s *FriendService) AreFriends(ctx context.Context, userID, friendID string) (bool, error) {
	// Try to get from cache first
	cacheKey := cacheTypes.CacheKeyFriendship.Format(userID, friendID)
	
	if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil {
		var areFriends bool
		if err := json.Unmarshal([]byte(cachedData), &areFriends); err == nil {
			log.Printf("Cache hit for friendship check: %s <-> %s", userID, friendID)
			return areFriends, nil
		}
		log.Printf("Failed to unmarshal cached friendship data: %v", err)
	}

	// Cache miss - get from database
	areFriends, err := s.friendRepo.AreFriends(ctx, userID, friendID)
	if err != nil {
		return false, err
	}

	// Cache the result
	if friendshipJSON, err := json.Marshal(areFriends); err == nil {
		ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyFriendship)
		if err := s.cache.Set(ctx, cacheKey, string(friendshipJSON), ttl); err != nil {
			log.Printf("Failed to cache friendship for %s <-> %s: %v", userID, friendID, err)
		} else {
			log.Printf("Cached friendship for: %s <-> %s", userID, friendID)
		}
	}

	return areFriends, nil
}

// GetMutualFriends finds mutual friends between two users with caching
func (s *FriendService) GetMutualFriends(ctx context.Context, userID1, userID2 string) ([]*domain.Friend, error) {
	// Validate users exist
	if err := s.validateUsers(ctx, userID1, userID2); err != nil {
		return nil, err
	}

	// Try to get from cache first (use consistent ordering for cache key)
	var cacheKey string
	if userID1 < userID2 {
		cacheKey = cacheTypes.CacheKeyMutualFriends.Format(userID1, userID2)
	} else {
		cacheKey = cacheTypes.CacheKeyMutualFriends.Format(userID2, userID1)
	}
	
	if cachedData, err := s.cache.Get(ctx, cacheKey); err == nil {
		var mutualFriends []*domain.Friend
		if err := json.Unmarshal([]byte(cachedData), &mutualFriends); err == nil {
			log.Printf("Cache hit for mutual friends: %s <-> %s", userID1, userID2)
			return mutualFriends, nil
		}
		log.Printf("Failed to unmarshal cached mutual friends data: %v", err)
	}

	// Cache miss - get from database
	mutualFriends, err := s.friendRepo.GetMutualFriends(ctx, userID1, userID2)
	if err != nil {
		return nil, fmt.Errorf("failed to get mutual friends: %w", err)
	}

	// Cache the result
	if mutualJSON, err := json.Marshal(mutualFriends); err == nil {
		ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyMutualFriends)
		if err := s.cache.Set(ctx, cacheKey, string(mutualJSON), ttl); err != nil {
			log.Printf("Failed to cache mutual friends for %s <-> %s: %v", userID1, userID2, err)
		} else {
			log.Printf("Cached mutual friends for: %s <-> %s", userID1, userID2)
		}
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

// invalidateUserCache invalidates all cache entries related to a user
func (s *FriendService) invalidateUserCache(ctx context.Context, userID string) {
	// List of cache keys to invalidate for this user
	keysToInvalidate := []string{
		cacheTypes.CacheKeyUserFriends.Format(userID),
		cacheTypes.CacheKeyFriendCount.Format(userID),
	}

	// Delete specific cache keys
	for _, key := range keysToInvalidate {
		if _, err := s.cache.Delete(ctx, key); err != nil {
			log.Printf("Failed to invalidate cache key %s: %v", key, err)
		} else {
			log.Printf("Invalidated cache key: %s", key)
		}
	}

	// Invalidate friendship pattern caches for this user
	friendshipPattern := fmt.Sprintf("friendship:%s:*", userID)
	if err := s.invalidatePattern(ctx, friendshipPattern); err != nil {
		log.Printf("Failed to invalidate friendship pattern %s: %v", friendshipPattern, err)
	}

	// Invalidate mutual friends pattern caches involving this user
	mutualPattern1 := fmt.Sprintf("mutual:friends:%s:*", userID)
	mutualPattern2 := fmt.Sprintf("mutual:friends:*:%s", userID)
	
	if err := s.invalidatePattern(ctx, mutualPattern1); err != nil {
		log.Printf("Failed to invalidate mutual friends pattern %s: %v", mutualPattern1, err)
	}
	if err := s.invalidatePattern(ctx, mutualPattern2); err != nil {
		log.Printf("Failed to invalidate mutual friends pattern %s: %v", mutualPattern2, err)
	}
}

// invalidatePattern invalidates cache keys matching a pattern
func (s *FriendService) invalidatePattern(ctx context.Context, pattern string) error {
	// This is a simplified implementation
	// In a production environment, you might want to use Redis SCAN for better performance
	// For now, we'll use the cache service's InvalidatePattern method if available
	
	// Check if the cache service supports pattern invalidation
	if cacheService, ok := s.cache.(*cache.Service); ok {
		return cacheService.InvalidatePattern(ctx, pattern)
	}
	
	// If pattern invalidation is not available, log a warning
	log.Printf("Pattern invalidation not supported for pattern: %s", pattern)
	return nil
}