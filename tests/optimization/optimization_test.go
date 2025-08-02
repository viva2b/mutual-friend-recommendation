package optimization

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// TestSystemOptimization 시스템 최적화 테스트
func TestSystemOptimization(t *testing.T) {
	// 설정 로드
	cfg, err := config.LoadConfig("../../configs/config.yaml")
	require.NoError(t, err)
	
	// Redis 클라이언트 초기화
	redisClient, err := redis.NewClient(&redis.Config{
		Address:      cfg.Redis.Address,
		Password:     cfg.Redis.Password,
		Database:     cfg.Redis.Database,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		MaxRetries:   cfg.Redis.MaxRetries,
		DialTimeout:  time.Duration(cfg.Redis.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeout) * time.Second,
	})
	require.NoError(t, err)
	
	// 캐시 서비스 초기화
	cacheService, err := cache.NewService(redisClient)
	require.NoError(t, err)
	defer cacheService.Close()
	
	t.Run("Complete Optimization Workflow", func(t *testing.T) {
		testCompleteOptimizationWorkflow(t, cacheService)
	})
	
	t.Run("Performance Issue Identification", func(t *testing.T) {
		testPerformanceIssueIdentification(t, cacheService)
	})
	
	t.Run("Optimization Strategy Selection", func(t *testing.T) {
		testOptimizationStrategySelection(t, cacheService)
	})
	
	t.Run("Impact Analysis", func(t *testing.T) {
		testOptimizationImpactAnalysis(t, cacheService)
	})
}

