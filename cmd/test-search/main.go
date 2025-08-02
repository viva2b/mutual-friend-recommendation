package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mutual-friend/api/proto"
)

func main() {
	// Connect to gRPC server
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewFriendServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test user search
	log.Println("🔍 Testing user search...")
	testUserSearch(ctx, client)

	// Test user suggestions
	log.Println("💡 Testing user suggestions...")
	testUserSuggestions(ctx, client)

	// Test search performance
	log.Println("🏎️  Testing search performance...")
	testSearchPerformance(ctx, client)

	log.Println("✅ All search tests completed!")
}

func testUserSearch(ctx context.Context, client pb.FriendServiceClient) {
	// Test cases
	testCases := []struct {
		name  string
		query string
		limit int32
	}{
		{"Empty query", "", 5},
		{"Username search", "john", 10},
		{"Display name search", "developer", 5},
		{"Bio search", "programming", 8},
	}

	for _, tc := range testCases {
		log.Printf("  Testing: %s", tc.name)
		
		req := &pb.SearchUsersRequest{
			Query:             tc.query,
			Limit:             tc.limit,
			IncludeHighlights: true,
			Sort: &pb.SortOptions{
				Field:      pb.SortOptions_RELEVANCE,
				Descending: true,
			},
			Filters: &pb.SearchFilters{
				ExcludeFriends:    true,
				MinMutualFriends:  0,
				MinFriendCount:    1,
			},
		}

		stream, err := client.SearchUsers(ctx, req)
		if err != nil {
			log.Printf("    ❌ Failed to search users: %v", err)
			continue
		}

		resultCount := 0
		var totalCount int32
		var metadata *pb.SearchMetadata

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				log.Printf("    ❌ Failed to receive response: %v", err)
				break
			}

			if resultCount == 0 {
				totalCount = resp.TotalCount
				metadata = resp.Metadata
			}

			if resp.User != nil {
				log.Printf("    👤 Found: %s (%s) - Score: %.2f", 
					resp.User.User.Username, 
					resp.User.User.DisplayName,
					resp.User.RelevanceScore)
				
				if resp.User.Highlight != "" {
					log.Printf("      💡 Highlight: %s", resp.User.Highlight)
				}
			}

			resultCount++
		}

		log.Printf("    ✅ Results: %d/%d", resultCount, totalCount)
		if metadata != nil {
			log.Printf("    ⏱️  Search time: %dms", metadata.TookMs)
		}
		log.Println()
	}
}

func testUserSuggestions(ctx context.Context, client pb.FriendServiceClient) {
	// Test different suggestion types
	suggestionTypes := []string{
		"interest_based",
		"mutual_friends", 
		"location_based",
		"mixed",
	}

	testUserID := "test-user-123"

	for _, sugType := range suggestionTypes {
		log.Printf("  Testing suggestions: %s", sugType)
		
		req := &pb.GetUserSuggestionsRequest{
			UserId:         testUserID,
			Limit:          5,
			SuggestionType: sugType,
			IncludeUserInfo: true,
		}

		stream, err := client.GetUserSuggestions(ctx, req)
		if err != nil {
			log.Printf("    ❌ Failed to get suggestions: %v", err)
			continue
		}

		resultCount := 0
		var totalCount int32

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				log.Printf("    ❌ Failed to receive response: %v", err)
				break
			}

			if resultCount == 0 {
				totalCount = resp.TotalCount
			}

			if resp.Suggestion != nil {
				log.Printf("    🎯 Suggested: %s (%s) - Score: %.2f - Reason: %s", 
					resp.Suggestion.User.Username, 
					resp.Suggestion.User.DisplayName,
					resp.Suggestion.SuggestionScore,
					resp.Suggestion.Reason)
				
				if resp.Suggestion.MutualFriendCount > 0 {
					log.Printf("      👥 Mutual friends: %d", resp.Suggestion.MutualFriendCount)
				}
			}

			resultCount++
		}

		log.Printf("    ✅ Suggestions: %d/%d", resultCount, totalCount)
		log.Println()
	}
}

func testSearchPerformance(ctx context.Context, client pb.FriendServiceClient) {
	log.Println("🏎️  Testing search performance...")
	
	queries := []string{"john", "developer", "programming", "engineer", "designer"}
	iterations := 10
	
	for _, query := range queries {
		log.Printf("  Performance test for query: '%s'", query)
		
		var totalDuration time.Duration
		successCount := 0
		
		for i := 0; i < iterations; i++ {
			start := time.Now()
			
			req := &pb.SearchUsersRequest{
				Query: query,
				Limit: 10,
			}
			
			stream, err := client.SearchUsers(ctx, req)
			if err != nil {
				log.Printf("    ❌ Iteration %d failed: %v", i+1, err)
				continue
			}
			
			// Consume all results
			for {
				_, err := stream.Recv()
				if err != nil {
					break
				}
			}
			
			duration := time.Since(start)
			totalDuration += duration
			successCount++
		}
		
		if successCount > 0 {
			avgDuration := totalDuration / time.Duration(successCount)
			log.Printf("    ✅ Average response time: %v (%d/%d successful)", 
				avgDuration, successCount, iterations)
		}
	}
}