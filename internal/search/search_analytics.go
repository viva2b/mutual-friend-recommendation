package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"mutual-friend/pkg/elasticsearch"
)

// SearchAnalytics provides comprehensive search analytics and performance monitoring
type SearchAnalytics struct {
	esClient  *elasticsearch.Client
	optimizer *SearchOptimizer
	metrics   *AnalyticsMetrics
}

// AnalyticsMetrics tracks comprehensive search analytics
type AnalyticsMetrics struct {
	QueryPatterns     map[string]*QueryPattern     `json:"query_patterns"`
	UserBehavior      map[string]*UserBehavior     `json:"user_behavior"`
	PerformanceStats  *PerformanceStats            `json:"performance_stats"`
	SearchTrends      []*SearchTrend               `json:"search_trends"`
	PopularityMetrics *PopularityMetrics           `json:"popularity_metrics"`
}

// QueryPattern tracks patterns for specific query types
type QueryPattern struct {
	Query          string            `json:"query"`
	Count          int64             `json:"count"`
	AvgResponseTime time.Duration    `json:"avg_response_time"`
	SuccessRate    float64           `json:"success_rate"`
	ResultCounts   []int             `json:"result_counts"`
	Variations     []string          `json:"variations"`
	PeakHours      map[int]int64     `json:"peak_hours"` // hour -> count
}

// UserBehavior tracks search behavior for individual users
type UserBehavior struct {
	UserID          string            `json:"user_id"`
	SearchCount     int64             `json:"search_count"`
	AvgSessionLength time.Duration    `json:"avg_session_length"`
	TopQueries      []string          `json:"top_queries"`
	SearchFrequency map[string]int64  `json:"search_frequency"` // query -> count
	LastSeen        time.Time         `json:"last_seen"`
}

// PerformanceStats tracks overall system performance
type PerformanceStats struct {
	TotalQueries         int64         `json:"total_queries"`
	AvgResponseTime      time.Duration `json:"avg_response_time"`
	P95ResponseTime      time.Duration `json:"p95_response_time"`
	P99ResponseTime      time.Duration `json:"p99_response_time"`
	ErrorRate            float64       `json:"error_rate"`
	ThroughputPerSecond  float64       `json:"throughput_per_second"`
	CacheHitRate         float64       `json:"cache_hit_rate"`
	IndexOptimization    float64       `json:"index_optimization"`
}

// SearchTrend tracks search trends over time
type SearchTrend struct {
	Timestamp   time.Time `json:"timestamp"`
	Query       string    `json:"query"`
	Count       int64     `json:"count"`
	Category    string    `json:"category"`
	Popularity  float64   `json:"popularity"`
}

// PopularityMetrics tracks what makes searches popular
type PopularityMetrics struct {
	TrendingQueries    []*TrendingQuery    `json:"trending_queries"`
	EmergingPatterns   []*EmergingPattern  `json:"emerging_patterns"`
	SeasonalTrends     []*SeasonalTrend    `json:"seasonal_trends"`
	GeographicTrends   []*GeographicTrend  `json:"geographic_trends"`
}

// TrendingQuery represents a currently trending search query
type TrendingQuery struct {
	Query        string    `json:"query"`
	CurrentCount int64     `json:"current_count"`
	PreviousCount int64    `json:"previous_count"`
	GrowthRate   float64   `json:"growth_rate"`
	Category     string    `json:"category"`
	FirstSeen    time.Time `json:"first_seen"`
}

// EmergingPattern represents newly emerging search patterns
type EmergingPattern struct {
	Pattern     string    `json:"pattern"`
	Confidence  float64   `json:"confidence"`
	Queries     []string  `json:"queries"`
	FirstSeen   time.Time `json:"first_seen"`
	Growth      float64   `json:"growth"`
}

// SeasonalTrend represents seasonal search patterns
type SeasonalTrend struct {
	Pattern       string    `json:"pattern"`
	Season        string    `json:"season"`
	PeakMonth     int       `json:"peak_month"`
	GrowthFactor  float64   `json:"growth_factor"`
	Queries       []string  `json:"queries"`
}

// GeographicTrend represents location-based search trends
type GeographicTrend struct {
	Location      string    `json:"location"`
	TopQueries    []string  `json:"top_queries"`
	UniquePatterns []string `json:"unique_patterns"`
	ActivityLevel float64   `json:"activity_level"`
}