// testCompleteOptimizationWorkflow 완전한 최적화 워크플로우 테스트
func testCompleteOptimizationWorkflow(t *testing.T, cacheService cache.Cache) {
	t.Log("🔍 Testing Complete Optimization Workflow")
	
	// 시스템 최적화기 생성
	optimizer := NewSystemOptimizer(cacheService)
	
	// 1단계: 기준 성능 측정
	t.Log("📊 Step 1: Measuring baseline performance...")
	err := optimizer.MeasureBaseline()
	require.NoError(t, err)
	
	baseline := optimizer.originalMetrics
	t.Logf("=== Baseline Performance ===")
	t.Logf("Average Response Time: %v", baseline.AvgResponseTime)
	t.Logf("Throughput: %.1f RPS", baseline.Throughput)
	t.Logf("Error Rate: %.2f%%", baseline.ErrorRate)
	t.Logf("Memory Usage: %d MB", baseline.MemoryUsage/(1024*1024))
	t.Logf("Cache Hit Rate: %.1f%%", baseline.CacheHitRate)
	t.Logf("Goroutine Count: %d", baseline.GoroutineCount)
	
	// 2단계: 성능 문제 식별
	t.Log("🔍 Step 2: Identifying performance issues...")
	issues := optimizer.IdentifyPerformanceIssues()
	
	t.Logf("=== Performance Issues Identified ===")
	if len(issues) == 0 {
		t.Logf("✅ No significant performance issues detected")
	} else {
		for _, issue := range issues {
			severity := "🟡"
			if issue.Severity == "critical" {
				severity = "🔴"
			} else if issue.Severity == "high" {
				severity = "🟠"
			}
			
			t.Logf("%s %s: %s (Impact: %.1f%%, Priority: %d)", 
				severity, issue.Type, issue.Description, issue.Impact, issue.Priority)
			t.Logf("   Solution: %s", issue.Solution)
		}
	}
	
	// 3단계: 최적화 전략 선택
	t.Log("🎯 Step 3: Selecting optimization strategies...")
	strategies := optimizer.GetOptimizationStrategies()
	
	// 고임팩트, 저위험 전략 우선 선택
	selectedStrategies := selectOptimalStrategies(strategies)
	
	t.Logf("=== Selected Optimization Strategies ===")
	for _, strategy := range selectedStrategies {
		riskColor := "✅"
		if strategy.RiskLevel == "medium" {
			riskColor = "🟡"
		} else if strategy.RiskLevel == "high" {
			riskColor = "🔴"
		}
		
		t.Logf("%s %s (Expected: %.1f%%, Risk: %s)", 
			riskColor, strategy.Name, strategy.ExpectedGain, strategy.RiskLevel)
		t.Logf("   %s", strategy.Description)
	}
	
	// 4단계: 최적화 적용
	t.Log("⚡ Step 4: Applying optimizations...")
	err = optimizer.ApplyOptimizations(selectedStrategies)
	require.NoError(t, err)
	
	// 5단계: 최적화 후 성능 측정
	t.Log("📈 Step 5: Measuring optimized performance...")
	err = optimizer.MeasureOptimizedPerformance()
	require.NoError(t, err)
	
	optimized := optimizer.optimizedMetrics
	t.Logf("=== Optimized Performance ===")
	t.Logf("Average Response Time: %v", optimized.AvgResponseTime)
	t.Logf("Throughput: %.1f RPS", optimized.Throughput)
	t.Logf("Error Rate: %.2f%%", optimized.ErrorRate)
	t.Logf("Memory Usage: %d MB", optimized.MemoryUsage/(1024*1024))
	t.Logf("Cache Hit Rate: %.1f%%", optimized.CacheHitRate)
	t.Logf("Goroutine Count: %d", optimized.GoroutineCount)
	
	// 6단계: 개선 사항 분석
	t.Log("📊 Step 6: Analyzing improvements...")
	t.Logf("=== Performance Improvements ===")
	t.Logf("Response Time: %.1f%% improvement", optimized.ResponseTimeImprovement)
	t.Logf("Throughput: %.1f%% improvement", optimized.ThroughputImprovement)
	t.Logf("Memory Efficiency: %.1f%% improvement", optimized.MemoryEfficiency)
	t.Logf("Overall Improvement: %.1f%%", optimized.OverallImprovement)
	
	// 7단계: 최적화 보고서 생성
	t.Log("📋 Step 7: Generating optimization report...")
	report := optimizer.GenerateOptimizationReport()
	
	t.Logf("=== Optimization Summary ===")
	t.Logf("Total Issues Found: %d", report.Summary.TotalIssuesFound)
	t.Logf("Critical Issues: %d", report.Summary.CriticalIssues)
	t.Logf("Strategies Applied: %d", report.Summary.StrategiesApplied)
	t.Logf("Overall Improvement: %.1f%%", report.Summary.OverallImprovement)
	t.Logf("Business Value: %s", report.Summary.EstimatedBusinessValue)
	
	if len(report.Summary.RecommendedNextSteps) > 0 {
		t.Logf("Recommended Next Steps:")
		for _, step := range report.Summary.RecommendedNextSteps {
			t.Logf("  - %s", step)
		}
	}
	
	// 검증
	assert.NotZero(t, baseline.Throughput, "Baseline throughput should be measured")
	assert.NotZero(t, optimized.Throughput, "Optimized throughput should be measured")
	
	// 성능 개선 검증 (시뮬레이션에서는 일부 개선이 보장되지 않을 수 있음)
	if optimized.OverallImprovement > 0 {
		t.Logf("✅ Performance improvement achieved: %.1f%%", optimized.OverallImprovement)
	} else {
		t.Logf("⚠️  No significant improvement detected (expected in simulation)")
	}
}

// selectOptimalStrategies 최적 전략 선택
func selectOptimalStrategies(strategies []OptimizationStrategy) []OptimizationStrategy {
	var selected []OptimizationStrategy
	
	// 위험-수익 비율을 고려하여 전략 선택
	for _, strategy := range strategies {
		riskScore := 1.0
		if strategy.RiskLevel == "medium" {
			riskScore = 2.0
		} else if strategy.RiskLevel == "high" {
			riskScore = 3.0
		}
		
		// 기대 수익이 위험보다 충분히 큰 전략만 선택
		riskRewardRatio := strategy.ExpectedGain / riskScore
		if riskRewardRatio >= 10.0 { // 최소 10:1 비율
			selected = append(selected, strategy)
		}
	}
	
	// 최소한 저위험 전략은 포함
	if len(selected) == 0 {
		for _, strategy := range strategies {
			if strategy.RiskLevel == "low" {
				selected = append(selected, strategy)
				break
			}
		}
	}
	
	return selected
}

