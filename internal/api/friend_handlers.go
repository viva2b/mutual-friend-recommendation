package api

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "mutual-friend/api/proto"
)

// AddFriend implements the AddFriend RPC method
func (s *FriendServiceServer) AddFriend(
	ctx context.Context,
	req *pb.AddFriendRequest,
) (*pb.AddFriendResponse, error) {
	// Validate request
	if err := validateAddFriendRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	log.Printf("Adding friend relationship: %s -> %s", req.UserId, req.FriendId)

	// Call service layer
	err := s.friendService.AddFriend(ctx, req.UserId, req.FriendId)
	if err != nil {
		log.Printf("Failed to add friend: %v", err)
		return nil, toGRPCError(err)
	}

	// Create response with friendship info
	friendship := &pb.Friend{
		UserId:    req.UserId,
		FriendId:  req.FriendId,
		CreatedAt: timestamppb.Now(),
	}

	return &pb.AddFriendResponse{
		Success:    true,
		Message:    "Friend relationship added successfully",
		Friendship: friendship,
	}, nil
}

// RemoveFriend implements the RemoveFriend RPC method
func (s *FriendServiceServer) RemoveFriend(
	ctx context.Context,
	req *pb.RemoveFriendRequest,
) (*pb.RemoveFriendResponse, error) {
	// Validate request
	if err := validateRemoveFriendRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	log.Printf("Removing friend relationship: %s -> %s", req.UserId, req.FriendId)

	// Call service layer
	err := s.friendService.RemoveFriend(ctx, req.UserId, req.FriendId)
	if err != nil {
		log.Printf("Failed to remove friend: %v", err)
		return nil, toGRPCError(err)
	}

	return &pb.RemoveFriendResponse{
		Success: true,
		Message: "Friend relationship removed successfully",
	}, nil
}