// NewSearchAnalytics creates a new SearchAnalytics instance
func NewSearchAnalytics(esClient *elasticsearch.Client, optimizer *SearchOptimizer) *SearchAnalytics {
	return &SearchAnalytics{
		esClient:  esClient,
		optimizer: optimizer,
		metrics: &AnalyticsMetrics{
			QueryPatterns:     make(map[string]*QueryPattern),
			UserBehavior:      make(map[string]*UserBehavior),
			PerformanceStats:  &PerformanceStats{},
			SearchTrends:      make([]*SearchTrend, 0),
			PopularityMetrics: &PopularityMetrics{},
		},
	}
}

// RecordSearchEvent records a search event for analytics
func (sa *SearchAnalytics) RecordSearchEvent(userID, query string, responseTime time.Duration, resultCount int, success bool) {
	// Update query patterns
	if pattern, exists := sa.metrics.QueryPatterns[query]; exists {
		pattern.Count++
		pattern.AvgResponseTime = (pattern.AvgResponseTime*time.Duration(pattern.Count-1) + responseTime) / time.Duration(pattern.Count)
		pattern.ResultCounts = append(pattern.ResultCounts, resultCount)
		if success {
			pattern.SuccessRate = (pattern.SuccessRate*float64(pattern.Count-1) + 1.0) / float64(pattern.Count)
		} else {
			pattern.SuccessRate = pattern.SuccessRate * float64(pattern.Count-1) / float64(pattern.Count)
		}
		
		// Track peak hours
		hour := time.Now().Hour()
		pattern.PeakHours[hour]++
	} else {
		sa.metrics.QueryPatterns[query] = &QueryPattern{
			Query:           query,
			Count:           1,
			AvgResponseTime: responseTime,
			SuccessRate:     1.0,
			ResultCounts:    []int{resultCount},
			Variations:      []string{query},
			PeakHours:       map[int]int64{time.Now().Hour(): 1},
		}
	}

	// Update user behavior
	if behavior, exists := sa.metrics.UserBehavior[userID]; exists {
		behavior.SearchCount++
		behavior.SearchFrequency[query]++
		behavior.LastSeen = time.Now()
		
		// Update top queries
		sa.updateTopQueries(behavior, query)
	} else {
		sa.metrics.UserBehavior[userID] = &UserBehavior{
			UserID:          userID,
			SearchCount:     1,
			TopQueries:      []string{query},
			SearchFrequency: map[string]int64{query: 1},
			LastSeen:        time.Now(),
		}
	}

	// Record search trend
	sa.metrics.SearchTrends = append(sa.metrics.SearchTrends, &SearchTrend{
		Timestamp:  time.Now(),
		Query:      query,
		Count:      1,
		Category:   sa.categorizeQuery(query),
		Popularity: sa.calculatePopularity(query),
	})

	// Update performance stats
	sa.updatePerformanceStats(responseTime, success)
}

// GenerateAnalyticsReport generates a comprehensive analytics report
func (sa *SearchAnalytics) GenerateAnalyticsReport(ctx context.Context) (*AnalyticsReport, error) {
	log.Println("📊 Generating comprehensive analytics report...")

	// Generate performance insights
	performanceInsights := sa.generatePerformanceInsights()
	
	// Generate user insights
	userInsights := sa.generateUserInsights()
	
	// Generate query insights
	queryInsights := sa.generateQueryInsights()
	
	// Generate trend analysis
	trendAnalysis := sa.generateTrendAnalysis()
	
	// Generate optimization recommendations
	optimizationRecs := sa.generateOptimizationRecommendations()

	report := &AnalyticsReport{
		GeneratedAt:              time.Now(),
		PerformanceInsights:      performanceInsights,
		UserInsights:            userInsights,
		QueryInsights:           queryInsights,
		TrendAnalysis:           trendAnalysis,
		OptimizationRecommendations: optimizationRecs,
		Metrics:                 sa.metrics,
	}

	return report, nil
}

