package search

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"mutual-friend/pkg/elasticsearch"
	pb "mutual-friend/api/proto"
)

// SearchService handles user search operations
type SearchService struct {
	esClient *elasticsearch.Client
}

// NewSearchService creates a new SearchService
func NewSearchService(esClient *elasticsearch.Client) *SearchService {
	return &SearchService{
		esClient: esClient,
	}
}

// SearchUsers searches for users based on query and filters
func (ss *SearchService) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest, currentUserID string) ([]*pb.UserSearchResult, string, int32, error) {
	// Build Elasticsearch query
	query := ss.buildSearchQuery(req, currentUserID)
	
	// Execute search
	response, err := ss.esClient.Search(ctx, "users", query)
	if err != nil {
		return nil, "", 0, fmt.Errorf("search failed: %w", err)
	}
	
	// Convert results
	results := make([]*pb.UserSearchResult, 0, len(response.Hits.Hits))
	for _, hit := range response.Hits.Hits {
		result, err := ss.convertSearchHitToResult(hit, req.IncludeHighlights)
		if err != nil {
			log.Printf("Failed to convert search hit: %v", err)
			continue
		}
		results = append(results, result)
	}
	
	// Generate next page token
	var nextPageToken string
	if len(results) == int(req.Limit) {
		// Create pagination token
		offset := ss.getOffsetFromPageToken(req.PageToken)
		nextOffset := offset + int(req.Limit)
		nextPageToken = ss.createPageToken(nextOffset)
	}
	
	return results, nextPageToken, int32(response.Hits.Total.Value), nil
}

// GetUserSuggestions gets user suggestions based on interests and social connections
func (ss *SearchService) GetUserSuggestions(ctx context.Context, req *pb.GetUserSuggestionsRequest) ([]*pb.UserSuggestion, string, int32, error) {
	// Build suggestion query based on type
	var query map[string]interface{}
	
	switch req.SuggestionType {
	case "interest_based":
		query = ss.buildInterestBasedSuggestionQuery(req.UserId, int(req.Limit))
	case "mutual_friends":
		query = ss.buildMutualFriendsSuggestionQuery(req.UserId, int(req.Limit))
	case "location_based":
		query = ss.buildLocationBasedSuggestionQuery(req.UserId, int(req.Limit))
	default:
		// Default to mixed suggestions
		query = ss.buildMixedSuggestionQuery(req.UserId, int(req.Limit))
	}
	
	// Execute search
	response, err := ss.esClient.Search(ctx, "users", query)
	if err != nil {
		return nil, "", 0, fmt.Errorf("suggestion search failed: %w", err)
	}
	
	// Convert results
	suggestions := make([]*pb.UserSuggestion, 0, len(response.Hits.Hits))
	for _, hit := range response.Hits.Hits {
		suggestion, err := ss.convertSearchHitToSuggestion(hit, req.SuggestionType)
		if err != nil {
			log.Printf("Failed to convert search hit to suggestion: %v", err)
			continue
		}
		suggestions = append(suggestions, suggestion)
	}
	
	return suggestions, "", int32(len(suggestions)), nil
}

// buildSearchQuery builds the Elasticsearch query for user search
func (ss *SearchService) buildSearchQuery(req *pb.SearchUsersRequest, currentUserID string) map[string]interface{} {
	offset := ss.getOffsetFromPageToken(req.PageToken)
	
	query := map[string]interface{}{
		"from": offset,
		"size": req.Limit,
		"query": map[string]interface{}{
			"function_score": map[string]interface{}{
				"query": ss.buildBoolQuery(req, currentUserID),
				"functions": ss.buildScoringFunctions(),
				"score_mode": "sum",
				"boost_mode": "multiply",
				"min_score": 0.1,
			},
		},
		"sort": ss.buildSortClauses(req.Sort),
		"_source": []string{
			"user_id", "username", "display_name", "bio", "interests",
			"social_metrics.*", "location.city", "location.state",
			"created_at", "updated_at", "last_active",
		},
	}
	
	// Add highlighting if requested
	if req.IncludeHighlights {
		query["highlight"] = ss.buildHighlightConfig()
	}
	
	return query
}

