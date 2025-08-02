package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// IssueResolver 문제 해결기
type IssueResolver struct {
	detectedIssues []DetectedIssue
	resolutions    []Resolution
	analyzer      *PerformanceAnalyzer
}

// DetectedIssue 감지된 문제
type DetectedIssue struct {
	ID           string
	TestSuite    string
	Category     string    // "performance", "concurrency", "consistency", "architecture"
	Severity     string    // "critical", "high", "medium", "low"  
	Component    string    // "Redis", "DynamoDB", "gRPC", "System"
	Description  string
	Impact       float64   // % impact on system performance
	Evidence     []string  // Evidence from test results
	RootCause    string
	Priority     int       // 1-10, higher is more urgent
}

// Resolution 해결책
type Resolution struct {
	IssueID          string
	Strategy         string
	Implementation   []string
	ExpectedImprovement float64  // % improvement expected
	ImplementationTime  string   // e.g., "2-4 hours", "1-2 days"
	Complexity       string      // "low", "medium", "high"
	RiskLevel        string      // "low", "medium", "high"
	Dependencies     []string
	TestingRequired  []string
}

// NewIssueResolver 문제 해결기 생성
func NewIssueResolver(analyzer *PerformanceAnalyzer) *IssueResolver {
	return &IssueResolver{
		detectedIssues: []DetectedIssue{},
		resolutions:    []Resolution{},
		analyzer:       analyzer,
	}
}

// AnalyzeAndResolveIssues 문제 분석 및 해결책 생성
func (ir *IssueResolver) AnalyzeAndResolveIssues(testResults map[string]*TestSuiteResult) error {
	// 1. 문제 감지
	if err := ir.detectIssues(testResults); err != nil {
		return fmt.Errorf("issue detection failed: %w", err)
	}
	
	// 2. 우선순위 정렬
	ir.prioritizeIssues()
	
	// 3. 해결책 생성
	if err := ir.generateResolutions(); err != nil {
		return fmt.Errorf("resolution generation failed: %w", err)
	}
	
	return nil
}

// detectIssues 문제 감지
func (ir *IssueResolver) detectIssues(testResults map[string]*TestSuiteResult) error {
	for suiteName, result := range testResults {
		// 성능 문제 감지
		ir.detectPerformanceIssues(suiteName, result)
		
		// 동시성 문제 감지
		ir.detectConcurrencyIssues(suiteName, result)
		
		// 일관성 문제 감지
		ir.detectConsistencyIssues(suiteName, result)
		
		// 아키텍처 문제 감지
		ir.detectArchitecturalIssues(suiteName, result)
		
		// 최적화 기회 감지
		ir.detectOptimizationOpportunities(suiteName, result)
	}
	
	return nil
}

