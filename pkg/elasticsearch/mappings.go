package elasticsearch

import (
	"context"
	"log"
)

// IndexMappings contains all the index mappings
type IndexMappings struct {
	client *Client
}

// NewIndexMappings creates a new IndexMappings instance
func NewIndexMappings(client *Client) *IndexMappings {
	return &IndexMappings{client: client}
}

// CreateAllIndexes creates all required indexes
func (im *IndexMappings) CreateAllIndexes(ctx context.Context) error {
	indexes := map[string]map[string]interface{}{
		"users":             im.GetUsersMapping(),
		"social-graph":      im.GetSocialGraphMapping(),
		"recommendations":   im.GetRecommendationsMapping(),
	}

	for indexName, mapping := range indexes {
		if err := im.client.CreateIndex(ctx, indexName, mapping); err != nil {
			return err
		}
		log.Printf("Created index: %s", indexName)
	}

	return nil
}

// GetUsersMapping returns the mapping for the users index
func (im *IndexMappings) GetUsersMapping() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   3,
			"number_of_replicas": 1,
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"username_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "keyword",
						"filter":    []string{"lowercase", "trim"},
					},
					"name_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "standard",
						"filter":    []string{"lowercase", "trim", "stop"},
					},
					"search_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "standard",
						"filter":    []string{"lowercase", "trim", "stop", "snowball"},
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"user_id": map[string]interface{}{
					"type":  "keyword",
					"index": true,
				},
				"username": map[string]interface{}{
					"type":     "text",
					"analyzer": "username_analyzer",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
						"suggest": map[string]interface{}{
							"type":     "completion",
							"analyzer": "username_analyzer",
						},
					},
				},
				"display_name": map[string]interface{}{
					"type":     "text",
					"analyzer": "name_analyzer",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
						"suggest": map[string]interface{}{
							"type":     "completion",
							"analyzer": "name_analyzer",
						},
					},
				},
				"email": map[string]interface{}{
					"type":  "keyword",
					"index": false,
				},
				"bio": map[string]interface{}{
					"type":     "text",
					"analyzer": "search_analyzer",
				},
				"location": map[string]interface{}{
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type": "keyword",
						},
						"state": map[string]interface{}{
							"type": "keyword",
						},
						"country": map[string]interface{}{
							"type": "keyword",
						},
						"coordinates": map[string]interface{}{
							"type": "geo_point",
						},
					},
				},
				"interests": map[string]interface{}{
					"type": "keyword",
					"fields": map[string]interface{}{
						"text": map[string]interface{}{
							"type":     "text",
							"analyzer": "search_analyzer",
						},
					},
				},
				"social_metrics": map[string]interface{}{
					"properties": map[string]interface{}{
						"friend_count": map[string]interface{}{
							"type": "integer",
						},
						"mutual_friend_count": map[string]interface{}{
							"type": "integer",
						},
						"popularity_score": map[string]interface{}{
							"type": "float",
						},
						"activity_score": map[string]interface{}{
							"type": "float",
						},
						"response_rate": map[string]interface{}{
							"type": "float",
						},
					},
				},
				"privacy_settings": map[string]interface{}{
					"properties": map[string]interface{}{
						"searchable": map[string]interface{}{
							"type": "boolean",
						},
						"show_mutual_friends": map[string]interface{}{
							"type": "boolean",
						},
						"location_visible": map[string]interface{}{
							"type": "boolean",
						},
					},
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
				"last_active": map[string]interface{}{
					"type": "date",
				},
				"status": map[string]interface{}{
					"type":  "keyword",
					"index": true,
				},
			},
		},
	}
}

// GetSocialGraphMapping returns the mapping for the social-graph index
func (im *IndexMappings) GetSocialGraphMapping() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   5,
			"number_of_replicas": 1,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"relationship_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_a": map[string]interface{}{
					"type": "keyword",
				},
				"user_b": map[string]interface{}{
					"type": "keyword",
				},
				"relationship_type": map[string]interface{}{
					"type": "keyword",
				},
				"strength_score": map[string]interface{}{
					"type": "float",
				},
				"mutual_connections": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"user_id": map[string]interface{}{
							"type": "keyword",
						},
						"connection_strength": map[string]interface{}{
							"type": "float",
						},
						"relationship_type": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"interaction_history": map[string]interface{}{
					"properties": map[string]interface{}{
						"total_interactions": map[string]interface{}{
							"type": "integer",
						},
						"last_interaction": map[string]interface{}{
							"type": "date",
						},
						"interaction_frequency": map[string]interface{}{
							"type": "float",
						},
						"interaction_types": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"graph_metrics": map[string]interface{}{
					"properties": map[string]interface{}{
						"centrality_score": map[string]interface{}{
							"type": "float",
						},
						"clustering_coefficient": map[string]interface{}{
							"type": "float",
						},
						"shortest_path_length": map[string]interface{}{
							"type": "integer",
						},
						"common_neighbors_count": map[string]interface{}{
							"type": "integer",
						},
					},
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetRecommendationsMapping returns the mapping for the recommendations index
func (im *IndexMappings) GetRecommendationsMapping() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   3,
			"number_of_replicas": 1,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"recommended_user_id": map[string]interface{}{
					"type": "keyword",
				},
				"recommendation_score": map[string]interface{}{
					"type": "float",
				},
				"mutual_friend_count": map[string]interface{}{
					"type": "integer",
				},
				"mutual_friends": map[string]interface{}{
					"type": "keyword",
				},
				"reason_code": map[string]interface{}{
					"type": "keyword",
				},
				"confidence_score": map[string]interface{}{
					"type": "float",
				},
				"algorithm_version": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
				"expires_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}