// generatePerformanceInsights analyzes performance patterns
func (sa *SearchAnalytics) generatePerformanceInsights() *PerformanceInsights {
	stats := sa.metrics.PerformanceStats
	
	// Calculate response time percentiles
	responseTimes := sa.collectResponseTimes()
	sort.Slice(responseTimes, func(i, j int) bool {
		return responseTimes[i] < responseTimes[j]
	})
	
	var p95, p99 time.Duration
	if len(responseTimes) > 0 {
		p95Index := int(float64(len(responseTimes)) * 0.95)
		p99Index := int(float64(len(responseTimes)) * 0.99)
		if p95Index < len(responseTimes) {
			p95 = responseTimes[p95Index]
		}
		if p99Index < len(responseTimes) {
			p99 = responseTimes[p99Index]
		}
	}

	return &PerformanceInsights{
		AverageResponseTime: stats.AvgResponseTime,
		P95ResponseTime:    p95,
		P99ResponseTime:    p99,
		ErrorRate:          stats.ErrorRate,
		ThroughputQPS:      stats.ThroughputPerSecond,
		PerformanceGrade:   sa.calculatePerformanceGrade(stats),
		Bottlenecks:        sa.identifyBottlenecks(),
		Recommendations:    sa.generatePerformanceRecommendations(stats),
	}
}

// generateUserInsights analyzes user behavior patterns
func (sa *SearchAnalytics) generateUserInsights() *UserInsights {
	totalUsers := len(sa.metrics.UserBehavior)
	totalSearches := int64(0)
	activeUsers := 0
	powerUsers := 0

	for _, behavior := range sa.metrics.UserBehavior {
		totalSearches += behavior.SearchCount
		if time.Since(behavior.LastSeen) < 24*time.Hour {
			activeUsers++
		}
		if behavior.SearchCount > 10 {
			powerUsers++
		}
	}

	avgSearchesPerUser := float64(0)
	if totalUsers > 0 {
		avgSearchesPerUser = float64(totalSearches) / float64(totalUsers)
	}

	return &UserInsights{
		TotalUsers:         totalUsers,
		ActiveUsers:        activeUsers,
		PowerUsers:         powerUsers,
		TotalSearches:      totalSearches,
		AvgSearchesPerUser: avgSearchesPerUser,
		UserSegments:       sa.segmentUsers(),
		BehaviorPatterns:   sa.identifyBehaviorPatterns(),
	}
}

// generateQueryInsights analyzes query patterns
func (sa *SearchAnalytics) generateQueryInsights() *QueryInsights {
	totalQueries := len(sa.metrics.QueryPatterns)
	topQueries := sa.getTopQueries(10)
	failingQueries := sa.getFailingQueries()
	
	return &QueryInsights{
		TotalUniqueQueries: totalQueries,
		TopQueries:        topQueries,
		FailingQueries:    failingQueries,
		QueryCategories:   sa.categorizeAllQueries(),
		SearchComplexity:  sa.analyzeSearchComplexity(),
	}
}

// generateTrendAnalysis analyzes search trends
func (sa *SearchAnalytics) generateTrendAnalysis() *TrendAnalysis {
	return &TrendAnalysis{
		TrendingQueries:   sa.identifyTrendingQueries(),
		EmergingPatterns:  sa.identifyEmergingPatterns(),
		SeasonalTrends:    sa.identifySeasonalTrends(),
		GeographicTrends:  sa.identifyGeographicTrends(),
		PredictedTrends:   sa.predictFutureTrends(),
	}
}

// Helper methods
func (sa *SearchAnalytics) updateTopQueries(behavior *UserBehavior, query string) {
	// Add query if not in top queries, or update existing
	found := false
	for i, q := range behavior.TopQueries {
		if q == query {
			found = true
			break
		}
		// If this query is more frequent, insert it here
		if sa.metrics.UserBehavior[behavior.UserID].SearchFrequency[query] > 
		   sa.metrics.UserBehavior[behavior.UserID].SearchFrequency[q] {
			// Insert at position i
			behavior.TopQueries = append(behavior.TopQueries[:i+1], behavior.TopQueries[i:]...)
			behavior.TopQueries[i] = query
			found = true
			break
		}
	}
	
	if !found && len(behavior.TopQueries) < 10 {
		behavior.TopQueries = append(behavior.TopQueries, query)
	}
}

func (sa *SearchAnalytics) categorizeQuery(query string) string {
	// Simple categorization logic
	query = strings.ToLower(query)
	if strings.Contains(query, "developer") || strings.Contains(query, "engineer") || strings.Contains(query, "programmer") {
		return "tech"
	}
	if strings.Contains(query, "designer") || strings.Contains(query, "artist") {
		return "design"
	}
	if strings.Contains(query, "manager") || strings.Contains(query, "lead") {
		return "management"
	}
	return "general"
}

