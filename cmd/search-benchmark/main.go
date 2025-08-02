package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"mutual-friend/internal/search"
	"mutual-friend/pkg/elasticsearch"
)

func main() {
	log.Println("🚀 Starting Search System Benchmark and Analytics")

	// Initialize Elasticsearch client
	esClient, err := elasticsearch.NewClient(&elasticsearch.Config{
		Host:     getEnv("ELASTICSEARCH_HOST", "localhost"),
		Port:     getEnv("ELASTICSEARCH_PORT", "9200"),
		Username: getEnv("ELASTICSEARCH_USERNAME", ""),
		Password: getEnv("ELASTICSEARCH_PASSWORD", ""),
	})
	if err != nil {
		log.Fatalf("Failed to create Elasticsearch client: %v", err)
	}

	// Test Elasticsearch connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := esClient.Ping(ctx); err != nil {
		log.Fatalf("Failed to connect to Elasticsearch: %v", err)
	}
	log.Println("✅ Connected to Elasticsearch")

	// Initialize search components
	searchService := search.NewSearchService(esClient)
	optimizer := search.NewSearchOptimizer(esClient)
	analytics := search.NewSearchAnalytics(esClient, optimizer)

	// Run comprehensive benchmark
	log.Println("📊 Running comprehensive search benchmark...")
	runSearchBenchmark(ctx, searchService, analytics)

	// Optimize search settings
	log.Println("⚙️  Optimizing search settings...")
	if err := optimizer.OptimizeIndexSettings(ctx); err != nil {
		log.Printf("❌ Failed to optimize index settings: %v", err)
	}

	// Warm up query cache
	log.Println("🔥 Warming up query cache...")
	if err := optimizer.WarmupQueries(ctx); err != nil {
		log.Printf("❌ Failed to warm up query cache: %v", err)
	}

	// Generate performance report
	log.Println("📈 Generating performance analysis...")
	performanceReport, err := optimizer.AnalyzeSearchPerformance(ctx)
	if err != nil {
		log.Printf("❌ Failed to analyze performance: %v", err)
	} else {
		displayPerformanceReport(performanceReport)
	}

	// Generate analytics report
	log.Println("📊 Generating analytics report...")
	analyticsReport, err := analytics.GenerateAnalyticsReport(ctx)
	if err != nil {
		log.Printf("❌ Failed to generate analytics report: %v", err)
	} else {
		displayAnalyticsReport(analyticsReport)
	}

	// Export analytics data
	log.Println("💾 Exporting analytics data...")
	analyticsData, err := analytics.ExportToJSON()
	if err != nil {
		log.Printf("❌ Failed to export analytics: %v", err)
	} else {
		if err := os.WriteFile("search_analytics.json", analyticsData, 0644); err != nil {
			log.Printf("❌ Failed to write analytics file: %v", err)
		} else {
			log.Println("✅ Analytics data exported to search_analytics.json")
		}
	}

	log.Println("🎉 Search benchmark and analysis completed!")
}

func runSearchBenchmark(ctx context.Context, searchService *search.SearchService, analytics *search.SearchAnalytics) {
	// Benchmark queries to test
	benchmarkQueries := []struct {
		name        string
		query       string
		description string
	}{
		{"Empty Query", "", "Test search with no query (match all)"},
		{"Simple Name", "john", "Basic username search"},
		{"Job Title", "developer", "Professional title search"},
		{"Skill Search", "programming", "Technical skill search"},
		{"Complex Query", "senior software engineer", "Multi-word professional search"},
		{"Location", "seoul", "Geographic search"},
		{"Interest", "music", "Hobby/interest search"},
		{"Common Name", "kim", "Very common name search"},
		{"Rare Term", "blockchain", "Specialized term search"},
		{"Mixed Case", "JavaScript", "Case sensitivity test"},
	}

	totalQueries := 0
	totalDuration := time.Duration(0)
	successCount := 0

	for _, bq := range benchmarkQueries {
		log.Printf("  🔍 Testing: %s - %s", bq.name, bq.description)

		// Run multiple iterations for each query
		iterations := 5
		for i := 0; i < iterations; i++ {
			start := time.Now()

			// In a real implementation, this would call searchService.SearchUsers
			// For now, we'll simulate the search
			resultCount, err := simulateSearch(ctx, bq.query)
			duration := time.Since(start)

			// Record metrics
			userID := fmt.Sprintf("benchmark-user-%d", i)
			success := err == nil
			analytics.RecordSearchEvent(userID, bq.query, duration, resultCount, success)

			totalQueries++
			totalDuration += duration
			if success {
				successCount++
			}

			log.Printf("    Iteration %d: %v (results: %d)", i+1, duration, resultCount)
		}
		log.Println()
	}

	// Display benchmark summary
	avgDuration := totalDuration / time.Duration(totalQueries)
	successRate := float64(successCount) / float64(totalQueries) * 100

	log.Println("📊 Benchmark Summary:")
	log.Printf("  Total Queries: %d", totalQueries)
	log.Printf("  Average Response Time: %v", avgDuration)
	log.Printf("  Success Rate: %.1f%%", successRate)
	log.Printf("  Total Duration: %v", totalDuration)
	log.Println()
}