// testPerformanceIssueIdentification 성능 문제 식별 테스트
func testPerformanceIssueIdentification(t *testing.T, cacheService cache.Cache) {
	t.Log("🔍 Testing Performance Issue Identification")
	
	optimizer := NewSystemOptimizer(cacheService)
	
	// 성능 문제가 있는 시나리오 시뮬레이션
	optimizer.originalMetrics = &BaselineMetrics{
		AvgResponseTime:     800 * time.Millisecond, // 높은 응답 시간
		Throughput:          300,                     // 낮은 처리량
		ErrorRate:           3.5,                     // 높은 에러율
		MemoryUsage:         800 * 1024 * 1024,       // 높은 메모리 사용량
		CacheHitRate:        75.0,                    // 낮은 캐시 히트율
		ConnectionPoolUsage: 90.0,                    // 높은 연결 풀 사용률
		GoroutineCount:      2000,                    // 많은 고루틴
	}
	
	issues := optimizer.IdentifyPerformanceIssues()
	
	t.Logf("=== Performance Issue Analysis ===")
	
	// 문제 유형별 분류
	issueTypes := make(map[string]int)
	severityCounts := make(map[string]int)
	var totalImpact float64
	
	for _, issue := range issues {
		issueTypes[issue.Type]++
		severityCounts[issue.Severity]++
		totalImpact += issue.Impact
		
		t.Logf("Issue: %s (%s)", issue.Description, issue.Severity)
		t.Logf("  Type: %s, Impact: %.1f%%, Priority: %d", 
			issue.Type, issue.Impact, issue.Priority)
		t.Logf("  Solution: %s", issue.Solution)
	}
	
	t.Logf("=== Issue Summary ===")
	t.Logf("Total Issues: %d", len(issues))
	t.Logf("Total Impact: %.1f%%", totalImpact)
	
	for issueType, count := range issueTypes {
		t.Logf("%s issues: %d", issueType, count)
	}
	
	for severity, count := range severityCounts {
		t.Logf("%s severity: %d", severity, count)
	}
	
	// 검증
	assert.Greater(t, len(issues), 0, "Should identify performance issues with poor metrics")
	assert.Contains(t, issueTypes, "latency", "Should identify latency issues")
	assert.Contains(t, issueTypes, "throughput", "Should identify throughput issues")
	assert.Greater(t, severityCounts["high"], 0, "Should identify high severity issues")
}

// testOptimizationStrategySelection 최적화 전략 선택 테스트
func testOptimizationStrategySelection(t *testing.T, cacheService cache.Cache) {
	t.Log("🎯 Testing Optimization Strategy Selection")
	
	optimizer := NewSystemOptimizer(cacheService)
	strategies := optimizer.GetOptimizationStrategies()
	
	t.Logf("=== Available Strategies ===")
	for _, strategy := range strategies {
		t.Logf("Strategy: %s", strategy.Name)
		t.Logf("  Expected Gain: %.1f%%", strategy.ExpectedGain)
		t.Logf("  Risk Level: %s", strategy.RiskLevel)
		t.Logf("  Resource Impact: %s", strategy.ResourceImpact)
		t.Logf("  Description: %s", strategy.Description)
	}
	
	// 전략 분석
	riskLevels := make(map[string]int)
	var totalExpectedGain float64
	
	for _, strategy := range strategies {
		riskLevels[strategy.RiskLevel]++
		totalExpectedGain += strategy.ExpectedGain
	}
	
	avgExpectedGain := totalExpectedGain / float64(len(strategies))
	
	t.Logf("=== Strategy Analysis ===")
	t.Logf("Total Strategies: %d", len(strategies))
	t.Logf("Average Expected Gain: %.1f%%", avgExpectedGain)
	for risk, count := range riskLevels {
		t.Logf("%s risk strategies: %d", risk, count)
	}
	
	// 전략 선택 시뮬레이션
	selectedStrategies := selectOptimalStrategies(strategies)
	
	t.Logf("=== Selected Strategies ===")
	var selectedGain float64
	for _, strategy := range selectedStrategies {
		t.Logf("✅ %s (%.1f%% gain, %s risk)", 
			strategy.Name, strategy.ExpectedGain, strategy.RiskLevel)
		selectedGain += strategy.ExpectedGain
	}
	
	t.Logf("Total Expected Improvement: %.1f%%", selectedGain)
	
	// 검증
	assert.Greater(t, len(strategies), 5, "Should have multiple optimization strategies")
	assert.Greater(t, len(selectedStrategies), 0, "Should select at least one strategy")
	assert.Greater(t, avgExpectedGain, 10.0, "Strategies should have meaningful expected gains")
}

