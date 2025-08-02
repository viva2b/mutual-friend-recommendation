package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	pb "mutual-friend/api/proto"
)

func main() {
	// Connect to the gRPC server
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewFriendServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// User UUIDs from setup script
	userMap := map[string]string{
		"alice":   "2c1db4fa-4bfd-4b3f-8ba4-5b1e6af13cef",
		"bob":     "3044887a-2e71-4c9d-b9e6-455f1ca3d51b",
		"charlie": "e8f80bb6-328f-4063-afb9-ce9724c3520b",
		"diana":   "e8aed391-d481-4a63-80ac-c6e5edbfcb01",
		"eve":     "edb7acd0-5690-4fa1-b11d-7cbe2a9b8ac4",
	}

	fmt.Println("🚀 Comprehensive gRPC API Testing with Input/Output Details")
	fmt.Println("================================================================================")  // 80 equals signs

	// Test 1: HealthCheck
	fmt.Println("📊 1. HealthCheck RPC")
	fmt.Println("INPUT: &emptypb.Empty{}")
	
	healthResp, err := client.HealthCheck(ctx, &emptypb.Empty{})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT: {\n")
		fmt.Printf("  Status: %q\n", healthResp.Status)
		fmt.Printf("  Message: %q\n", healthResp.Message)
		fmt.Printf("  Timestamp: %v\n", healthResp.Timestamp.AsTime())
		fmt.Printf("}\n")
	}
	fmt.Println()

	// Test 2: GetFriendCount
	fmt.Println("📊 2. GetFriendCount RPC")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["alice"])
	fmt.Printf("}\n")
	
	countResp, err := client.GetFriendCount(ctx, &pb.GetFriendCountRequest{
		UserId: userMap["alice"],
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT: {\n")
		fmt.Printf("  UserId: %q\n", countResp.UserId)
		fmt.Printf("  Count: %d\n", countResp.Count)
		fmt.Printf("}\n")
	}
	fmt.Println()

	// Test 3: AreFriends
	fmt.Println("📊 3. AreFriends RPC")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["alice"])
	fmt.Printf("  FriendId: %q\n", userMap["bob"])
	fmt.Printf("}\n")
	
	friendsResp, err := client.AreFriends(ctx, &pb.AreFriendsRequest{
		UserId:   userMap["alice"],
		FriendId: userMap["bob"],
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT: {\n")
		fmt.Printf("  AreFriends: %t\n", friendsResp.AreFriends)
		if friendsResp.FriendshipCreatedAt != nil {
			fmt.Printf("  FriendshipCreatedAt: %v\n", friendsResp.FriendshipCreatedAt.AsTime())
		}
		fmt.Printf("}\n")
	}
	fmt.Println()

	// Test 4: GetFriends (Streaming)
	fmt.Println("📊 4. GetFriends RPC (Server Streaming)")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["alice"])
	fmt.Printf("  Limit: 10\n")
	fmt.Printf("  IncludeUserInfo: true\n")
	fmt.Printf("}\n")
	
	friendsStream, err := client.GetFriends(ctx, &pb.GetFriendsRequest{
		UserId:           userMap["alice"],
		Limit:            10,
		IncludeUserInfo:  true,
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT (Stream):\n")
		friendCount := 0
		for {
			friendResp, err := friendsStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				fmt.Printf("  ERROR: %v\n", err)
				break
			}
			friendCount++
			fmt.Printf("  Response %d: {\n", friendCount)
			fmt.Printf("    Friend: {\n")
			fmt.Printf("      UserId: %q\n", friendResp.Friend.UserId)
			fmt.Printf("      FriendId: %q\n", friendResp.Friend.FriendId)
			fmt.Printf("      CreatedAt: %v\n", friendResp.Friend.CreatedAt.AsTime())
			if friendResp.Friend.FriendInfo != nil {
				fmt.Printf("      FriendInfo: {\n")
				fmt.Printf("        UserId: %q\n", friendResp.Friend.FriendInfo.UserId)
				fmt.Printf("        Username: %q\n", friendResp.Friend.FriendInfo.Username)
				fmt.Printf("        Email: %q\n", friendResp.Friend.FriendInfo.Email)
				fmt.Printf("        DisplayName: %q\n", friendResp.Friend.FriendInfo.DisplayName)
				fmt.Printf("      }\n")
			}
			fmt.Printf("    }\n")
			if friendResp.TotalCount > 0 {
				fmt.Printf("    TotalCount: %d\n", friendResp.TotalCount)
			}
			fmt.Printf("  }\n")
		}
		fmt.Printf("Total streamed responses: %d\n", friendCount)
	}
	fmt.Println()

	// Test 5: GetMutualFriends (Streaming)
	fmt.Println("📊 5. GetMutualFriends RPC (Server Streaming)")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["alice"])
	fmt.Printf("  OtherUserId: %q\n", userMap["bob"])
	fmt.Printf("  Limit: 10\n")
	fmt.Printf("  IncludeUserInfo: true\n")
	fmt.Printf("}\n")
	
	mutualStream, err := client.GetMutualFriends(ctx, &pb.GetMutualFriendsRequest{
		UserId:         userMap["alice"],
		OtherUserId:    userMap["bob"],
		Limit:          10,
		IncludeUserInfo: true,
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT (Stream):\n")
		mutualCount := 0
		for {
			mutualResp, err := mutualStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				fmt.Printf("  ERROR: %v\n", err)
				break
			}
			mutualCount++
			fmt.Printf("  Response %d: {\n", mutualCount)
			fmt.Printf("    MutualFriend: {\n")
			fmt.Printf("      FriendId: %q\n", mutualResp.MutualFriend.FriendId)
			fmt.Printf("      MutualCount: %d\n", mutualResp.MutualFriend.MutualCount)
			if mutualResp.MutualFriend.FriendInfo != nil {
				fmt.Printf("      FriendInfo: {\n")
				fmt.Printf("        UserId: %q\n", mutualResp.MutualFriend.FriendInfo.UserId)
				fmt.Printf("        Username: %q\n", mutualResp.MutualFriend.FriendInfo.Username)
				fmt.Printf("        Email: %q\n", mutualResp.MutualFriend.FriendInfo.Email)
				fmt.Printf("      }\n")
			}
			fmt.Printf("    }\n")
			if mutualResp.TotalCount > 0 {
				fmt.Printf("    TotalCount: %d\n", mutualResp.TotalCount)
			}
			fmt.Printf("  }\n")
		}
		fmt.Printf("Total mutual friends: %d\n", mutualCount)
	}
	fmt.Println()

	// Test 6: AddFriend
	fmt.Println("📊 6. AddFriend RPC")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["eve"])
	fmt.Printf("  FriendId: %q\n", userMap["alice"])
	fmt.Printf("}\n")
	
	addResp, err := client.AddFriend(ctx, &pb.AddFriendRequest{
		UserId:   userMap["eve"],
		FriendId: userMap["alice"],
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT: {\n")
		fmt.Printf("  Success: %t\n", addResp.Success)
		fmt.Printf("  Message: %q\n", addResp.Message)
		if addResp.Friendship != nil {
			fmt.Printf("  Friendship: {\n")
			fmt.Printf("    UserId: %q\n", addResp.Friendship.UserId)
			fmt.Printf("    FriendId: %q\n", addResp.Friendship.FriendId)
			fmt.Printf("    CreatedAt: %v\n", addResp.Friendship.CreatedAt.AsTime())
			fmt.Printf("  }\n")
		}
		fmt.Printf("}\n")
	}
	fmt.Println()

	// Test 7: SearchUsers (Elasticsearch) - This is the key test!
	fmt.Println("📊 7. SearchUsers RPC (Elasticsearch Integration)")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  Query: \"alice\"\n")
	fmt.Printf("  Limit: 10\n")
	fmt.Printf("  IncludeHighlights: true\n")
	fmt.Printf("}\n")
	
	searchStream, err := client.SearchUsers(ctx, &pb.SearchUsersRequest{
		Query:             "alice",
		Limit:             10,
		IncludeHighlights: true,
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT (Stream):\n")
		searchCount := 0
		for {
			searchResp, err := searchStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				fmt.Printf("  ERROR: %v\n", err)
				break
			}
			searchCount++
			fmt.Printf("  SearchResult %d: {\n", searchCount)
			if searchResp.User != nil {
				fmt.Printf("    User: {\n")
				fmt.Printf("      UserId: %q\n", searchResp.User.User.UserId)
				fmt.Printf("      Username: %q\n", searchResp.User.User.Username)
				fmt.Printf("      Email: %q\n", searchResp.User.User.Email)
				fmt.Printf("      DisplayName: %q\n", searchResp.User.User.DisplayName)
				fmt.Printf("      RelevanceScore: %.4f\n", searchResp.User.RelevanceScore)
				fmt.Printf("      MutualFriendCount: %d\n", searchResp.User.MutualFriendCount)
				if searchResp.User.Highlight != "" {
					fmt.Printf("      Highlight: %q\n", searchResp.User.Highlight)
				}
				if searchResp.User.SocialMetrics != nil {
					fmt.Printf("      SocialMetrics: {\n")
					fmt.Printf("        FriendCount: %d\n", searchResp.User.SocialMetrics.FriendCount)
					fmt.Printf("        MutualFriendCount: %d\n", searchResp.User.SocialMetrics.MutualFriendCount)
					fmt.Printf("        PopularityScore: %.4f\n", searchResp.User.SocialMetrics.PopularityScore)
					fmt.Printf("        ActivityScore: %.4f\n", searchResp.User.SocialMetrics.ActivityScore)
					fmt.Printf("      }\n")
				}
				fmt.Printf("    }\n")
			}
			if searchResp.TotalCount > 0 {
				fmt.Printf("    TotalCount: %d\n", searchResp.TotalCount)
			}
			if searchResp.Metadata != nil {
				fmt.Printf("    Metadata: {\n")
				fmt.Printf("      TookMs: %d\n", searchResp.Metadata.TookMs)
				fmt.Printf("      TimedOut: %t\n", searchResp.Metadata.TimedOut)
				fmt.Printf("    }\n")
			}
			fmt.Printf("  }\n")
		}
		fmt.Printf("Total search results: %d\n", searchCount)
	}
	fmt.Println()

	// Test 8: GetUserSuggestions (Advanced Elasticsearch)
	fmt.Println("📊 8. GetUserSuggestions RPC (Advanced Elasticsearch)")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["alice"])
	fmt.Printf("  Limit: 5\n")
	fmt.Printf("  SuggestionType: \"mutual_friends\"\n")
	fmt.Printf("  IncludeUserInfo: true\n")
	fmt.Printf("}\n")
	
	suggestionsStream, err := client.GetUserSuggestions(ctx, &pb.GetUserSuggestionsRequest{
		UserId:          userMap["alice"],
		Limit:           5,
		SuggestionType:  "mutual_friends",
		IncludeUserInfo: true,
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT (Stream):\n")
		suggestionCount := 0
		for {
			suggestionResp, err := suggestionsStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				fmt.Printf("  ERROR: %v\n", err)
				break
			}
			suggestionCount++
			fmt.Printf("  Suggestion %d: {\n", suggestionCount)
			if suggestionResp.Suggestion != nil {
				fmt.Printf("    User: {\n")
				fmt.Printf("      UserId: %q\n", suggestionResp.Suggestion.User.UserId)
				fmt.Printf("      Username: %q\n", suggestionResp.Suggestion.User.Username)
				fmt.Printf("      Email: %q\n", suggestionResp.Suggestion.User.Email)
				fmt.Printf("    }\n")
				fmt.Printf("    SuggestionScore: %.4f\n", suggestionResp.Suggestion.SuggestionScore)
				fmt.Printf("    Reason: %q\n", suggestionResp.Suggestion.Reason)
				fmt.Printf("    MutualFriendCount: %d\n", suggestionResp.Suggestion.MutualFriendCount)
				if len(suggestionResp.Suggestion.SharedInterests) > 0 {
					fmt.Printf("    SharedInterests: %v\n", suggestionResp.Suggestion.SharedInterests)
				}
			}
			if suggestionResp.TotalCount > 0 {
				fmt.Printf("    TotalCount: %d\n", suggestionResp.TotalCount)
			}
			fmt.Printf("  }\n")
		}
		fmt.Printf("Total suggestions: %d\n", suggestionCount)
	}
	fmt.Println()

	// Test 9: RemoveFriend
	fmt.Println("📊 9. RemoveFriend RPC")
	fmt.Printf("INPUT: {\n")
	fmt.Printf("  UserId: %q\n", userMap["eve"])
	fmt.Printf("  FriendId: %q\n", userMap["alice"])
	fmt.Printf("}\n")
	
	removeResp, err := client.RemoveFriend(ctx, &pb.RemoveFriendRequest{
		UserId:   userMap["eve"],
		FriendId: userMap["alice"],
	})
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("OUTPUT: {\n")
		fmt.Printf("  Success: %t\n", removeResp.Success)
		fmt.Printf("  Message: %q\n", removeResp.Message)
		fmt.Printf("}\n")
	}
	fmt.Println()

	fmt.Println("================================================================================")
	fmt.Println("🎉 Comprehensive API Testing Complete!")
	fmt.Println("🔍 Elasticsearch Integration Status: Testing completed above")
}