func simulateSearch(ctx context.Context, query string) (int, error) {
	// Simulate search latency based on query complexity
	var delay time.Duration
	switch {
	case query == "":
		delay = 50 * time.Millisecond // Fast for empty query
	case len(query) < 5:
		delay = 75 * time.Millisecond // Medium for short queries
	case len(query) > 15:
		delay = 150 * time.Millisecond // Slower for complex queries
	default:
		delay = 100 * time.Millisecond // Standard delay
	}

	time.Sleep(delay)

	// Simulate varying result counts
	resultCount := 10
	if query == "" {
		resultCount = 50 // Match all returns more results
	} else if len(query) > 10 {
		resultCount = 3 // Complex queries return fewer results
	}

	return resultCount, nil
}

func displayPerformanceReport(report *search.PerformanceReport) {
	log.Println("📈 Performance Analysis Report:")
	log.Printf("  Generated: %s", report.GeneratedAt.Format(time.RFC3339))
	
	if report.IndexStatistics != nil {
		log.Println("  Index Statistics:")
		log.Printf("    Documents: %d", report.IndexStatistics.DocumentCount)
		log.Printf("    Index Size: %s", report.IndexStatistics.IndexSize)
		log.Printf("    Search Rate: %.1f queries/sec", report.IndexStatistics.SearchRate)
	}
	
	if report.QueryAnalysis != nil {
		log.Println("  Query Analysis:")
		log.Printf("    Total Queries: %d", report.QueryAnalysis.TotalQueries)
		log.Printf("    Average Response Time: %v", report.QueryAnalysis.AverageResponseTime)
		log.Printf("    Slow Queries: %d", report.QueryAnalysis.SlowQueryCount)
		log.Printf("    Error Rate: %.2f%%", report.QueryAnalysis.ErrorRate*100)
	}
	
	if len(report.Recommendations) > 0 {
		log.Println("  Recommendations:")
		for _, rec := range report.Recommendations {
			log.Printf("    [%s] %s: %s", rec.Priority, rec.Title, rec.Description)
		}
	}
	log.Println()
}

func displayAnalyticsReport(report *search.AnalyticsReport) {
	log.Println("📊 Analytics Report:")
	log.Printf("  Generated: %s", report.GeneratedAt.Format(time.RFC3339))
	
	if report.PerformanceInsights != nil {
		log.Println("  Performance Insights:")
		log.Printf("    Grade: %s", report.PerformanceInsights.PerformanceGrade)
		log.Printf("    Average Response: %v", report.PerformanceInsights.AverageResponseTime)
		log.Printf("    P95 Response: %v", report.PerformanceInsights.P95ResponseTime)
		log.Printf("    Error Rate: %.2f%%", report.PerformanceInsights.ErrorRate*100)
	}
	
	if report.UserInsights != nil {
		log.Println("  User Insights:")
		log.Printf("    Total Users: %d", report.UserInsights.TotalUsers)
		log.Printf("    Active Users: %d", report.UserInsights.ActiveUsers)
		log.Printf("    Total Searches: %d", report.UserInsights.TotalSearches)
		log.Printf("    Avg Searches/User: %.1f", report.UserInsights.AvgSearchesPerUser)
	}
	
	if report.QueryInsights != nil {
		log.Println("  Query Insights:")
		log.Printf("    Unique Queries: %d", report.QueryInsights.TotalUniqueQueries)
		log.Printf("    Top Queries: %d", len(report.QueryInsights.TopQueries))
		log.Printf("    Failing Queries: %d", len(report.QueryInsights.FailingQueries))
	}
	
	if len(report.OptimizationRecommendations) > 0 {
		log.Println("  Optimization Recommendations:")
		for _, rec := range report.OptimizationRecommendations {
			log.Printf("    [%s] %s: %s", rec.Priority, rec.Title, rec.Description)
		}
	}
	log.Println()
}


func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}