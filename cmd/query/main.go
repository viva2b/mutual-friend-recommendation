package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"mutual-friend/internal/repository"
	"mutual-friend/pkg/config"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create DynamoDB client
	dynamoClient, err := repository.NewDynamoDBClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client: %v", err)
	}

	// Create repositories
	userRepo := repository.NewUserRepository(dynamoClient)
	friendRepo := repository.NewFriendRepository(dynamoClient)

	fmt.Println("=== DynamoDB 테이블 분석 ===\n")

	// 1. 모든 사용자 조회
	fmt.Println("📋 사용자 목록:")
	users, err := userRepo.ListUsers(ctx, 10)
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		return
	}

	userMap := make(map[string]string) // ID -> Username
	for i, user := range users {
		fmt.Printf("%d. %s (%s)\n   - ID: %s\n   - Email: %s\n", 
			i+1, user.DisplayName, user.Username, user.ID, user.Email)
		userMap[user.ID] = user.Username
	}

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 2. 각 사용자별 친구 관계 분석
	fmt.Println("👥 친구 관계 분석:")
	for _, user := range users {
		friends, _, err := friendRepo.GetFriends(ctx, user.ID, 10, nil)
		if err != nil {
			log.Printf("Failed to get friends for %s: %v", user.Username, err)
			continue
		}

		fmt.Printf("🙍 %s의 친구들 (%d명):\n", user.Username, len(friends))
		for _, friend := range friends {
			friendName := userMap[friend.FriendID]
			if friendName == "" {
				friendName = friend.FriendID[:8] + "..."
			}
			fmt.Printf("   - %s (status: %s)\n", friendName, friend.Status)
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 50) + "\n")

	// 3. 상호 친구 분석
	fmt.Println("🤝 상호 친구 분석:")
	if len(users) >= 2 {
		for i := 0; i < len(users)-1; i++ {
			for j := i + 1; j < len(users); j++ {
				user1 := users[i]
				user2 := users[j]
				
				mutualFriends, err := friendRepo.GetMutualFriends(ctx, user1.ID, user2.ID)
				if err != nil {
					continue
				}

				if len(mutualFriends) > 0 {
					fmt.Printf("👫 %s와 %s의 공통 친구 (%d명):\n", 
						user1.Username, user2.Username, len(mutualFriends))
					for _, mutual := range mutualFriends {
						mutualName := userMap[mutual.FriendID]
						if mutualName == "" {
							mutualName = mutual.FriendID[:8] + "..."
						}
						fmt.Printf("   - %s\n", mutualName)
					}
					fmt.Println()
				}
			}
		}
	}

	fmt.Println(strings.Repeat("=", 50) + "\n")

	// 4. 스키마 구조 설명
	fmt.Println("🏗️  DynamoDB 스키마 구조:")
	fmt.Println("Primary Table:")
	fmt.Println("   PK (Partition Key) | SK (Sort Key)      | Type    | 설명")
	fmt.Println("   ------------------|-------------------|---------|------------------")
	fmt.Println("   USER#alice-uuid   | PROFILE           | USER    | 사용자 프로필")
	fmt.Println("   USER#alice-uuid   | FRIEND#bob-uuid   | FRIEND  | Alice의 친구 Bob")
	fmt.Println("   USER#alice-uuid   | RECOMMENDATIONS   | RECOM   | Alice의 추천 목록")
	fmt.Println()
	fmt.Println("GSI1 (Global Secondary Index):")
	fmt.Println("   GSI1PK             | GSI1SK            | 용도")
	fmt.Println("   ------------------|-------------------|------------------")
	fmt.Println("   FRIEND#bob-uuid   | USER#alice-uuid   | Bob을 친구로 둔 사용자들")
	fmt.Println()

	// 5. 실제 데이터 예시
	fmt.Println("📊 실제 저장된 데이터 샘플:")
	if len(users) > 0 {
		user := users[0]
		fmt.Printf("사용자 프로필 레코드:\n")
		fmt.Printf("   PK: USER#%s\n", user.ID)
		fmt.Printf("   SK: PROFILE\n")
		fmt.Printf("   Type: USER\n")
		fmt.Printf("   Data: {username: %s, email: %s, ...}\n\n", user.Username, user.Email)

		friends, _, _ := friendRepo.GetFriends(ctx, user.ID, 1, nil)
		if len(friends) > 0 {
			friend := friends[0]
			fmt.Printf("친구 관계 레코드:\n")
			fmt.Printf("   PK: USER#%s\n", user.ID)
			fmt.Printf("   SK: FRIEND#%s\n", friend.FriendID)
			fmt.Printf("   GSI1PK: FRIEND#%s\n", friend.FriendID)
			fmt.Printf("   GSI1SK: USER#%s\n", user.ID)
			fmt.Printf("   Type: FRIEND\n")
			fmt.Printf("   Data: {user_id: %s, friend_id: %s, status: %s}\n", 
				friend.UserID, friend.FriendID, friend.Status)
		}
	}
}