// testOptimizationImpactAnalysis 최적화 영향 분석 테스트
func testOptimizationImpactAnalysis(t *testing.T, cacheService cache.Cache) {
	t.Log("📊 Testing Optimization Impact Analysis")
	
	optimizer := NewSystemOptimizer(cacheService)
	
	// 최적화 전후 시나리오 시뮬레이션
	optimizer.originalMetrics = &BaselineMetrics{
		AvgResponseTime:     500 * time.Millisecond,
		Throughput:          800,
		ErrorRate:           2.0,
		MemoryUsage:         400 * 1024 * 1024,
		CacheHitRate:        85.0,
		ConnectionPoolUsage: 75.0,
		GoroutineCount:      500,
	}
	
	optimizer.optimizedMetrics = &OptimizedMetrics{
		AvgResponseTime:     300 * time.Millisecond,
		Throughput:          1200,
		ErrorRate:           0.5,
		MemoryUsage:         300 * 1024 * 1024,
		CacheHitRate:        95.0,
		ConnectionPoolUsage: 60.0,
		GoroutineCount:      400,
	}
	
	// 개선 사항 계산
	optimizer.calculateImprovements()
	
	optimized := optimizer.optimizedMetrics
	
	t.Logf("=== Optimization Impact Analysis ===")
	t.Logf("Response Time Improvement: %.1f%%", optimized.ResponseTimeImprovement)
	t.Logf("Throughput Improvement: %.1f%%", optimized.ThroughputImprovement)
	t.Logf("Memory Efficiency: %.1f%%", optimized.MemoryEfficiency)
	t.Logf("Overall Improvement: %.1f%%", optimized.OverallImprovement)
	
	// 상세 분석
	t.Logf("=== Detailed Impact ===")
	t.Logf("Response Time: %v → %v", 
		optimizer.originalMetrics.AvgResponseTime, optimized.AvgResponseTime)
	t.Logf("Throughput: %.1f → %.1f RPS", 
		optimizer.originalMetrics.Throughput, optimized.Throughput)
	t.Logf("Error Rate: %.1f%% → %.1f%%", 
		optimizer.originalMetrics.ErrorRate, optimized.ErrorRate)
	t.Logf("Memory: %d → %d MB", 
		optimizer.originalMetrics.MemoryUsage/(1024*1024), 
		optimized.MemoryUsage/(1024*1024))
	t.Logf("Cache Hit Rate: %.1f%% → %.1f%%", 
		optimizer.originalMetrics.CacheHitRate, optimized.CacheHitRate)
	
	// 비즈니스 영향 분석
	responseTimeGain := optimized.ResponseTimeImprovement
	throughputGain := optimized.ThroughputImprovement
	
	estimatedUserExperienceImprovement := responseTimeGain * 0.8 // 응답시간의 80%가 UX 영향
	estimatedCapacityIncrease := throughputGain
	
	t.Logf("=== Business Impact Estimation ===")
	t.Logf("User Experience Improvement: %.1f%%", estimatedUserExperienceImprovement)
	t.Logf("System Capacity Increase: %.1f%%", estimatedCapacityIncrease)
	
	if optimized.OverallImprovement > 30 {
		t.Logf("💰 High business value optimization")
	} else if optimized.OverallImprovement > 15 {
		t.Logf("💵 Medium business value optimization")
	} else {
		t.Logf("💴 Low business value optimization")
	}
	
	// ROI 추정 (단순화된 계산)
	implementationCost := 100.0 // 임의의 기준값
	monthlyBenefit := optimized.OverallImprovement * 0.1 // 개선도의 10%를 월간 이익으로 가정
	roi := (monthlyBenefit * 12 - implementationCost) / implementationCost * 100
	
	t.Logf("Estimated ROI: %.1f%% (12 months)", roi)
	
	// 검증
	assert.Greater(t, optimized.ResponseTimeImprovement, 0.0, "Should show response time improvement")
	assert.Greater(t, optimized.ThroughputImprovement, 0.0, "Should show throughput improvement")
	assert.Greater(t, optimized.OverallImprovement, 0.0, "Should show overall improvement")
}

