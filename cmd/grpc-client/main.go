package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "mutual-friend/api/proto"
	"mutual-friend/pkg/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to gRPC server
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", cfg.GRPC.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	// Create client
	client := pb.NewFriendServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== gRPC Friend Service Client Test ===\n")

	// Test users (from our test data)
	userID1 := "f85d8354-c4e9-4e69-b80b-56a168c2933f" // Alice
	userID2 := "afd231e3-db12-4944-a213-55c7955ef0ac" // Bob
	userID3 := "37051378-2bc5-4e48-92a0-2f3b273cbfd8" // Diana

	// Test 1: Health Check
	fmt.Println("1️⃣ Testing Health Check...")
	healthResp, err := client.HealthCheck(ctx, &emptypb.Empty{})
	if err != nil {
		log.Printf("Health check failed: %v", err)
	} else {
		fmt.Printf("   Status: %s\n", healthResp.Status)
		fmt.Printf("   Message: %s\n", healthResp.Message)
	}

	// Test 2: Check if users are friends
	fmt.Println("\n2️⃣ Testing AreFriends...")
	areFriendsResp, err := client.AreFriends(ctx, &pb.AreFriendsRequest{
		UserId:   userID1,
		FriendId: userID2,
	})
	if err != nil {
		log.Printf("AreFriends failed: %v", err)
	} else {
		fmt.Printf("   %s and %s are friends: %v\n", 
			userID1[:8], userID2[:8], areFriendsResp.AreFriends)
	}

	// Test 3: Add friend if not already friends
	if areFriendsResp != nil && !areFriendsResp.AreFriends {
		fmt.Println("\n3️⃣ Testing AddFriend...")
		addResp, err := client.AddFriend(ctx, &pb.AddFriendRequest{
			UserId:   userID1,
			FriendId: userID2,
		})
		if err != nil {
			log.Printf("AddFriend failed: %v", err)
		} else {
			fmt.Printf("   Success: %v\n", addResp.Success)
			fmt.Printf("   Message: %s\n", addResp.Message)
		}
	}

	// Test 4: Get friend count
	fmt.Println("\n4️⃣ Testing GetFriendCount...")
	countResp, err := client.GetFriendCount(ctx, &pb.GetFriendCountRequest{
		UserId: userID1,
	})
	if err != nil {
		log.Printf("GetFriendCount failed: %v", err)
	} else {
		fmt.Printf("   User %s has %d friends\n", userID1[:8], countResp.Count)
	}

	// Test 5: Get friends list (streaming)
	fmt.Println("\n5️⃣ Testing GetFriends (streaming)...")
	friendsStream, err := client.GetFriends(ctx, &pb.GetFriendsRequest{
		UserId:           userID1,
		Limit:            10,
		IncludeUserInfo:  true,
	})
	if err != nil {
		log.Printf("GetFriends failed: %v", err)
	} else {
		fmt.Printf("   Friends of %s:\n", userID1[:8])
		friendCount := 0
		for {
			resp, err := friendsStream.Recv() 
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("   Error receiving friend: %v", err)
				break
			}
			
			friendCount++
			fmt.Printf("     %d. Friend ID: %s\n", friendCount, resp.Friend.FriendId[:8])
			if resp.Friend.FriendInfo != nil {
				fmt.Printf("        Display Name: %s\n", resp.Friend.FriendInfo.DisplayName)
			}
			
			if resp.TotalCount > 0 {
				fmt.Printf("        (Total: %d friends)\n", resp.TotalCount)
			}
		}
		fmt.Printf("   Received %d friends via streaming\n", friendCount)
	}

	// Test 6: Get mutual friends (streaming)
	fmt.Println("\n6️⃣ Testing GetMutualFriends (streaming)...")
	mutualStream, err := client.GetMutualFriends(ctx, &pb.GetMutualFriendsRequest{
		UserId:          userID1,
		OtherUserId:     userID3,
		Limit:           10,
		IncludeUserInfo: true,
	})
	if err != nil {
		log.Printf("GetMutualFriends failed: %v", err)
	} else {
		fmt.Printf("   Mutual friends between %s and %s:\n", userID1[:8], userID3[:8])
		mutualCount := 0
		for {
			resp, err := mutualStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("   Error receiving mutual friend: %v", err)
				break
			}
			
			mutualCount++
			fmt.Printf("     %d. Friend ID: %s\n", mutualCount, resp.MutualFriend.FriendId[:8])
			if resp.MutualFriend.FriendInfo != nil {
				fmt.Printf("        Display Name: %s\n", resp.MutualFriend.FriendInfo.DisplayName)
			}
			
			if resp.TotalCount > 0 {
				fmt.Printf("        (Total: %d mutual friends)\n", resp.TotalCount)
			}
		}
		fmt.Printf("   Found %d mutual friends via streaming\n", mutualCount)
	}

	// Test 7: Remove friend
	fmt.Println("\n7️⃣ Testing RemoveFriend...")
	removeResp, err := client.RemoveFriend(ctx, &pb.RemoveFriendRequest{
		UserId:   userID1,
		FriendId: userID2,
	})
	if err != nil {
		log.Printf("RemoveFriend failed: %v", err)
	} else {
		fmt.Printf("   Success: %v\n", removeResp.Success)
		fmt.Printf("   Message: %s\n", removeResp.Message)
	}

	// Test 8: Verify removal
	fmt.Println("\n8️⃣ Verifying friend removal...")
	areFriendsResp2, err := client.AreFriends(ctx, &pb.AreFriendsRequest{
		UserId:   userID1,
		FriendId: userID2,
	})
	if err != nil {
		log.Printf("AreFriends verification failed: %v", err)
	} else {
		fmt.Printf("   %s and %s are friends after removal: %v\n", 
			userID1[:8], userID2[:8], areFriendsResp2.AreFriends)
	}

	fmt.Println("\n✅ gRPC client test completed!")
	fmt.Println("📋 Check server logs for detailed processing information")
}