// GetFriends implements the GetFriends RPC method with server streaming
func (s *FriendServiceServer) GetFriends(
	req *pb.GetFriendsRequest,
	stream grpc.ServerStreamingServer[pb.GetFriendsResponse],
) error {
	// Validate request
	if err := validateGetFriendsRequest(req); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	log.Printf("Getting friends for user: %s (limit: %d)", req.UserId, req.Limit)

	// Set default limit if not provided
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Parse page token (simplified - in real implementation you'd decode it properly)
	var startKey map[string]interface{}
	if req.PageToken != "" {
		// For now, we'll keep it simple and not implement pagination
		startKey = nil
	}

	// Get friends from service
	friends, nextKey, err := s.friendService.GetFriends(stream.Context(), req.UserId, limit, startKey)
	if err != nil {
		log.Printf("Failed to get friends: %v", err)
		return toGRPCError(err)
	}

	// Get friend count for the first response
	totalCount, err := s.friendService.GetFriendCount(stream.Context(), req.UserId)
	if err != nil {
		log.Printf("Failed to get friend count: %v", err)
		// Don't fail the entire request for count error
		totalCount = 0
	}

	// Stream friends back to client
	for i, friend := range friends {
		pbFriend := &pb.Friend{
			UserId:    friend.UserID,
			FriendId:  friend.FriendID,
			CreatedAt: timestamppb.New(friend.CreatedAt),
		}

		// Include user info if requested (simplified for now)
		if req.IncludeUserInfo {
			// In a real implementation, you'd fetch user details
			pbFriend.FriendInfo = &pb.User{
				UserId:      friend.FriendID,
				Username:    fmt.Sprintf("user_%s", friend.FriendID[:8]),
				DisplayName: fmt.Sprintf("User %s", friend.FriendID[:8]),
			}
		}

		// Prepare response
		response := &pb.GetFriendsResponse{
			Friend: pbFriend,
		}

		// Include total count in first response
		if i == 0 {
			response.TotalCount = int32(totalCount)
		}

		// Include next page token in last response (if applicable)
		if i == len(friends)-1 && nextKey != nil {
			response.NextPageToken = "next_page_token_placeholder"
		}

		// Send to client
		if err := stream.Send(response); err != nil {
			log.Printf("Failed to send friend response: %v", err)
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	log.Printf("Successfully streamed %d friends for user %s", len(friends), req.UserId)
	return nil
}

// GetFriendCount implements the GetFriendCount RPC method
func (s *FriendServiceServer) GetFriendCount(
	ctx context.Context,
	req *pb.GetFriendCountRequest,
) (*pb.GetFriendCountResponse, error) {
	// Validate request
	if err := validateGetFriendCountRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	log.Printf("Getting friend count for user: %s", req.UserId)

	// Call service layer
	count, err := s.friendService.GetFriendCount(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get friend count: %v", err)
		return nil, toGRPCError(err)
	}

	return &pb.GetFriendCountResponse{
		UserId: req.UserId,
		Count:  int32(count),
	}, nil
}

// AreFriends implements the AreFriends RPC method
func (s *FriendServiceServer) AreFriends(
	ctx context.Context,
	req *pb.AreFriendsRequest,
) (*pb.AreFriendsResponse, error) {
	// Validate request
	if err := validateAreFriendsRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	log.Printf("Checking friendship: %s <-> %s", req.UserId, req.FriendId)

	// Call service layer
	areFriends, err := s.friendService.AreFriends(ctx, req.UserId, req.FriendId)
	if err != nil {
		log.Printf("Failed to check friendship: %v", err)
		return nil, toGRPCError(err)
	}

	response := &pb.AreFriendsResponse{
		AreFriends: areFriends,
	}

	// If they are friends, we could add the friendship creation timestamp
	// For now, keep it simple
	if areFriends {
		response.FriendshipCreatedAt = timestamppb.Now() // Placeholder
	}

	return response, nil
}

// GetMutualFriends implements the GetMutualFriends RPC method with server streaming
func (s *FriendServiceServer) GetMutualFriends(
	req *pb.GetMutualFriendsRequest,
	stream grpc.ServerStreamingServer[pb.GetMutualFriendsResponse],
) error {
	// Validate request
	if err := validateGetMutualFriendsRequest(req); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	log.Printf("Getting mutual friends: %s <-> %s", req.UserId, req.OtherUserId)

	// Call service layer
	mutualFriends, err := s.friendService.GetMutualFriends(stream.Context(), req.UserId, req.OtherUserId)
	if err != nil {
		log.Printf("Failed to get mutual friends: %v", err)
		return toGRPCError(err)
	}

	// Stream mutual friends back to client
	for i, friend := range mutualFriends {
		mutualFriend := &pb.MutualFriend{
			FriendId:    friend.FriendID,
			MutualCount: 1, // Simplified - in real implementation you'd calculate this
		}

		// Include user info if requested
		if req.IncludeUserInfo {
			mutualFriend.FriendInfo = &pb.User{
				UserId:      friend.FriendID,
				Username:    fmt.Sprintf("user_%s", friend.FriendID[:8]),
				DisplayName: fmt.Sprintf("User %s", friend.FriendID[:8]),
			}
		}

		// Prepare response
		response := &pb.GetMutualFriendsResponse{
			MutualFriend: mutualFriend,
		}

		// Include total count in first response
		if i == 0 {
			response.TotalCount = int32(len(mutualFriends))
		}

		// Send to client
		if err := stream.Send(response); err != nil {
			log.Printf("Failed to send mutual friend response: %v", err)
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	log.Printf("Successfully streamed %d mutual friends", len(mutualFriends))
	return nil
}

// GetRecommendations implements the GetRecommendations RPC method (placeholder for future implementation)
func (s *FriendServiceServer) GetRecommendations(
	req *pb.GetRecommendationsRequest,
	stream grpc.ServerStreamingServer[pb.GetRecommendationsResponse],
) error {
	// This will be implemented in later phases
	return status.Errorf(codes.Unimplemented, "recommendations feature not yet implemented")
}

// Validation functions
func validateAddFriendRequest(req *pb.AddFriendRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.FriendId == "" {
		return fmt.Errorf("friend_id is required")
	}
	if req.UserId == req.FriendId {
		return fmt.Errorf("user cannot be friends with themselves")
	}
	return nil
}

func validateRemoveFriendRequest(req *pb.RemoveFriendRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.FriendId == "" {
		return fmt.Errorf("friend_id is required")
	}
	if req.UserId == req.FriendId {
		return fmt.Errorf("user cannot remove themselves as friend")
	}
	return nil
}

func validateGetFriendsRequest(req *pb.GetFriendsRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if req.Limit > 100 {
		return fmt.Errorf("limit cannot exceed 100")
	}
	return nil
}

func validateGetFriendCountRequest(req *pb.GetFriendCountRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	return nil
}

func validateAreFriendsRequest(req *pb.AreFriendsRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.FriendId == "" {
		return fmt.Errorf("friend_id is required")
	}
	return nil
}

func validateGetMutualFriendsRequest(req *pb.GetMutualFriendsRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.OtherUserId == "" {
		return fmt.Errorf("other_user_id is required")
	}
	if req.UserId == req.OtherUserId {
		return fmt.Errorf("cannot get mutual friends with the same user")
	}
	if req.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if req.Limit > 100 {
		return fmt.Errorf("limit cannot exceed 100")
	}
	return nil
}