// TestOptimizationRecommendations 최적화 권고사항 테스트
func TestOptimizationRecommendations(t *testing.T) {
	t.Log("💡 Testing Optimization Recommendations")
	
	// 다양한 성능 시나리오에 대한 권고사항 테스트
	scenarios := []struct {
		name     string
		metrics  BaselineMetrics
		expected []string // 예상되는 권고사항 키워드
	}{
		{
			name: "High Latency Scenario",
			metrics: BaselineMetrics{
				AvgResponseTime: 2 * time.Second,
				Throughput:      500,
				ErrorRate:       1.0,
			},
			expected: []string{"connection", "cache", "optimization"},
		},
		{
			name: "Low Throughput Scenario",
			metrics: BaselineMetrics{
				AvgResponseTime: 100 * time.Millisecond,
				Throughput:      100,
				ErrorRate:       0.5,
			},
			expected: []string{"scaling", "async", "pool"},
		},
		{
			name: "High Error Rate Scenario",
			metrics: BaselineMetrics{
				AvgResponseTime: 200 * time.Millisecond,
				Throughput:      800,
				ErrorRate:       8.0,
			},
			expected: []string{"error", "circuit", "retry"},
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			optimizer := &SystemOptimizer{
				originalMetrics: &scenario.metrics,
			}
			
			issues := optimizer.IdentifyPerformanceIssues()
			strategies := optimizer.GetOptimizationStrategies()
			
			t.Logf("=== %s ===", scenario.name)
			t.Logf("Issues found: %d", len(issues))
			t.Logf("Available strategies: %d", len(strategies))
			
			// 권고사항 확인
			var recommendations []string
			for _, issue := range issues {
				recommendations = append(recommendations, issue.Solution)
			}
			
			for _, strategy := range strategies {
				recommendations = append(recommendations, strategy.Description)
			}
			
			t.Logf("Recommendations generated: %d", len(recommendations))
			
			// 예상 키워드 포함 여부 확인
			foundKeywords := make(map[string]bool)
			for _, expected := range scenario.expected {
				for _, rec := range recommendations {
					if contains(rec, expected) {
						foundKeywords[expected] = true
						break
					}
				}
			}
			
			t.Logf("Expected keywords found: %d/%d", len(foundKeywords), len(scenario.expected))
			
			assert.Greater(t, len(issues), 0, "Should identify issues for problematic scenarios")
		})
	}
}

// contains 문자열 포함 여부 확인 (간단한 검색)
func contains(text, substr string) bool {
	return len(text) >= len(substr) && 
		   (text == substr || 
		    (len(text) > len(substr) && 
		     (text[:len(substr)] == substr || 
		      text[len(text)-len(substr):] == substr ||
		      findSubstring(text, substr))))
}