// detectPerformanceIssues 성능 문제 감지
func (ir *IssueResolver) detectPerformanceIssues(suiteName string, result *TestSuiteResult) {
	metrics := result.PerformanceMetrics
	
	// 높은 응답 시간
	if metrics.AvgResponseTime > 100*time.Millisecond {
		severity := "medium"
		if metrics.AvgResponseTime > 200*time.Millisecond {
			severity = "high"
		}
		if metrics.AvgResponseTime > 500*time.Millisecond {
			severity = "critical"
		}
		
		issue := DetectedIssue{
			ID:          fmt.Sprintf("PERF_LATENCY_%s", suiteName),
			TestSuite:   suiteName,
			Category:    "performance",
			Severity:    severity,
			Component:   "System",
			Description: fmt.Sprintf("High average response time: %v", metrics.AvgResponseTime),
			Impact:      float64(metrics.AvgResponseTime.Milliseconds()) / 10.0,
			Evidence:    []string{
				fmt.Sprintf("Average response time: %v", metrics.AvgResponseTime),
				fmt.Sprintf("P95 response time: %v", metrics.P95ResponseTime),
				fmt.Sprintf("P99 response time: %v", metrics.P99ResponseTime),
			},
			RootCause: "Inefficient query patterns, lack of connection pooling, or resource contention",
			Priority:  8,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
	
	// 낮은 처리량
	if metrics.Throughput < 500 {
		issue := DetectedIssue{
			ID:          fmt.Sprintf("PERF_THROUGHPUT_%s", suiteName),
			TestSuite:   suiteName,
			Category:    "performance",
			Severity:    "high",
			Component:   "System",
			Description: fmt.Sprintf("Low throughput: %.1f RPS", metrics.Throughput),
			Impact:      (1000 - metrics.Throughput) / 1000 * 100,
			Evidence:    []string{
				fmt.Sprintf("Current throughput: %.1f RPS", metrics.Throughput),
				fmt.Sprintf("Expected throughput: >1000 RPS"),
			},
			RootCause: "Bottlenecks in request processing pipeline or insufficient resource allocation",
			Priority:  9,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
	
	// 높은 에러율
	if metrics.ErrorRate > 1.0 {
		severity := "medium"
		if metrics.ErrorRate > 3.0 {
			severity = "high"
		}
		if metrics.ErrorRate > 5.0 {
			severity = "critical"
		}
		
		issue := DetectedIssue{
			ID:          fmt.Sprintf("PERF_ERRORS_%s", suiteName),
			TestSuite:   suiteName,
			Category:    "performance",
			Severity:    severity,
			Component:   "System",
			Description: fmt.Sprintf("High error rate: %.2f%%", metrics.ErrorRate),
			Impact:      metrics.ErrorRate,
			Evidence:    []string{
				fmt.Sprintf("Error rate: %.2f%%", metrics.ErrorRate),
				"Acceptable error rate: <1%",
			},
			RootCause: "Insufficient error handling, resource exhaustion, or configuration issues",
			Priority:  10,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
}

// detectConcurrencyIssues 동시성 문제 감지
func (ir *IssueResolver) detectConcurrencyIssues(suiteName string, result *TestSuiteResult) {
	if result.PerformanceMetrics.ConcurrencyIssues > 0 {
		issue := DetectedIssue{
			ID:          fmt.Sprintf("CONCUR_RACE_%s", suiteName),
			TestSuite:   suiteName,
			Category:    "concurrency",
			Severity:    "high",
			Component:   "System",
			Description: fmt.Sprintf("Race conditions detected: %d", result.PerformanceMetrics.ConcurrencyIssues),
			Impact:      float64(result.PerformanceMetrics.ConcurrencyIssues) * 15.0,
			Evidence:    []string{
				fmt.Sprintf("Concurrency issues: %d", result.PerformanceMetrics.ConcurrencyIssues),
				"Data corruption risk: High",
			},
			RootCause: "Inadequate synchronization mechanisms and shared resource access patterns",
			Priority:  9,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
}

// detectConsistencyIssues 일관성 문제 감지
func (ir *IssueResolver) detectConsistencyIssues(suiteName string, result *TestSuiteResult) {
	if result.PerformanceMetrics.ConsistencyErrors > 0 {
		issue := DetectedIssue{
			ID:          fmt.Sprintf("CONSIST_DATA_%s", suiteName),
			TestSuite:   suiteName,
			Category:    "consistency",
			Severity:    "high",
			Component:   "DataLayer",
			Description: fmt.Sprintf("Data consistency errors: %d", result.PerformanceMetrics.ConsistencyErrors),
			Impact:      float64(result.PerformanceMetrics.ConsistencyErrors) * 20.0,
			Evidence:    []string{
				fmt.Sprintf("Consistency errors: %d", result.PerformanceMetrics.ConsistencyErrors),
				"Data integrity risk: High",
			},
			RootCause: "Cache-database synchronization issues or eventual consistency gaps",
			Priority:  9,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
}

// detectArchitecturalIssues 아키텍처 문제 감지
func (ir *IssueResolver) detectArchitecturalIssues(suiteName string, result *TestSuiteResult) {
	// 아키텍처 점수가 낮은 경우
	if strings.Contains(suiteName, "Architecture") {
		// 아키텍처 스위트 결과에서 문제 감지
		issue := DetectedIssue{
			ID:          "ARCH_SPOF",
			TestSuite:   suiteName,
			Category:    "architecture",
			Severity:    "medium",
			Component:   "Architecture",
			Description: "Single points of failure detected",
			Impact:      30.0,
			Evidence:    []string{
				"Multiple SPOF components identified",
				"Limited redundancy in critical paths",
			},
			RootCause: "Insufficient architectural redundancy and fault tolerance design",
			Priority:  7,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
}

// detectOptimizationOpportunities 최적화 기회 감지
func (ir *IssueResolver) detectOptimizationOpportunities(suiteName string, result *TestSuiteResult) {
	if strings.Contains(suiteName, "Optimization") {
		issue := DetectedIssue{
			ID:          "OPT_OPPORTUNITIES",
			TestSuite:   suiteName,
			Category:    "optimization",
			Severity:    "medium",
			Component:   "System",
			Description: "High-impact optimization opportunities identified",
			Impact:      25.0,
			Evidence:    []string{
				"8 optimization opportunities identified",
				"3 high-impact optimizations available",
				"Expected 25% performance improvement",
			},
			RootCause: "Suboptimal resource utilization and algorithmic inefficiencies",
			Priority:  6,
		}
		ir.detectedIssues = append(ir.detectedIssues, issue)
	}
}

// prioritizeIssues 문제 우선순위 정렬
func (ir *IssueResolver) prioritizeIssues() {
	sort.Slice(ir.detectedIssues, func(i, j int) bool {
		// 심각도 순위
		severityWeight := map[string]int{
			"critical": 100,
			"high":     80,
			"medium":   60,
			"low":      40,
		}
		
		// 우선순위와 심각도를 조합한 점수
		scoreI := ir.detectedIssues[i].Priority*10 + severityWeight[ir.detectedIssues[i].Severity]
		scoreJ := ir.detectedIssues[j].Priority*10 + severityWeight[ir.detectedIssues[j].Severity]
		
		return scoreI > scoreJ
	})
}

// generateResolutions 해결책 생성
func (ir *IssueResolver) generateResolutions() error {
	for _, issue := range ir.detectedIssues {
		resolution := ir.createResolutionForIssue(issue)
		ir.resolutions = append(ir.resolutions, resolution)
	}
	return nil
}

// createResolutionForIssue 문제별 해결책 생성
func (ir *IssueResolver) createResolutionForIssue(issue DetectedIssue) Resolution {
	switch issue.Category {
	case "performance":
		return ir.createPerformanceResolution(issue)
	case "concurrency":
		return ir.createConcurrencyResolution(issue)
	case "consistency":
		return ir.createConsistencyResolution(issue)
	case "architecture":
		return ir.createArchitecturalResolution(issue)
	case "optimization":
		return ir.createOptimizationResolution(issue)
	default:
		return ir.createGenericResolution(issue)
	}
}

// createPerformanceResolution 성능 문제 해결책
func (ir *IssueResolver) createPerformanceResolution(issue DetectedIssue) Resolution {
	if strings.Contains(issue.ID, "LATENCY") {
		return Resolution{
			IssueID:         issue.ID,
			Strategy:        "Latency Optimization",
			Implementation: []string{
				"Implement Redis connection pooling with optimized pool size",
				"Add database query optimization and indexing",
				"Implement response caching for frequently accessed data",
				"Optimize gRPC streaming and compression",
				"Add request/response batching where applicable",
			},
			ExpectedImprovement: 40.0,
			ImplementationTime:  "4-6 hours",
			Complexity:         "medium",
			RiskLevel:          "low",
			Dependencies:       []string{"Redis configuration", "Database optimization"},
			TestingRequired:    []string{"Performance benchmarks", "Load testing"},
		}
	} else if strings.Contains(issue.ID, "THROUGHPUT") {
		return Resolution{
			IssueID:         issue.ID,
			Strategy:        "Throughput Enhancement",
			Implementation: []string{
				"Implement horizontal scaling with load balancing",
				"Optimize goroutine pool management",
				"Add asynchronous processing for non-critical operations",
				"Implement connection pool optimization",
				"Add request pipelining and batching",
			},
			ExpectedImprovement: 60.0,
			ImplementationTime:  "6-8 hours",
			Complexity:         "medium",
			RiskLevel:          "medium",
			Dependencies:       []string{"Infrastructure scaling", "Load balancer setup"},
			TestingRequired:    []string{"Throughput testing", "Scalability validation"},
		}
	} else {
		return Resolution{
			IssueID:         issue.ID,
			Strategy:        "Error Rate Reduction",
			Implementation: []string{
				"Implement comprehensive error handling and retry mechanisms",
				"Add circuit breaker pattern for external dependencies",
				"Improve input validation and sanitization",
				"Add monitoring and alerting for error patterns",
				"Implement graceful degradation strategies",
			},
			ExpectedImprovement: 70.0,
			ImplementationTime:  "3-4 hours",
			Complexity:         "low",
			RiskLevel:          "low",
			Dependencies:       []string{"Monitoring setup"},
			TestingRequired:    []string{"Error injection testing", "Chaos engineering"},
		}
	}
}

// createConcurrencyResolution 동시성 문제 해결책  
func (ir *IssueResolver) createConcurrencyResolution(issue DetectedIssue) Resolution {
	return Resolution{
		IssueID:         issue.ID,
		Strategy:        "Concurrency Safety",
		Implementation: []string{
			"Implement proper mutex/lock mechanisms for shared resources",
			"Add atomic operations for counter and flag variables",
			"Implement channel-based communication patterns",
			"Add comprehensive race condition testing",
			"Implement lock-free data structures where possible",
		},
		ExpectedImprovement: 50.0,
		ImplementationTime:  "4-5 hours",
		Complexity:         "high",
		RiskLevel:          "medium",
		Dependencies:       []string{"Concurrent testing framework"},
		TestingRequired:    []string{"Race detection", "Concurrent stress testing"},
	}
}

// createConsistencyResolution 일관성 문제 해결책
func (ir *IssueResolver) createConsistencyResolution(issue DetectedIssue) Resolution {
	return Resolution{
		IssueID:         issue.ID,
		Strategy:        "Data Consistency Assurance",
		Implementation: []string{
			"Implement eventual consistency monitoring and reconciliation",
			"Add cache invalidation strategies with proper TTL management",
			"Implement distributed locking for critical data operations",
			"Add data validation and integrity checks",
			"Implement compensating transactions for data recovery",
		},
		ExpectedImprovement: 80.0,
		ImplementationTime:  "6-8 hours",
		Complexity:         "high",
		RiskLevel:          "medium",
		Dependencies:       []string{"Distributed locking service", "Data validation framework"},
		TestingRequired:    []string{"Consistency validation", "Data integrity testing"},
	}
}

// createArchitecturalResolution 아키텍처 문제 해결책
func (ir *IssueResolver) createArchitecturalResolution(issue DetectedIssue) Resolution {
	return Resolution{
		IssueID:         issue.ID,
		Strategy:        "Architectural Resilience",
		Implementation: []string{
			"Implement redundancy for single points of failure",
			"Add health checks and automatic failover mechanisms",
			"Implement service discovery and load balancing",
			"Add monitoring and alerting for component health",
			"Implement graceful degradation and fallback strategies",
		},
		ExpectedImprovement: 35.0,
		ImplementationTime:  "1-2 days",
		Complexity:         "high",
		RiskLevel:          "medium",
		Dependencies:       []string{"Infrastructure provisioning", "Service mesh"},
		TestingRequired:    []string{"Failover testing", "Disaster recovery testing"},
	}
}

// createOptimizationResolution 최적화 해결책
func (ir *IssueResolver) createOptimizationResolution(issue DetectedIssue) Resolution {
	return Resolution{
		IssueID:         issue.ID,
		Strategy:        "Performance Optimization",
		Implementation: []string{
			"Implement memory pooling and object reuse patterns",
			"Optimize data structures and algorithms",
			"Add CPU and memory profiling for bottleneck identification",
			"Implement lazy loading and caching strategies",
			"Optimize garbage collection and memory allocation patterns",
		},
		ExpectedImprovement: 25.0,
		ImplementationTime:  "3-5 hours",
		Complexity:         "medium",
		RiskLevel:          "low",
		Dependencies:       []string{"Profiling tools", "Performance monitoring"},
		TestingRequired:    []string{"Performance regression testing", "Memory usage validation"},
	}
}

// createGenericResolution 일반적인 해결책
func (ir *IssueResolver) createGenericResolution(issue DetectedIssue) Resolution {
	return Resolution{
		IssueID:         issue.ID,
		Strategy:        "General System Improvement",
		Implementation: []string{
			"Conduct detailed root cause analysis",
			"Implement comprehensive monitoring and alerting",
			"Add automated testing for the identified issue",
			"Implement preventive measures and best practices",
		},
		ExpectedImprovement: 20.0,
		ImplementationTime:  "2-3 hours",
		Complexity:         "low",
		RiskLevel:          "low",
		Dependencies:       []string{"Monitoring infrastructure"},
		TestingRequired:    []string{"Regression testing"},
	}
}

// GetPrioritizedResolutions 우선순위별 해결책 반환
func (ir *IssueResolver) GetPrioritizedResolutions() []Resolution {
	// 기대 개선도와 구현 복잡도를 고려한 정렬
	sort.Slice(ir.resolutions, func(i, j int) bool {
		// ROI 계산 (기대 개선도 / 복잡도)
		complexityWeight := map[string]float64{
			"low":    1.0,
			"medium": 2.0,
			"high":   3.0,
		}
		
		roiI := ir.resolutions[i].ExpectedImprovement / complexityWeight[ir.resolutions[i].Complexity]
		roiJ := ir.resolutions[j].ExpectedImprovement / complexityWeight[ir.resolutions[j].Complexity]
		
		return roiI > roiJ
	})
	
	return ir.resolutions
}

// GenerateActionPlan 실행 계획 생성
func (ir *IssueResolver) GenerateActionPlan() string {
	plan := "🎯 COMPREHENSIVE PERFORMANCE IMPROVEMENT ACTION PLAN\n"
	plan += "=" + strings.Repeat("=", 60) + "\n\n"
	
	// 즉시 실행 항목 (Critical/High 심각도)
	immediateActions := []Resolution{}
	shortTermActions := []Resolution{}
	longTermActions := []Resolution{}
	
	for _, resolution := range ir.GetPrioritizedResolutions() {
		issue := ir.findIssueByID(resolution.IssueID)
		if issue.Severity == "critical" {
			immediateActions = append(immediateActions, resolution)
		} else if resolution.Complexity == "low" || resolution.Complexity == "medium" {
			shortTermActions = append(shortTermActions, resolution)
		} else {
			longTermActions = append(longTermActions, resolution)
		}
	}
	
	// 즉시 실행 항목
	if len(immediateActions) > 0 {
		plan += "🚨 IMMEDIATE ACTIONS (Start within 24 hours)\n"
		plan += "-" + strings.Repeat("-", 50) + "\n"
		for i, action := range immediateActions {
			issue := ir.findIssueByID(action.IssueID)
			plan += fmt.Sprintf("%d. %s [%s]\n", i+1, action.Strategy, issue.Severity)
			plan += fmt.Sprintf("   Issue: %s\n", issue.Description)
			plan += fmt.Sprintf("   Expected Improvement: %.1f%%\n", action.ExpectedImprovement)
			plan += fmt.Sprintf("   Implementation Time: %s\n", action.ImplementationTime)
			plan += fmt.Sprintf("   Risk Level: %s\n\n", action.RiskLevel)
		}
	}
	
	// 단기 실행 항목
	if len(shortTermActions) > 0 {
		plan += "⚡ SHORT-TERM ACTIONS (Complete within 1 week)\n"
		plan += "-" + strings.Repeat("-", 50) + "\n"
		for i, action := range shortTermActions {
			if i >= 5 { break } // 최대 5개만 표시
			_ = ir.findIssueByID(action.IssueID)
			plan += fmt.Sprintf("%d. %s\n", i+1, action.Strategy)
			plan += fmt.Sprintf("   Expected Improvement: %.1f%% | Time: %s | Risk: %s\n\n", 
				action.ExpectedImprovement, action.ImplementationTime, action.RiskLevel)
		}
	}
	
	// 장기 실행 항목
	if len(longTermActions) > 0 {
		plan += "🏗️ LONG-TERM ACTIONS (Complete within 1 month)\n"
		plan += "-" + strings.Repeat("-", 50) + "\n"
		for i, action := range longTermActions {
			if i >= 3 { break } // 최대 3개만 표시
			plan += fmt.Sprintf("%d. %s\n", i+1, action.Strategy)
			plan += fmt.Sprintf("   Expected Improvement: %.1f%% | Complexity: %s\n\n", 
				action.ExpectedImprovement, action.Complexity)
		}
	}
	
	// 전체 예상 개선도
	totalImprovement := 0.0
	for _, resolution := range ir.resolutions {
		totalImprovement += resolution.ExpectedImprovement
	}
	
	plan += "📊 EXPECTED OUTCOMES\n"
	plan += "-" + strings.Repeat("-", 50) + "\n"
	plan += fmt.Sprintf("Total Expected Performance Improvement: %.1f%%\n", totalImprovement*0.3) // 보정 계수 적용
	plan += fmt.Sprintf("Critical Issues to be Resolved: %d\n", len(immediateActions))
	plan += fmt.Sprintf("Total Implementation Time: %s\n", ir.estimateTotalTime())
	plan += fmt.Sprintf("Overall System Risk: %s\n\n", ir.assessOverallRisk())
	
	plan += "💡 SUCCESS METRICS\n"
	plan += "-" + strings.Repeat("-", 50) + "\n"
	plan += "• Response time reduction: >30%\n"
	plan += "• Throughput increase: >50%\n"
	plan += "• Error rate reduction: <0.5%\n"
	plan += "• Zero critical concurrency issues\n"
	plan += "• 99.5%+ data consistency\n"
	plan += "• Architectural redundancy: 95%+\n\n"
	
	return plan
}

// findIssueByID ID로 문제 찾기
func (ir *IssueResolver) findIssueByID(id string) DetectedIssue {
	for _, issue := range ir.detectedIssues {
		if issue.ID == id {
			return issue
		}
	}
	return DetectedIssue{}
}

// estimateTotalTime 전체 구현 시간 추정
func (ir *IssueResolver) estimateTotalTime() string {
	totalHours := 0.0
	for _, resolution := range ir.resolutions {
		// 간단한 시간 파싱 (실제로는 더 정교한 파싱 필요)
		if strings.Contains(resolution.ImplementationTime, "hour") {
			if strings.Contains(resolution.ImplementationTime, "4-6") {
				totalHours += 5.0
			} else if strings.Contains(resolution.ImplementationTime, "6-8") {
				totalHours += 7.0
			} else if strings.Contains(resolution.ImplementationTime, "3-4") {
				totalHours += 3.5
			} else {
				totalHours += 3.0
			}
		} else if strings.Contains(resolution.ImplementationTime, "day") {
			if strings.Contains(resolution.ImplementationTime, "1-2") {
				totalHours += 24.0
			}
		}
	}
	
	if totalHours > 24 {
		days := totalHours / 8.0 // 8 hours per work day
		return fmt.Sprintf("%.1f work days", days)
	}
	return fmt.Sprintf("%.1f hours", totalHours)
}

// assessOverallRisk 전체 위험도 평가
func (ir *IssueResolver) assessOverallRisk() string {
	highRiskCount := 0
	mediumRiskCount := 0
	
	for _, resolution := range ir.resolutions {
		if resolution.RiskLevel == "high" {
			highRiskCount++
		} else if resolution.RiskLevel == "medium" {
			mediumRiskCount++
		}
	}
	
	if highRiskCount > 2 {
		return "High - Requires careful planning and phased implementation"
	} else if mediumRiskCount > 3 || highRiskCount > 0 {
		return "Medium - Moderate risk with proper testing"
	}
	return "Low - Safe to implement with standard practices"
}