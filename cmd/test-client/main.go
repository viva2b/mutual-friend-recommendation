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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// User UUIDs from setup script
	userMap := map[string]string{
		"alice":   "2c1db4fa-4bfd-4b3f-8ba4-5b1e6af13cef",
		"bob":     "3044887a-2e71-4c9d-b9e6-455f1ca3d51b",
		"charlie": "e8f80bb6-328f-4063-afb9-ce9724c3520b",
		"diana":   "e8aed391-d481-4a63-80ac-c6e5edbfcb01",
		"eve":     "edb7acd0-5690-4fa1-b11d-7cbe2a9b8ac4",
	}

	fmt.Println("🚀 Starting gRPC API Tests...")
	fmt.Println("==================================================")  // 50 equals signs

	// Test 1: Health Check
	fmt.Println("📊 Test 1: Health Check")
	healthResp, err := client.HealthCheck(ctx, &emptypb.Empty{})
	if err != nil {
		log.Printf("❌ Health check failed: %v", err)
	} else {
		fmt.Printf("✅ Health check: %s - %s\n", healthResp.Status, healthResp.Message)
	}
	fmt.Println()

	// Test 2: Get Friend Count (should work for existing users from setup)
	fmt.Println("📊 Test 2: Get Friend Count")
	countResp, err := client.GetFriendCount(ctx, &pb.GetFriendCountRequest{
		UserId: userMap["alice"],
	})
	if err != nil {
		log.Printf("❌ Get friend count failed: %v", err)
	} else {
		fmt.Printf("✅ Alice's friend count: %d\n", countResp.Count)
	}
	fmt.Println()

	// Test 3: Check if users are friends (Alice and Bob should be friends from setup)
	fmt.Println("📊 Test 3: Check Friendship")
	friendsResp, err := client.AreFriends(ctx, &pb.AreFriendsRequest{
		UserId:   userMap["alice"],
		FriendId: userMap["bob"],
	})
	if err != nil {
		log.Printf("❌ Check friendship failed: %v", err)
	} else {
		fmt.Printf("✅ Alice and Bob are friends: %v\n", friendsResp.AreFriends)
		if friendsResp.AreFriends && friendsResp.FriendshipCreatedAt != nil {
			fmt.Printf("   Friendship created at: %v\n", friendsResp.FriendshipCreatedAt.AsTime())
		}
	}
	fmt.Println()

	// Test 4: Get Friends List (stream)
	fmt.Println("📊 Test 4: Get Friends List (Streaming)")
	friendsStream, err := client.GetFriends(ctx, &pb.GetFriendsRequest{
		UserId:           userMap["alice"],
		Limit:            10,
		IncludeUserInfo:  true,
	})
	if err != nil {
		log.Printf("❌ Get friends stream failed: %v", err)
	} else {
		friendCount := 0
		for {
			friendResp, err := friendsStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				log.Printf("❌ Stream receive failed: %v", err)
				break
			}
			friendCount++
			fmt.Printf("   Friend %d: %s (%s)\n", 
				friendCount, 
				friendResp.Friend.FriendId, 
				friendResp.Friend.FriendInfo.Username)
		}
		fmt.Printf("✅ Total friends streamed: %d\n", friendCount)
	}
	fmt.Println()

	// Test 5: Get Mutual Friends
	fmt.Println("📊 Test 5: Get Mutual Friends (Streaming)")
	mutualStream, err := client.GetMutualFriends(ctx, &pb.GetMutualFriendsRequest{
		UserId:         userMap["alice"],
		OtherUserId:    userMap["bob"],
		Limit:          10,
		IncludeUserInfo: true,
	})
	if err != nil {
		log.Printf("❌ Get mutual friends stream failed: %v", err)
	} else {
		mutualCount := 0
		for {
			mutualResp, err := mutualStream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				log.Printf("❌ Mutual friends stream receive failed: %v", err)
				break
			}
			mutualCount++
			fmt.Printf("   Mutual friend %d: %s (%s)\n", 
				mutualCount, 
				mutualResp.MutualFriend.FriendId,
				mutualResp.MutualFriend.FriendInfo.Username)
		}
		fmt.Printf("✅ Total mutual friends: %d\n", mutualCount)
	}
	fmt.Println()

	// Test 6: Add a new friendship
	fmt.Println("📊 Test 6: Add New Friendship")
	addResp, err := client.AddFriend(ctx, &pb.AddFriendRequest{
		UserId:   userMap["diana"],
		FriendId: userMap["charlie"],
	})
	if err != nil {
		log.Printf("❌ Add friend failed: %v", err)
	} else {
		fmt.Printf("✅ Add friend result: %v - %s\n", addResp.Success, addResp.Message)
		if addResp.Friendship != nil {
			fmt.Printf("   Friendship: %s <-> %s\n", 
				addResp.Friendship.UserId, 
				addResp.Friendship.FriendId)
		}
	}
	fmt.Println()

	// Test 7: Verify the new friendship
	fmt.Println("📊 Test 7: Verify New Friendship")
	verifyResp, err := client.AreFriends(ctx, &pb.AreFriendsRequest{
		UserId:   userMap["diana"],
		FriendId: userMap["charlie"],
	})
	if err != nil {
		log.Printf("❌ Verify friendship failed: %v", err)
	} else {
		fmt.Printf("✅ Diana and Charlie are friends: %v\n", verifyResp.AreFriends)
	}
	fmt.Println()

	// Test 8: Cache Performance Test
	fmt.Println("📊 Test 8: Cache Performance Test")
	start := time.Now()
	for i := 0; i < 5; i++ {
		_, err := client.GetFriendCount(ctx, &pb.GetFriendCountRequest{
			UserId: userMap["alice"],
		})
		if err != nil {
			log.Printf("❌ Cache test iteration %d failed: %v", i+1, err)
		}
	}
	duration := time.Since(start)
	fmt.Printf("✅ 5 friend count requests completed in %v (avg: %v per request)\n", 
		duration, duration/5)
	fmt.Println()

	fmt.Println("==================================================")
	fmt.Println("🎉 API Testing Complete!")
}