func (sa *SearchAnalytics) calculatePopularity(query string) float64 {
	if pattern, exists := sa.metrics.QueryPatterns[query]; exists {
		// Simple popularity calculation based on frequency and recency
		recentBoost := 1.0
		if time.Now().Hour() >= 9 && time.Now().Hour() <= 17 {
			recentBoost = 1.5 // Boost during business hours
		}
		return float64(pattern.Count) * recentBoost
	}
	return 1.0
}

func (sa *SearchAnalytics) updatePerformanceStats(responseTime time.Duration, success bool) {
	stats := sa.metrics.PerformanceStats
	stats.TotalQueries++
	
	if stats.TotalQueries == 1 {
		stats.AvgResponseTime = responseTime
	} else {
		stats.AvgResponseTime = (stats.AvgResponseTime*time.Duration(stats.TotalQueries-1) + responseTime) / time.Duration(stats.TotalQueries)
	}
	
	if !success {
		stats.ErrorRate = (stats.ErrorRate*float64(stats.TotalQueries-1) + 1.0) / float64(stats.TotalQueries)
	} else {
		stats.ErrorRate = stats.ErrorRate * float64(stats.TotalQueries-1) / float64(stats.TotalQueries)
	}
}

func (sa *SearchAnalytics) collectResponseTimes() []time.Duration {
	var times []time.Duration
	for _, pattern := range sa.metrics.QueryPatterns {
		for i := int64(0); i < pattern.Count; i++ {
			times = append(times, pattern.AvgResponseTime)
		}
	}
	return times
}

func (sa *SearchAnalytics) calculatePerformanceGrade(stats *PerformanceStats) string {
	score := 100.0
	
	// Deduct points for slow response times
	if stats.AvgResponseTime > 200*time.Millisecond {
		score -= 20
	}
	if stats.AvgResponseTime > 500*time.Millisecond {
		score -= 30
	}
	
	// Deduct points for high error rate
	if stats.ErrorRate > 0.01 {
		score -= 15
	}
	if stats.ErrorRate > 0.05 {
		score -= 25
	}
	
	if score >= 90 {
		return "A"
	} else if score >= 80 {
		return "B"
	} else if score >= 70 {
		return "C"
	} else if score >= 60 {
		return "D"
	}
	return "F"
}

// Data structures for analytics report
type AnalyticsReport struct {
	GeneratedAt                 time.Time                    `json:"generated_at"`
	PerformanceInsights         *PerformanceInsights         `json:"performance_insights"`
	UserInsights               *UserInsights                `json:"user_insights"`
	QueryInsights              *QueryInsights               `json:"query_insights"`
	TrendAnalysis              *TrendAnalysis               `json:"trend_analysis"`
	OptimizationRecommendations []OptimizationRecommendation `json:"optimization_recommendations"`
	Metrics                    *AnalyticsMetrics            `json:"metrics"`
}

type PerformanceInsights struct {
	AverageResponseTime time.Duration `json:"average_response_time"`
	P95ResponseTime     time.Duration `json:"p95_response_time"`
	P99ResponseTime     time.Duration `json:"p99_response_time"`
	ErrorRate          float64       `json:"error_rate"`
	ThroughputQPS      float64       `json:"throughput_qps"`
	PerformanceGrade   string        `json:"performance_grade"`
	Bottlenecks        []string      `json:"bottlenecks"`
	Recommendations    []string      `json:"recommendations"`
}

type UserInsights struct {
	TotalUsers         int                    `json:"total_users"`
	ActiveUsers        int                    `json:"active_users"`
	PowerUsers         int                    `json:"power_users"`
	TotalSearches      int64                  `json:"total_searches"`
	AvgSearchesPerUser float64                `json:"avg_searches_per_user"`
	UserSegments       map[string]int         `json:"user_segments"`
	BehaviorPatterns   []string              `json:"behavior_patterns"`
}

type QueryInsights struct {
	TotalUniqueQueries int                    `json:"total_unique_queries"`
	TopQueries         []*QueryPattern        `json:"top_queries"`
	FailingQueries     []*QueryPattern        `json:"failing_queries"`
	QueryCategories    map[string]int         `json:"query_categories"`
	SearchComplexity   map[string]float64     `json:"search_complexity"`
}

