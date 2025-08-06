package batch

import (
	"context"
	"fmt"
	"sort"
	"time"

	"mutual-friend/internal/domain"
	"mutual-friend/internal/repository"
)

// RecommendationService handles friend recommendation calculations
type RecommendationService struct {
	friendRepo *repository.FriendRepository
}

// NewRecommendationService creates a new recommendation service
func NewRecommendationService(friendRepo *repository.FriendRepository) *RecommendationService {
	return &RecommendationService{
		friendRepo: friendRepo,
	}
}

// Calculate2HopRecommendations calculates friend recommendations using 2-hop BFS algorithm
func (rs *RecommendationService) Calculate2HopRecommendations(ctx context.Context, userID string) ([]domain.Recommendation, error) {
	// Get current friends of the user (1-hop)
	currentFriends, _, err := rs.friendRepo.GetFriends(ctx, userID, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user friends: %w", err)
	}

	// Create a map for quick lookup of current friends
	currentFriendMap := make(map[string]bool)
	currentFriendIDs := make([]string, 0, len(currentFriends))
	for _, friend := range currentFriends {
		currentFriendMap[friend.FriendID] = true
		currentFriendIDs = append(currentFriendIDs, friend.FriendID)
	}

	// Map to track visited users and avoid duplicates
	visited := make(map[string]bool)
	visited[userID] = true // Don't recommend self
	for friendID := range currentFriendMap {
		visited[friendID] = true // Don't recommend existing friends
	}

	// Map to store potential recommendations with their mutual friend connections
	candidateScores := make(map[string]*recommendationCandidate)

	// 2-Hop BFS: For each friend, get their friends
	for _, friendID := range currentFriendIDs {
		// Get friends of friend (2-hop)
		friendsOfFriend, _, err := rs.friendRepo.GetFriends(ctx, friendID, 0, nil)
		if err != nil {
			// Skip if we can't get this friend's connections
			continue
		}

		// Process each 2-hop connection
		for _, fof := range friendsOfFriend {
			candidateID := fof.FriendID

			// Skip if already visited (self, current friend, or already processed)
			if visited[candidateID] {
				continue
			}

			// Initialize or update candidate
			if candidate, exists := candidateScores[candidateID]; exists {
				// Add this mutual friend connection
				candidate.mutualFriends = append(candidate.mutualFriends, friendID)
				candidate.mutualFriendCount++
			} else {
				candidateScores[candidateID] = &recommendationCandidate{
					userID:            candidateID,
					mutualFriends:     []string{friendID},
					mutualFriendCount: 1,
				}
			}
		}
	}

	// Convert candidates to recommendations with scores
	recommendations := make([]domain.Recommendation, 0, len(candidateScores))
	for candidateID, candidate := range candidateScores {
		score := rs.calculateRecommendationScore(
			candidate.mutualFriendCount,
			len(currentFriends),
			candidate.mutualFriends,
		)

		recommendation := domain.Recommendation{
			UserID:            userID,
			RecommendedUserID: candidateID,
			MutualFriends:     candidate.mutualFriends,
			MutualFriendCount: candidate.mutualFriendCount,
			Score:             score,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		recommendations = append(recommendations, recommendation)
	}

	// Sort recommendations by score (descending) and limit to top 50
	sort.Slice(recommendations, func(i, j int) bool {
		// Primary sort by score
		if recommendations[i].Score != recommendations[j].Score {
			return recommendations[i].Score > recommendations[j].Score
		}
		// Secondary sort by mutual friend count
		if recommendations[i].MutualFriendCount != recommendations[j].MutualFriendCount {
			return recommendations[i].MutualFriendCount > recommendations[j].MutualFriendCount
		}
		// Tertiary sort by user ID for consistency
		return recommendations[i].RecommendedUserID < recommendations[j].RecommendedUserID
	})

	// Limit to top 50 recommendations
	if len(recommendations) > 50 {
		recommendations = recommendations[:50]
	}

	return recommendations, nil
}

// calculateRecommendationScore calculates a score for a recommendation
func (rs *RecommendationService) calculateRecommendationScore(mutualFriendCount, totalFriends int, mutualFriends []string) float64 {
	if totalFriends == 0 {
		return float64(mutualFriendCount)
	}

	// Base score: number of mutual friends
	baseScore := float64(mutualFriendCount)

	// Jaccard coefficient: mutual friends / total unique friends
	// This gives higher scores to users who share a larger percentage of the user's friend network
	jaccardScore := float64(mutualFriendCount) / float64(totalFriends)

	// Bonus for having multiple mutual friends (network density)
	densityBonus := 0.0
	if mutualFriendCount >= 3 {
		densityBonus = float64(mutualFriendCount-2) * 0.5
	}

	// Combined score with weights
	// - 50% weight on absolute mutual friend count
	// - 30% weight on Jaccard coefficient
	// - 20% weight on density bonus
	finalScore := (baseScore * 0.5) + (jaccardScore * 30.0) + (densityBonus * 0.2)

	return finalScore
}

// SaveRecommendations saves recommendations to DynamoDB
func (rs *RecommendationService) SaveRecommendations(ctx context.Context, userID string, recommendations []domain.Recommendation) error {
	if len(recommendations) == 0 {
		// No recommendations to save
		return nil
	}

	// Save to DynamoDB using the repository
	return rs.friendRepo.SaveRecommendations(ctx, userID, recommendations)
}

// recommendationCandidate is an internal struct for tracking recommendation candidates
type recommendationCandidate struct {
	userID            string
	mutualFriends     []string
	mutualFriendCount int
}

// GetMutualFriendsForRecommendation gets the list of mutual friends between two users
// This is useful for displaying why a recommendation was made
func (rs *RecommendationService) GetMutualFriendsForRecommendation(ctx context.Context, userID, recommendedUserID string) ([]string, error) {
	// Get friends of both users
	userFriends, _, err := rs.friendRepo.GetFriends(ctx, userID, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user friends: %w", err)
	}

	recommendedUserFriends, _, err := rs.friendRepo.GetFriends(ctx, recommendedUserID, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommended user friends: %w", err)
	}

	// Create a map of user's friends for quick lookup
	userFriendMap := make(map[string]bool)
	for _, friend := range userFriends {
		userFriendMap[friend.FriendID] = true
	}

	// Find mutual friends
	var mutualFriends []string
	for _, friend := range recommendedUserFriends {
		if userFriendMap[friend.FriendID] {
			mutualFriends = append(mutualFriends, friend.FriendID)
		}
	}

	return mutualFriends, nil
}

// BatchCalculateRecommendations calculates recommendations for multiple users in parallel
// This demonstrates the goroutine pipeline pattern for batch processing
func (rs *RecommendationService) BatchCalculateRecommendations(ctx context.Context, userIDs []string, workerCount int) (map[string][]domain.Recommendation, error) {
	// Create channels for the pipeline
	userIDChan := make(chan string, len(userIDs))
	resultChan := make(chan userRecommendationResult, len(userIDs))

	// Start worker goroutines
	for i := 0; i < workerCount; i++ {
		go rs.recommendationWorker(ctx, userIDChan, resultChan)
	}

	// Send user IDs to the channel
	for _, userID := range userIDs {
		userIDChan <- userID
	}
	close(userIDChan)

	// Collect results
	results := make(map[string][]domain.Recommendation)
	errors := make([]error, 0)

	for i := 0; i < len(userIDs); i++ {
		result := <-resultChan
		if result.err != nil {
			errors = append(errors, fmt.Errorf("failed to calculate recommendations for user %s: %w", result.userID, result.err))
		} else {
			results[result.userID] = result.recommendations
		}
	}

	// Return error if any calculations failed
	if len(errors) > 0 {
		return results, fmt.Errorf("batch calculation had %d errors: %v", len(errors), errors[0])
	}

	return results, nil
}

// recommendationWorker is a worker goroutine for batch recommendation calculation
func (rs *RecommendationService) recommendationWorker(ctx context.Context, userIDs <-chan string, results chan<- userRecommendationResult) {
	for userID := range userIDs {
		recommendations, err := rs.Calculate2HopRecommendations(ctx, userID)
		results <- userRecommendationResult{
			userID:          userID,
			recommendations: recommendations,
			err:             err,
		}
	}
}

// userRecommendationResult holds the result of a recommendation calculation
type userRecommendationResult struct {
	userID          string
	recommendations []domain.Recommendation
	err             error
}