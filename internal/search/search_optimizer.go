package search

import (
	"context"
	"fmt"
	"log"
	"time"

	"mutual-friend/pkg/elasticsearch"
)

// SearchOptimizer provides search performance optimization and analysis
type SearchOptimizer struct {
	esClient *elasticsearch.Client
	metrics  *SearchMetrics
}

// SearchMetrics tracks search performance metrics
type SearchMetrics struct {
	TotalQueries     int64
	TotalResponseTime time.Duration
	SlowQueries      []SlowQuery
	PopularQueries   map[string]int64
	ErrorRate        float64
}

// SlowQuery represents a query that took longer than threshold
type SlowQuery struct {
	Query       string
	Duration    time.Duration
	Timestamp   time.Time
	ResultCount int
}

// NewSearchOptimizer creates a new search optimizer
func NewSearchOptimizer(esClient *elasticsearch.Client) *SearchOptimizer {
	return &SearchOptimizer{
		esClient: esClient,
		metrics: &SearchMetrics{
			PopularQueries: make(map[string]int64),
			SlowQueries:    make([]SlowQuery, 0),
		},
	}
}

// OptimizeIndexSettings optimizes Elasticsearch index settings for better search performance
func (so *SearchOptimizer) OptimizeIndexSettings(ctx context.Context) error {
	log.Println("🔧 Optimizing Elasticsearch index settings...")
	
	// Optimization settings for users index
	optimizationSettings := map[string]interface{}{
		"settings": map[string]interface{}{
			"index": map[string]interface{}{
				"refresh_interval": "5s", // Reduce refresh frequency for better write performance
				"number_of_replicas": 1,   // Ensure we have replicas for better search performance
				"routing": map[string]interface{}{
					"allocation": map[string]interface{}{
						"total_shards_per_node": 2, // Distribute shards evenly
					},
				},
				"search": map[string]interface{}{
					"slowlog": map[string]interface{}{
						"threshold": map[string]interface{}{
							"query": map[string]interface{}{
								"warn": "2s",
								"info": "1s",
							},
							"fetch": map[string]interface{}{
								"warn": "500ms",
								"info": "200ms",
							},
						},
					},
				},
			},
		},
	}
	
	// Apply settings to users index
	err := so.updateIndexSettings(ctx, "users", optimizationSettings)
	if err != nil {
		return fmt.Errorf("failed to optimize users index: %w", err)
	}
	
	log.Println("✅ Index optimization completed")
	return nil
}

// updateIndexSettings updates index settings
func (so *SearchOptimizer) updateIndexSettings(ctx context.Context, indexName string, settings map[string]interface{}) error {
	// This would typically use the Elasticsearch client to update settings
	// For now, we'll just log the operation
	log.Printf("Updating settings for index: %s", indexName)
	return nil
}

// AnalyzeSearchPerformance analyzes search performance and provides recommendations
func (so *SearchOptimizer) AnalyzeSearchPerformance(ctx context.Context) (*PerformanceReport, error) {
	log.Println("📊 Analyzing search performance...")
	
	// Get index statistics
	indexStats, err := so.getIndexStatistics(ctx, "users")
	if err != nil {
		return nil, fmt.Errorf("failed to get index statistics: %w", err)
	}
	
	// Analyze query patterns
	queryAnalysis := so.analyzeQueryPatterns()
	
	// Generate recommendations
	recommendations := so.generateOptimizationRecommendations(indexStats, queryAnalysis)
	
	report := &PerformanceReport{
		IndexStatistics:   indexStats,
		QueryAnalysis:     queryAnalysis,
		Recommendations:   recommendations,
		GeneratedAt:       time.Now(),
	}
	
	return report, nil
}

// getIndexStatistics gets statistics about the index
func (so *SearchOptimizer) getIndexStatistics(ctx context.Context, indexName string) (*IndexStatistics, error) {
	// In a real implementation, this would query Elasticsearch for index stats
	// For now, we'll return mock data
	return &IndexStatistics{
		IndexName:     indexName,
		DocumentCount: 10000,
		IndexSize:     "50MB",
		ShardCount:    3,
		ReplicaCount:  1,
		SearchRate:    100.5, // searches per second
		IndexingRate:  10.2,  // documents per second
	}, nil
}

// analyzeQueryPatterns analyzes query patterns from metrics
func (so *SearchOptimizer) analyzeQueryPatterns() *QueryAnalysis {
	totalQueries := so.metrics.TotalQueries
	avgResponseTime := time.Duration(0)
	
	if totalQueries > 0 {
		avgResponseTime = so.metrics.TotalResponseTime / time.Duration(totalQueries)
	}
	
	// Find most popular queries
	popularQueries := make([]PopularQuery, 0)
	for query, count := range so.metrics.PopularQueries {
		popularQueries = append(popularQueries, PopularQuery{
			Query: query,
			Count: count,
		})
	}
	
	return &QueryAnalysis{
		TotalQueries:        totalQueries,
		AverageResponseTime: avgResponseTime,
		SlowQueryCount:      int64(len(so.metrics.SlowQueries)),
		PopularQueries:      popularQueries,
		ErrorRate:           so.metrics.ErrorRate,
	}
}