type TrendAnalysis struct {
	TrendingQueries  []*TrendingQuery    `json:"trending_queries"`
	EmergingPatterns []*EmergingPattern  `json:"emerging_patterns"`
	SeasonalTrends   []*SeasonalTrend    `json:"seasonal_trends"`
	GeographicTrends []*GeographicTrend  `json:"geographic_trends"`
	PredictedTrends  []string           `json:"predicted_trends"`
}

type OptimizationRecommendation struct {
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"`
}

// Placeholder implementations for complex analysis methods
func (sa *SearchAnalytics) identifyBottlenecks() []string {
	return []string{"Query parsing", "Index lookup", "Result ranking"}
}

func (sa *SearchAnalytics) generatePerformanceRecommendations(stats *PerformanceStats) []string {
	recommendations := []string{}
	if stats.AvgResponseTime > 200*time.Millisecond {
		recommendations = append(recommendations, "Consider adding more Elasticsearch replicas")
	}
	if stats.ErrorRate > 0.05 {
		recommendations = append(recommendations, "Investigate and fix query parsing errors")
	}
	return recommendations
}

func (sa *SearchAnalytics) segmentUsers() map[string]int {
	return map[string]int{
		"casual":     0,
		"regular":    0,
		"power":      0,
		"enterprise": 0,
	}
}

func (sa *SearchAnalytics) identifyBehaviorPatterns() []string {
	return []string{"Peak usage during business hours", "Higher search frequency on weekdays"}
}

func (sa *SearchAnalytics) getTopQueries(limit int) []*QueryPattern {
	patterns := make([]*QueryPattern, 0, len(sa.metrics.QueryPatterns))
	for _, pattern := range sa.metrics.QueryPatterns {
		patterns = append(patterns, pattern)
	}
	
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})
	
	if len(patterns) > limit {
		patterns = patterns[:limit]
	}
	
	return patterns
}

func (sa *SearchAnalytics) getFailingQueries() []*QueryPattern {
	failing := make([]*QueryPattern, 0)
	for _, pattern := range sa.metrics.QueryPatterns {
		if pattern.SuccessRate < 0.8 {
			failing = append(failing, pattern)
		}
	}
	return failing
}

func (sa *SearchAnalytics) categorizeAllQueries() map[string]int {
	categories := map[string]int{}
	for _, pattern := range sa.metrics.QueryPatterns {
		category := sa.categorizeQuery(pattern.Query)
		categories[category] += int(pattern.Count)
	}
	return categories
}

func (sa *SearchAnalytics) analyzeSearchComplexity() map[string]float64 {
	return map[string]float64{
		"simple":    0.3,
		"moderate":  0.5,
		"complex":   0.2,
	}
}

func (sa *SearchAnalytics) identifyTrendingQueries() []*TrendingQuery {
	return []*TrendingQuery{}
}

func (sa *SearchAnalytics) identifyEmergingPatterns() []*EmergingPattern {
	return []*EmergingPattern{}
}

func (sa *SearchAnalytics) identifySeasonalTrends() []*SeasonalTrend {
	return []*SeasonalTrend{}
}

func (sa *SearchAnalytics) identifyGeographicTrends() []*GeographicTrend {
	return []*GeographicTrend{}
}

func (sa *SearchAnalytics) predictFutureTrends() []string {
	return []string{"Increased mobile search", "Voice search adoption", "AI-assisted search"}
}

func (sa *SearchAnalytics) generateOptimizationRecommendations() []OptimizationRecommendation {
	return []OptimizationRecommendation{
		{
			Type:        "Performance",
			Priority:    "High",
			Title:       "Implement Query Caching",
			Description: "Add Redis caching for frequently executed queries",
			Impact:      "30-50% response time improvement",
			Effort:      "Medium",
		},
		{
			Type:        "User Experience", 
			Priority:    "Medium",
			Title:       "Add Autocomplete",
			Description: "Implement search suggestions based on popular queries",
			Impact:      "Improved user engagement",
			Effort:      "High",
		},
	}
}

// Export analytics data to JSON
func (sa *SearchAnalytics) ExportToJSON() ([]byte, error) {
	return json.MarshalIndent(sa.metrics, "", "  ")
}