// buildBoolQuery builds the boolean query part
func (ss *SearchService) buildBoolQuery(req *pb.SearchUsersRequest, currentUserID string) map[string]interface{} {
	boolQuery := map[string]interface{}{
		"must":     ss.buildMustClauses(req.Query),
		"filter":   ss.buildFilterClauses(req.Filters, currentUserID),
		"must_not": ss.buildMustNotClauses(req.Filters, currentUserID),
	}
	
	return map[string]interface{}{
		"bool": boolQuery,
	}
}

// buildMustClauses builds the must clauses for text search
func (ss *SearchService) buildMustClauses(query string) []map[string]interface{} {
	if query == "" {
		return []map[string]interface{}{
			{"match_all": map[string]interface{}{}},
		}
	}
	
	return []map[string]interface{}{
		{
			"multi_match": map[string]interface{}{
				"query": query,
				"fields": []string{
					"username^3",
					"display_name^2",
					"bio^1",
					"interests.text^1.5",
				},
				"type":          "best_fields",
				"fuzziness":     "AUTO",
				"prefix_length": 1,
			},
		},
	}
}

// buildFilterClauses builds filter clauses
func (ss *SearchService) buildFilterClauses(filters *pb.SearchFilters, currentUserID string) []map[string]interface{} {
	var filterClauses []map[string]interface{}
	
	// Privacy filter - only searchable users
	filterClauses = append(filterClauses, map[string]interface{}{
		"term": map[string]interface{}{
			"privacy_settings.searchable": true,
		},
	})
	
	// Active users only
	filterClauses = append(filterClauses, map[string]interface{}{
		"term": map[string]interface{}{
			"status": "active",
		},
	})
	
	if filters == nil {
		return filterClauses
	}
	
	// Location filter
	if len(filters.Locations) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{
				"location.city": filters.Locations,
			},
		})
	}
	
	// Interests filter
	if len(filters.Interests) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{
				"interests": filters.Interests,
			},
		})
	}
	
	// Friend count range
	if filters.MinFriendCount > 0 || filters.MaxFriendCount > 0 {
		rangeFilter := map[string]interface{}{}
		if filters.MinFriendCount > 0 {
			rangeFilter["gte"] = filters.MinFriendCount
		}
		if filters.MaxFriendCount > 0 {
			rangeFilter["lte"] = filters.MaxFriendCount
		}
		filterClauses = append(filterClauses, map[string]interface{}{
			"range": map[string]interface{}{
				"social_metrics.friend_count": rangeFilter,
			},
		})
	}
	
	// Mutual friends filter
	if filters.MinMutualFriends > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"range": map[string]interface{}{
				"social_metrics.mutual_friend_count": map[string]interface{}{
					"gte": filters.MinMutualFriends,
				},
			},
		})
	}
	
	return filterClauses
}

// buildMustNotClauses builds must_not clauses
func (ss *SearchService) buildMustNotClauses(filters *pb.SearchFilters, currentUserID string) []map[string]interface{} {
	var mustNotClauses []map[string]interface{}
	
	// Exclude current user
	mustNotClauses = append(mustNotClauses, map[string]interface{}{
		"term": map[string]interface{}{
			"user_id": currentUserID,
		},
	})
	
	// TODO: Exclude existing friends if requested
	// This would require getting friend IDs from the database
	
	return mustNotClauses
}

// buildScoringFunctions builds the function score functions
func (ss *SearchService) buildScoringFunctions() []map[string]interface{} {
	return []map[string]interface{}{
		// Boost based on mutual friend count
		{
			"field_value_factor": map[string]interface{}{
				"field":    "social_metrics.mutual_friend_count",
				"factor":   0.2,
				"modifier": "log1p",
				"missing":  0,
			},
			"weight": 1.5,
		},
		// Boost based on friend count (popularity)
		{
			"field_value_factor": map[string]interface{}{
				"field":    "social_metrics.friend_count",
				"factor":   0.1,
				"modifier": "log1p",
				"missing":  0,
			},
			"weight": 1.2,
		},
		// Boost based on activity score
		{
			"field_value_factor": map[string]interface{}{
				"field":    "social_metrics.activity_score",
				"factor":   0.3,
				"modifier": "none",
				"missing":  0,
			},
			"weight": 1.3,
		},
	}
}