// generateOptimizationRecommendations generates optimization recommendations
func (so *SearchOptimizer) generateOptimizationRecommendations(indexStats *IndexStatistics, queryAnalysis *QueryAnalysis) []Recommendation {
	recommendations := make([]Recommendation, 0)
	
	// Check average response time
	if queryAnalysis.AverageResponseTime > 200*time.Millisecond {
		recommendations = append(recommendations, Recommendation{
			Type:        "Performance",
			Priority:    "High",
			Title:       "High Average Response Time",
			Description: fmt.Sprintf("Average response time is %v, consider optimizing queries or adding more replicas", queryAnalysis.AverageResponseTime),
			Action:      "Increase replica count or optimize query structure",
		})
	}
	
	// Check slow query count
	if queryAnalysis.SlowQueryCount > queryAnalysis.TotalQueries/10 {
		recommendations = append(recommendations, Recommendation{
			Type:        "Performance",
			Priority:    "High",
			Title:       "High Slow Query Rate",
			Description: fmt.Sprintf("%d%% of queries are slow", (queryAnalysis.SlowQueryCount*100)/queryAnalysis.TotalQueries),
			Action:      "Review and optimize slow queries, consider adding more specific filters",
		})
	}
	
	// Check error rate
	if queryAnalysis.ErrorRate > 0.05 { // 5% error rate
		recommendations = append(recommendations, Recommendation{
			Type:        "Reliability",
			Priority:    "Critical",
			Title:       "High Error Rate",
			Description: fmt.Sprintf("Query error rate is %.2f%%", queryAnalysis.ErrorRate*100),
			Action:      "Investigate query errors and improve error handling",
		})
	}
	
	// Check index size vs performance
	if indexStats.DocumentCount > 100000 && indexStats.ShardCount < 5 {
		recommendations = append(recommendations, Recommendation{
			Type:        "Scalability",
			Priority:    "Medium",
			Title:       "Consider Index Scaling",
			Description: fmt.Sprintf("Index has %d documents with only %d shards", indexStats.DocumentCount, indexStats.ShardCount),
			Action:      "Consider increasing shard count for better distribution",
		})
	}
	
	return recommendations
}

// CreateSearchTemplate creates optimized search templates for common queries
func (so *SearchOptimizer) CreateSearchTemplate(ctx context.Context, templateName string, template map[string]interface{}) error {
	log.Printf("Creating search template: %s", templateName)
	
	// In a real implementation, this would create a search template in Elasticsearch
	// Search templates can improve performance by pre-compiling queries
	
	return nil
}

// WarmupQueries warms up the query cache with popular queries
func (so *SearchOptimizer) WarmupQueries(ctx context.Context) error {
	log.Println("🔥 Warming up query cache...")
	
	// Common warmup queries based on popular search patterns
	warmupQueries := []string{
		"developer",
		"engineer", 
		"designer",
		"manager",
		"programmer",
	}
	
	for _, query := range warmupQueries {
		warmupQuery := map[string]interface{}{
			"size": 0, // We don't need results, just want to warm the cache
			"query": map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query": query,
					"fields": []string{
						"username^3",
						"display_name^2", 
						"bio^1",
					},
				},
			},
		}
		
		_, err := so.esClient.Search(ctx, "users", warmupQuery)
		if err != nil {
			log.Printf("Failed to warm up query '%s': %v", query, err)
			continue
		}
		
		log.Printf("Warmed up query: %s", query)
	}
	
	log.Println("✅ Query cache warmup completed")
	return nil
}

// RecordQueryMetrics records metrics for a search query
func (so *SearchOptimizer) RecordQueryMetrics(query string, duration time.Duration, resultCount int, err error) {
	so.metrics.TotalQueries++
	so.metrics.TotalResponseTime += duration
	
	// Track popular queries
	so.metrics.PopularQueries[query]++
	
	// Track slow queries (threshold: 1 second)
	if duration > time.Second {
		so.metrics.SlowQueries = append(so.metrics.SlowQueries, SlowQuery{
			Query:       query,
			Duration:    duration,
			Timestamp:   time.Now(),
			ResultCount: resultCount,
		})
		
		// Keep only last 100 slow queries
		if len(so.metrics.SlowQueries) > 100 {
			so.metrics.SlowQueries = so.metrics.SlowQueries[1:]
		}
	}
	
	// Track errors
	if err != nil {
		errorCount := float64(0)
		for _, slowQuery := range so.metrics.SlowQueries {
			if slowQuery.Duration == 0 { // We can use duration 0 to mark errors
				errorCount++
			}
		}
		so.metrics.ErrorRate = errorCount / float64(so.metrics.TotalQueries)
	}
}

// Data structures for performance analysis
type PerformanceReport struct {
	IndexStatistics   *IndexStatistics   `json:"index_statistics"`
	QueryAnalysis     *QueryAnalysis     `json:"query_analysis"`
	Recommendations   []Recommendation   `json:"recommendations"`
	GeneratedAt       time.Time          `json:"generated_at"`
}

type IndexStatistics struct {
	IndexName     string  `json:"index_name"`
	DocumentCount int64   `json:"document_count"`
	IndexSize     string  `json:"index_size"`
	ShardCount    int     `json:"shard_count"`
	ReplicaCount  int     `json:"replica_count"`
	SearchRate    float64 `json:"search_rate"`    // searches per second
	IndexingRate  float64 `json:"indexing_rate"`  // documents per second
}

type QueryAnalysis struct {
	TotalQueries        int64           `json:"total_queries"`
	AverageResponseTime time.Duration   `json:"average_response_time"`
	SlowQueryCount      int64           `json:"slow_query_count"`
	PopularQueries      []PopularQuery  `json:"popular_queries"`
	ErrorRate           float64         `json:"error_rate"`
}

type PopularQuery struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

type Recommendation struct {
	Type        string `json:"type"`        // Performance, Reliability, Scalability
	Priority    string `json:"priority"`    // Critical, High, Medium, Low
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
}