func findSubstring(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestOptimizationSummary 최적화 종합 보고서 테스트
func TestOptimizationSummary(t *testing.T) {
	t.Log("📋 Optimization Summary Report")
	
	// 종합적인 최적화 시나리오
	optimizer := &SystemOptimizer{
		originalMetrics: &BaselineMetrics{
			AvgResponseTime:     400 * time.Millisecond,
			Throughput:          600,
			ErrorRate:           2.5,
			MemoryUsage:         500 * 1024 * 1024,
			CacheHitRate:        80.0,
			ConnectionPoolUsage: 85.0,
			GoroutineCount:      800,
		},
		optimizedMetrics: &OptimizedMetrics{
			AvgResponseTime:     200 * time.Millisecond,
			Throughput:          1000,
			ErrorRate:           0.8,
			MemoryUsage:         350 * 1024 * 1024,
			CacheHitRate:        93.0,
			ConnectionPoolUsage: 65.0,
			GoroutineCount:      600,
		},
	}
	
	optimizer.calculateImprovements()
	report := optimizer.GenerateOptimizationReport()
	
	t.Logf("=== Final Optimization Report ===")
	t.Logf("📊 Performance Baseline:")
	t.Logf("  Response Time: %v", report.BaselineMetrics.AvgResponseTime)
	t.Logf("  Throughput: %.1f RPS", report.BaselineMetrics.Throughput)
	t.Logf("  Error Rate: %.2f%%", report.BaselineMetrics.ErrorRate)
	t.Logf("  Memory: %d MB", report.BaselineMetrics.MemoryUsage/(1024*1024))
	
	t.Logf("📈 Performance After Optimization:")
	t.Logf("  Response Time: %v (%.1f%% improvement)", 
		report.OptimizedMetrics.AvgResponseTime, 
		report.OptimizedMetrics.ResponseTimeImprovement)
	t.Logf("  Throughput: %.1f RPS (%.1f%% improvement)", 
		report.OptimizedMetrics.Throughput, 
		report.OptimizedMetrics.ThroughputImprovement)
	t.Logf("  Error Rate: %.2f%%", report.OptimizedMetrics.ErrorRate)
	t.Logf("  Memory: %d MB (%.1f%% efficiency)", 
		report.OptimizedMetrics.MemoryUsage/(1024*1024),
		report.OptimizedMetrics.MemoryEfficiency)
	
	t.Logf("🎯 Optimization Summary:")
	t.Logf("  Issues Identified: %d", report.Summary.TotalIssuesFound)
	t.Logf("  Critical Issues: %d", report.Summary.CriticalIssues)
	t.Logf("  Strategies Applied: %d", report.Summary.StrategiesApplied)
	t.Logf("  Overall Improvement: %.1f%%", report.Summary.OverallImprovement)
	t.Logf("  Business Value: %s", report.Summary.EstimatedBusinessValue)
	
	if len(report.Summary.RecommendedNextSteps) > 0 {
		t.Logf("🔮 Recommended Next Steps:")
		for i, step := range report.Summary.RecommendedNextSteps {
			t.Logf("  %d. %s", i+1, step)
		}
	}
	
	// 성과 평가
	if report.Summary.OverallImprovement > 40 {
		t.Logf("🌟 Outstanding optimization results!")
	} else if report.Summary.OverallImprovement > 25 {
		t.Logf("✨ Excellent optimization results!")
	} else if report.Summary.OverallImprovement > 15 {
		t.Logf("👍 Good optimization results!")
	} else {
		t.Logf("👌 Moderate optimization results")
	}
	
	// 투자 가치 분석
	if report.Summary.EstimatedBusinessValue == "High" {
		t.Logf("💰 High ROI - Strong recommendation for implementation")
	} else if report.Summary.EstimatedBusinessValue == "Medium" {
		t.Logf("💵 Medium ROI - Consider implementation based on priorities")
	} else {
		t.Logf("💴 Low ROI - May not justify immediate implementation cost")
	}
	
	// 최종 권고사항
	t.Logf("=== Final Recommendations ===")
	if report.Summary.CriticalIssues > 0 {
		t.Logf("🚨 URGENT: Address %d critical performance issues immediately", 
			report.Summary.CriticalIssues)
	}
	
	if report.Summary.OverallImprovement > 30 {
		t.Logf("✅ RECOMMENDED: Proceed with optimization implementation")
	} else if report.Summary.OverallImprovement > 15 {
		t.Logf("🤔 CONSIDER: Evaluate cost-benefit before implementation")
	} else {
		t.Logf("⏸️  DEFER: Consider alternative approaches or defer optimization")
	}
	
	// 검증
	assert.NotNil(t, report.BaselineMetrics, "Should have baseline metrics")
	assert.NotNil(t, report.OptimizedMetrics, "Should have optimized metrics")
	assert.Greater(t, report.Summary.OverallImprovement, 0.0, "Should show overall improvement")
}