// buildSortClauses builds sort clauses
func (ss *SearchService) buildSortClauses(sort *pb.SortOptions) []map[string]interface{} {
	if sort == nil {
		// Default sort by relevance
		return []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	}
	
	var field string
	switch sort.Field {
	case pb.SortOptions_MUTUAL_FRIENDS:
		field = "social_metrics.mutual_friend_count"
	case pb.SortOptions_CREATED_AT:
		field = "created_at"
	case pb.SortOptions_FRIEND_COUNT:
		field = "social_metrics.friend_count"
	case pb.SortOptions_ACTIVITY_SCORE:
		field = "social_metrics.activity_score"
	default:
		field = "_score"
	}
	
	order := "desc"
	if !sort.Descending {
		order = "asc"
	}
	
	return []map[string]interface{}{
		{field: map[string]interface{}{"order": order}},
		{"_score": map[string]interface{}{"order": "desc"}}, // Secondary sort by relevance
	}
}

// buildHighlightConfig builds highlighting configuration
func (ss *SearchService) buildHighlightConfig() map[string]interface{} {
	return map[string]interface{}{
		"fields": map[string]interface{}{
			"username": map[string]interface{}{
				"fragment_size": 150,
				"number_of_fragments": 1,
			},
			"display_name": map[string]interface{}{
				"fragment_size": 150,
				"number_of_fragments": 1,
			},
			"bio": map[string]interface{}{
				"fragment_size": 150,
				"number_of_fragments": 2,
			},
		},
		"pre_tags":  []string{"<em>"},
		"post_tags": []string{"</em>"},
	}
}

// Suggestion query builders
func (ss *SearchService) buildInterestBasedSuggestionQuery(userID string, limit int) map[string]interface{} {
	// Simplified - in real implementation, would get user's interests first
	return map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"interests": []string{"programming", "tech", "music"}, // Would be user's actual interests
							"boost":     2.0,
						},
					},
				},
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"status": "active"}},
					{"term": map[string]interface{}{"privacy_settings.searchable": true}},
				},
				"must_not": []map[string]interface{}{
					{"term": map[string]interface{}{"user_id": userID}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"social_metrics.activity_score": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		},
	}
}

func (ss *SearchService) buildMutualFriendsSuggestionQuery(userID string, limit int) map[string]interface{} {
	return map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"status": "active"}},
					{"term": map[string]interface{}{"privacy_settings.searchable": true}},
					{"range": map[string]interface{}{
						"social_metrics.mutual_friend_count": map[string]interface{}{
							"gt": 0,
						},
					}},
				},
				"must_not": []map[string]interface{}{
					{"term": map[string]interface{}{"user_id": userID}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"social_metrics.mutual_friend_count": map[string]interface{}{"order": "desc"}},
			{"social_metrics.activity_score": map[string]interface{}{"order": "desc"}},
		},
	}
}

func (ss *SearchService) buildLocationBasedSuggestionQuery(userID string, limit int) map[string]interface{} {
	// Simplified - would get user's location first
	return map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"status": "active"}},
					{"term": map[string]interface{}{"privacy_settings.searchable": true}},
					{"term": map[string]interface{}{"location.city": "Seoul"}}, // Would be user's actual city
				},
				"must_not": []map[string]interface{}{
					{"term": map[string]interface{}{"user_id": userID}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"social_metrics.activity_score": map[string]interface{}{"order": "desc"}},
		},
	}
}

func (ss *SearchService) buildMixedSuggestionQuery(userID string, limit int) map[string]interface{} {
	return map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"function_score": map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"filter": []map[string]interface{}{
							{"term": map[string]interface{}{"status": "active"}},
							{"term": map[string]interface{}{"privacy_settings.searchable": true}},
						},
						"must_not": []map[string]interface{}{
							{"term": map[string]interface{}{"user_id": userID}},
						},
					},
				},
				"functions": []map[string]interface{}{
					{
						"field_value_factor": map[string]interface{}{
							"field":    "social_metrics.mutual_friend_count",
							"factor":   0.3,
							"modifier": "log1p",
						},
						"weight": 2.0,
					},
					{
						"field_value_factor": map[string]interface{}{
							"field":    "social_metrics.activity_score",
							"factor":   0.5,
							"modifier": "none",
						},
						"weight": 1.5,
					},
				},
				"score_mode": "sum",
				"boost_mode": "multiply",
			},
		},
	}
}

