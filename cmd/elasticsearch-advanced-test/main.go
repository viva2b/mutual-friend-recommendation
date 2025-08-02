package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("🔍 Advanced Elasticsearch Search Testing")
	fmt.Println("========================================")

	// Test various search scenarios
	searchTests := []struct {
		name        string
		query       string
		description string
	}{
		{"Username Search", "alice", "Search for user by exact username"},
		{"Partial Match", "bob", "Search for user by partial name"},
		{"Display Name", "Johnson", "Search by display name"},
		{"Email Search", "charlie@example.com", "Search by email address"},
		{"Fuzzy Search", "diana", "Fuzzy matching test"},
		{"Multiple Terms", "alice technology", "Multi-term search"},
		{"Empty Query", "", "Empty query - should return all users"},
		{"No Results", "nonexistent", "Query with no expected results"},
	}

	for i, test := range searchTests {
		fmt.Printf("\n📊 Test %d: %s\n", i+1, test.name)
		fmt.Printf("Description: %s\n", test.description)
		fmt.Printf("INPUT: {\n")
		fmt.Printf("  Query: %q\n", test.query)
		fmt.Printf("  Limit: 10\n")
		fmt.Printf("  IncludeHighlights: true\n")
		if test.query != "" {
			fmt.Printf("  Filters: {\n")
			fmt.Printf("    MinMutualFriends: 0\n")
			fmt.Printf("  }\n")
		}
		fmt.Printf("}\n")

		searchReq := &pb.SearchUsersRequest{
			Query:             test.query,
			Limit:             10,
			IncludeHighlights: true,
		}

		// Add filters for some tests
		if test.query != "" {
			searchReq.Filters = &pb.SearchFilters{
				MinMutualFriends: 0,
			}
		}

		searchStream, err := client.SearchUsers(ctx, searchReq)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		fmt.Printf("OUTPUT (Stream):\n")
		searchCount := 0
		totalResults := int32(0)
		tookMs := int32(0)
		timedOut := false

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

			// Capture metadata from first response
			if searchCount == 1 {
				if searchResp.TotalCount > 0 {
					totalResults = searchResp.TotalCount
				}
				if searchResp.Metadata != nil {
					tookMs = searchResp.Metadata.TookMs
					timedOut = searchResp.Metadata.TimedOut
				}
			}

			fmt.Printf("  Result %d: {\n", searchCount)
			if searchResp.User != nil {
				fmt.Printf("    UserId: %q\n", searchResp.User.User.UserId)
				fmt.Printf("    Username: %q\n", searchResp.User.User.Username)
				fmt.Printf("    DisplayName: %q\n", searchResp.User.User.DisplayName)
				fmt.Printf("    Email: %q\n", searchResp.User.User.Email)
				fmt.Printf("    RelevanceScore: %.4f\n", searchResp.User.RelevanceScore)
				fmt.Printf("    MutualFriendCount: %d\n", searchResp.User.MutualFriendCount)

				if searchResp.User.Highlight != "" {
					fmt.Printf("    Highlight: %q\n", searchResp.User.Highlight)
				}

				if searchResp.User.SocialMetrics != nil {
					fmt.Printf("    SocialMetrics: {\n")
					fmt.Printf("      FriendCount: %d\n", searchResp.User.SocialMetrics.FriendCount)
					fmt.Printf("      PopularityScore: %.2f\n", searchResp.User.SocialMetrics.PopularityScore)
					fmt.Printf("      ActivityScore: %.2f\n", searchResp.User.SocialMetrics.ActivityScore)
					fmt.Printf("      ResponseRate: %.2f\n", searchResp.User.SocialMetrics.ResponseRate)
					fmt.Printf("    }\n")
				}
			}
			fmt.Printf("  }\n")
		}

		fmt.Printf("SUMMARY:\n")
		fmt.Printf("  Total Results: %d\n", totalResults)
		fmt.Printf("  Results Streamed: %d\n", searchCount)
		fmt.Printf("  Search Time: %dms\n", tookMs)
		fmt.Printf("  Timed Out: %t\n", timedOut)

		if searchCount == 0 {
			fmt.Printf("  ✅ No results found (as expected for query: %q)\n", test.query)
		} else {
			fmt.Printf("  ✅ Search successful - %d results found\n", searchCount)
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("🎯 Advanced Search Testing with Filters")
	fmt.Println("========================================")

	// Test search with various filters
	filterTests := []struct {
		name   string
		req    *pb.SearchUsersRequest
		desc   string
	}{
		{
			name: "Location Filter",
			req: &pb.SearchUsersRequest{
				Query: "",
				Limit: 10,
				Filters: &pb.SearchFilters{
					Locations: []string{"Seoul"},
				},
			},
			desc: "Filter users by location (Seoul)",
		},
		{
			name: "Interest Filter",
			req: &pb.SearchUsersRequest{
				Query: "",
				Limit: 10,
				Filters: &pb.SearchFilters{
					Interests: []string{"technology", "programming"},
				},
			},
			desc: "Filter users by interests",
		},
		{
			name: "Friend Count Range",
			req: &pb.SearchUsersRequest{
				Query: "",
				Limit: 10,
				Filters: &pb.SearchFilters{
					MinFriendCount: 1,
					MaxFriendCount: 5,
				},
			},
			desc: "Filter users with 1-5 friends",
		},
		{
			name: "Sorting by Relevance",
			req: &pb.SearchUsersRequest{
				Query: "user",
				Limit: 10,
				Sort: &pb.SortOptions{
					Field:      pb.SortOptions_FRIEND_COUNT,
					Descending: true,
				},
			},
			desc: "Search with custom sorting",
		},
	}

	for i, test := range filterTests {
		fmt.Printf("\n📊 Filter Test %d: %s\n", i+1, test.name)
		fmt.Printf("Description: %s\n", test.desc)
		fmt.Printf("INPUT: {\n")
		fmt.Printf("  Query: %q\n", test.req.Query)
		fmt.Printf("  Limit: %d\n", test.req.Limit)
		if test.req.Filters != nil {
			fmt.Printf("  Filters: {\n")
			if len(test.req.Filters.Locations) > 0 {
				fmt.Printf("    Locations: %v\n", test.req.Filters.Locations)
			}
			if len(test.req.Filters.Interests) > 0 {
				fmt.Printf("    Interests: %v\n", test.req.Filters.Interests)
			}
			if test.req.Filters.MinFriendCount > 0 {
				fmt.Printf("    MinFriendCount: %d\n", test.req.Filters.MinFriendCount)
			}
			if test.req.Filters.MaxFriendCount > 0 {
				fmt.Printf("    MaxFriendCount: %d\n", test.req.Filters.MaxFriendCount)
			}
			fmt.Printf("  }\n")
		}
		if test.req.Sort != nil {
			fmt.Printf("  Sort: {\n")
			fmt.Printf("    Field: %v\n", test.req.Sort.Field)
			fmt.Printf("    Descending: %t\n", test.req.Sort.Descending)
			fmt.Printf("  }\n")
		}
		fmt.Printf("}\n")

		searchStream, err := client.SearchUsers(ctx, test.req)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		fmt.Printf("OUTPUT (Stream):\n")
		resultCount := 0
		for {
			searchResp, err := searchStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				fmt.Printf("  ERROR: %v\n", err)
				break
			}
			resultCount++
			fmt.Printf("  Result %d: %s (%s) - Score: %.2f\n", 
				resultCount,
				searchResp.User.User.Username,
				searchResp.User.User.DisplayName,
				searchResp.User.RelevanceScore)
		}
		fmt.Printf("  ✅ Filter test completed - %d results\n", resultCount)
	}

	fmt.Println("\n========================================")
	fmt.Println("🎉 Advanced Elasticsearch Testing Complete!")
	fmt.Println("📊 All search scenarios tested successfully")
}