// Helper functions for pagination
func (ss *SearchService) getOffsetFromPageToken(pageToken string) int {
	if pageToken == "" {
		return 0
	}
	
	decoded, err := base64.StdEncoding.DecodeString(pageToken)
	if err != nil {
		return 0
	}
	
	offset, err := strconv.Atoi(string(decoded))
	if err != nil {
		return 0
	}
	
	return offset
}

func (ss *SearchService) createPageToken(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// Conversion functions
func (ss *SearchService) convertSearchHitToResult(hit elasticsearch.SearchHit, includeHighlights bool) (*pb.UserSearchResult, error) {
	// Parse source
	userID, _ := hit.Source["user_id"].(string)
	username, _ := hit.Source["username"].(string)
	displayName, _ := hit.Source["display_name"].(string)
	bio, _ := hit.Source["bio"].(string)
	
	// Parse social metrics
	var socialMetrics *pb.SocialMetrics
	if metrics, ok := hit.Source["social_metrics"].(map[string]interface{}); ok {
		socialMetrics = &pb.SocialMetrics{
			FriendCount:       int32(getIntFromInterface(metrics["friend_count"])),
			MutualFriendCount: int32(getIntFromInterface(metrics["mutual_friend_count"])),
			PopularityScore:   float32(getFloatFromInterface(metrics["popularity_score"])),
			ActivityScore:     float32(getFloatFromInterface(metrics["activity_score"])),
			ResponseRate:      float32(getFloatFromInterface(metrics["response_rate"])),
		}
	}
	
	// Parse timestamps
	var createdAt, updatedAt *time.Time
	if ts, ok := hit.Source["created_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			createdAt = &parsed
		}
	}
	if ts, ok := hit.Source["updated_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			updatedAt = &parsed
		}
	}
	
	user := &pb.User{
		UserId:      userID,
		Username:    username,
		DisplayName: displayName,
	}
	
	// Add timestamps if available
	if createdAt != nil {
		user.CreatedAt = timestamppb.New(*createdAt)
	}
	if updatedAt != nil {
		user.UpdatedAt = timestamppb.New(*updatedAt)
	}
	
	// Build highlight
	var highlight string
	if includeHighlights && hit.Highlight != nil {
		highlights := []string{}
		for field, hlText := range hit.Highlight {
			if len(hlText) > 0 {
				highlights = append(highlights, fmt.Sprintf("%s: %s", field, strings.Join(hlText, " ... ")))
			}
		}
		highlight = strings.Join(highlights, " | ")
	}
	
	result := &pb.UserSearchResult{
		User:              user,
		RelevanceScore:    float32(hit.Score),
		MutualFriendCount: socialMetrics.MutualFriendCount,
		Highlight:         highlight,
		SocialMetrics:     socialMetrics,
	}
	
	return result, nil
}

func (ss *SearchService) convertSearchHitToSuggestion(hit elasticsearch.SearchHit, suggestionType string) (*pb.UserSuggestion, error) {
	result, err := ss.convertSearchHitToResult(hit, false)
	if err != nil {
		return nil, err
	}
	
	// Generate reason based on suggestion type
	reason := ss.generateSuggestionReason(suggestionType, result.SocialMetrics)
	
	suggestion := &pb.UserSuggestion{
		User:             result.User,
		SuggestionScore:  result.RelevanceScore,
		Reason:           reason,
		SharedInterests:  []string{}, // Would be populated in real implementation
		MutualFriendCount: result.MutualFriendCount,
		SocialMetrics:    result.SocialMetrics,
	}
	
	return suggestion, nil
}

func (ss *SearchService) generateSuggestionReason(suggestionType string, metrics *pb.SocialMetrics) string {
	switch suggestionType {
	case "interest_based":
		return "Shares similar interests"
	case "mutual_friends":
		if metrics.MutualFriendCount > 0 {
			return fmt.Sprintf("%d mutual friends", metrics.MutualFriendCount)
		}
		return "Connected through your network"
	case "location_based":
		return "Lives in your area"
	default:
		return "Recommended for you"
	}
}

// Helper functions
func getIntFromInterface(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

func getFloatFromInterface(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}