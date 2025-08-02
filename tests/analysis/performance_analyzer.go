package analysis

import (
	"fmt"
	"sort"
	"time"
)

// PerformanceAnalyzer 성능 분석기
type PerformanceAnalyzer struct {
	testResults map[string]*TestSuiteResult
	startTime   time.Time
	endTime     time.Time
}

// TestSuiteResult 테스트 스위트 결과
type TestSuiteResult struct {
	SuiteName     string
	Duration      time.Duration
	TestsRun      int
	TestsPassed   int
	TestsFailed   int
	TestsSkipped  int
	BottleneckIssues []BottleneckIssue
	PerformanceMetrics PerformanceMetrics
	Recommendations []string
}

// BottleneckIssue 병목 문제
type BottleneckIssue struct {
	Component   string    // "Redis", "DynamoDB", "gRPC", "Network"
	Type        string    // "latency", "throughput", "memory", "cpu"
	Severity    string    // "critical", "high", "medium", "low"
	Description string
	Impact      float64   // % impact on overall performance
	Solution    string
	Priority    int       // 1-10
}

// PerformanceMetrics 성능 메트릭
type PerformanceMetrics struct {
	AvgResponseTime   time.Duration
	MaxResponseTime   time.Duration
	MinResponseTime   time.Duration
	P95ResponseTime   time.Duration
	P99ResponseTime   time.Duration
	Throughput        float64  // RPS
	ErrorRate         float64  // %
	MemoryUsage       uint64   // bytes
	CPUUtilization    float64  // %
	CacheHitRate      float64  // %
	ConcurrencyIssues int
	ConsistencyErrors int
}

// AnalysisReport 분석 보고서
type AnalysisReport struct {
	OverallScore      float64
	TotalDuration     time.Duration
	SystemHealth      string
	CriticalIssues    []BottleneckIssue
	MajorIssues       []BottleneckIssue
	MinorIssues       []BottleneckIssue
	TopRecommendations []string
	PerformanceRanking map[string]float64
	ReadinessAssessment string
}

// NewPerformanceAnalyzer 성능 분석기 생성
func NewPerformanceAnalyzer() *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		testResults: make(map[string]*TestSuiteResult),
		startTime:   time.Now(),
	}
}

// AddTestResult 테스트 결과 추가
func (pa *PerformanceAnalyzer) AddTestResult(suiteName string, result *TestSuiteResult) {
	pa.testResults[suiteName] = result
}

// AnalyzeResults 결과 분석
func (pa *PerformanceAnalyzer) AnalyzeResults() *AnalysisReport {
	pa.endTime = time.Now()
	
	report := &AnalysisReport{
		TotalDuration: pa.endTime.Sub(pa.startTime),
		PerformanceRanking: make(map[string]float64),
	}
	
	// 전체 문제 수집
	var allIssues []BottleneckIssue
	var totalScore float64
	suiteCount := 0
	
	for suiteName, result := range pa.testResults {
		// 스위트별 점수 계산
		suiteScore := pa.calculateSuiteScore(result)
		report.PerformanceRanking[suiteName] = suiteScore
		totalScore += suiteScore
		suiteCount++
		
		// 문제점 수집
		allIssues = append(allIssues, result.BottleneckIssues...)
	}
	
	// 전체 점수 계산
	if suiteCount > 0 {
		report.OverallScore = totalScore / float64(suiteCount)
	}
	
	// 심각도별 문제 분류
	report.CriticalIssues = pa.filterIssuesBySeverity(allIssues, "critical")
	report.MajorIssues = pa.filterIssuesBySeverity(allIssues, "high")
	report.MinorIssues = pa.filterIssuesBySeverity(allIssues, "medium")
	
	// 시스템 상태 평가
	report.SystemHealth = pa.assessSystemHealth(report.OverallScore, len(report.CriticalIssues))
	
	// 준비도 평가
	report.ReadinessAssessment = pa.assessReadiness(report.OverallScore, len(report.CriticalIssues))
	
	// 최상위 권고사항 생성
	report.TopRecommendations = pa.generateTopRecommendations(allIssues)
	
	return report
}

// calculateSuiteScore 스위트별 점수 계산
func (pa *PerformanceAnalyzer) calculateSuiteScore(result *TestSuiteResult) float64 {
	if result.TestsRun == 0 {
		return 0.0
	}
	
	// 기본 통과율
	passRate := float64(result.TestsPassed) / float64(result.TestsRun) * 100
	
	// 성능 메트릭 점수
	performanceScore := 100.0
	
	// 응답 시간 점수 (100ms 기준)
	if result.PerformanceMetrics.AvgResponseTime > 100*time.Millisecond {
		penalty := float64(result.PerformanceMetrics.AvgResponseTime.Milliseconds()) / 10.0
		performanceScore -= penalty
	}
	
	// 에러율 점수 (1% 기준)
	if result.PerformanceMetrics.ErrorRate > 1.0 {
		performanceScore -= result.PerformanceMetrics.ErrorRate * 10
	}
	
	// 처리량 점수 (500 RPS 기준)
	if result.PerformanceMetrics.Throughput < 500 {
		penalty := (500 - result.PerformanceMetrics.Throughput) / 10.0
		performanceScore -= penalty
	}
	
	// 동시성/일관성 문제 점수
	concurrencyPenalty := float64(result.PerformanceMetrics.ConcurrencyIssues) * 5.0
	consistencyPenalty := float64(result.PerformanceMetrics.ConsistencyErrors) * 8.0
	
	performanceScore -= concurrencyPenalty + consistencyPenalty
	
	// 최종 점수 (통과율 70% + 성능 점수 30%)
	finalScore := (passRate * 0.7) + (performanceScore * 0.3)
	
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}
	
	return finalScore
}

// filterIssuesBySeverity 심각도별 문제 필터링
func (pa *PerformanceAnalyzer) filterIssuesBySeverity(issues []BottleneckIssue, severity string) []BottleneckIssue {
	var filtered []BottleneckIssue
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	
	// 우선순위별 정렬
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Priority == filtered[j].Priority {
			return filtered[i].Impact > filtered[j].Impact
		}
		return filtered[i].Priority > filtered[j].Priority
	})
	
	return filtered
}

// assessSystemHealth 시스템 상태 평가
func (pa *PerformanceAnalyzer) assessSystemHealth(overallScore float64, criticalIssues int) string {
	if criticalIssues > 0 {
		return "🚨 CRITICAL - Immediate attention required"
	}
	
	if overallScore >= 90 {
		return "🌟 EXCELLENT - System performing optimally"
	} else if overallScore >= 80 {
		return "✨ GOOD - System performing well with minor issues"
	} else if overallScore >= 70 {
		return "👍 FAIR - System functional but needs improvement"
	} else if overallScore >= 60 {
		return "⚠️ POOR - Significant performance issues detected"
	} else {
		return "💥 FAILING - System requires major remediation"
	}
}

// assessReadiness 준비도 평가
func (pa *PerformanceAnalyzer) assessReadiness(overallScore float64, criticalIssues int) string {
	if criticalIssues > 0 {
		return "🛑 NOT READY - Critical issues must be resolved before production"
	}
	
	if overallScore >= 85 {
		return "✅ PRODUCTION READY - System meets production standards"
	} else if overallScore >= 75 {
		return "🎯 ALMOST READY - Minor optimizations recommended"
	} else if overallScore >= 65 {
		return "🔧 NEEDS WORK - Performance improvements required"
	} else {
		return "🚧 MAJOR WORK NEEDED - Extensive optimization required"
	}
}

// generateTopRecommendations 최상위 권고사항 생성
func (pa *PerformanceAnalyzer) generateTopRecommendations(issues []BottleneckIssue) []string {
	var recommendations []string
	
	// 고빈도 문제 분석
	componentIssues := make(map[string]int)
	typeIssues := make(map[string]int)
	
	for _, issue := range issues {
		componentIssues[issue.Component]++
		typeIssues[issue.Type]++
	}
	
	// 컴포넌트별 권고사항
	if componentIssues["Redis"] >= 3 {
		recommendations = append(recommendations, "Optimize Redis configuration: connection pooling, memory management, and cache strategies")
	}
	
	if componentIssues["DynamoDB"] >= 2 {
		recommendations = append(recommendations, "Review DynamoDB table design: partition keys, indexes, and read/write capacity")
	}
	
	if componentIssues["gRPC"] >= 2 {
		recommendations = append(recommendations, "Improve gRPC performance: streaming optimization, connection reuse, and payload compression")
	}
	
	// 타입별 권고사항
	if typeIssues["latency"] >= 3 {
		recommendations = append(recommendations, "Address latency issues: implement caching, optimize database queries, and reduce network calls")
	}
	
	if typeIssues["throughput"] >= 2 {
		recommendations = append(recommendations, "Improve throughput: horizontal scaling, connection pooling, and async processing")
	}
	
	if typeIssues["memory"] >= 2 {
		recommendations = append(recommendations, "Optimize memory usage: implement object pooling, reduce allocations, and tune garbage collection")
	}
	
	// 일반적인 권고사항
	if len(issues) > 10 {
		recommendations = append(recommendations, "Conduct comprehensive system architecture review and implement monitoring")
	}
	
	// 최대 5개 권고사항 반환
	if len(recommendations) > 5 {
		recommendations = recommendations[:5]
	}
	
	return recommendations
}

// GenerateDetailedReport 상세 보고서 생성
func (pa *PerformanceAnalyzer) GenerateDetailedReport(report *AnalysisReport) string {
	output := ""
	
	output += "📊 COMPREHENSIVE PERFORMANCE ANALYSIS REPORT\n"
	output += "========================================================\n\n"
	
	// 실행 요약
	output += "⏱️ EXECUTION SUMMARY\n"
	output += "----------------------------------------\n"
	output += fmt.Sprintf("Total Duration: %v\n", report.TotalDuration)
	output += fmt.Sprintf("Overall Score: %.1f/100\n", report.OverallScore)
	output += fmt.Sprintf("System Health: %s\n", report.SystemHealth)
	output += fmt.Sprintf("Production Readiness: %s\n\n", report.ReadinessAssessment)
	
	// 스위트별 성능 순위
	output += "🏆 PERFORMANCE RANKING BY TEST SUITE\n"
	output += "----------------------------------------\n"
	
	// 성능 순으로 정렬
	type suiteScore struct {
		name  string
		score float64
	}
	var scores []suiteScore
	for name, score := range report.PerformanceRanking {
		scores = append(scores, suiteScore{name, score})
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	
	for i, suite := range scores {
		rank := i + 1
		status := "🟢"
		if suite.score < 70 {
			status = "🔴"
		} else if suite.score < 85 {
			status = "🟡"
		}
		output += fmt.Sprintf("%d. %s %s (%.1f/100)\n", rank, status, suite.name, suite.score)
	}
	output += "\n"
	
	// 문제점 요약
	output += "⚠️ ISSUES SUMMARY\n"
	output += "----------------------------------------\n"
	output += fmt.Sprintf("🔴 Critical Issues: %d\n", len(report.CriticalIssues))
	output += fmt.Sprintf("🟡 Major Issues: %d\n", len(report.MajorIssues))
	output += fmt.Sprintf("🟢 Minor Issues: %d\n\n", len(report.MinorIssues))
	
	// 중요 문제점 상세
	if len(report.CriticalIssues) > 0 {
		output += "🚨 CRITICAL ISSUES (IMMEDIATE ACTION REQUIRED)\n"
		output += "----------------------------------------\n"
		for i, issue := range report.CriticalIssues {
			if i >= 5 { break } // 최대 5개만 표시
			output += fmt.Sprintf("%d. [%s] %s\n", i+1, issue.Component, issue.Description)
			output += fmt.Sprintf("   Impact: %.1f%% | Priority: %d | Solution: %s\n\n", issue.Impact, issue.Priority, issue.Solution)
		}
	}
	
	// 주요 권고사항
	if len(report.TopRecommendations) > 0 {
		output += "💡 TOP RECOMMENDATIONS\n"
		output += "----------------------------------------\n"
		for i, rec := range report.TopRecommendations {
			output += fmt.Sprintf("%d. %s\n", i+1, rec)
		}
		output += "\n"
	}
	
	// 다음 단계
	output += "🔮 NEXT STEPS\n"
	output += "----------------------------------------\n"
	if len(report.CriticalIssues) > 0 {
		output += "1. 🚨 IMMEDIATE: Address all critical issues before proceeding\n"
		output += "2. 🔍 INVESTIGATE: Perform deep-dive analysis on failing components\n"
		output += "3. 🔄 RE-TEST: Run targeted tests after fixes\n"
	} else if report.OverallScore < 75 {
		output += "1. 🔧 OPTIMIZE: Focus on major performance bottlenecks\n"
		output += "2. 📊 MONITOR: Implement continuous performance monitoring\n"
		output += "3. 🧪 VALIDATE: Run regression tests after optimization\n"
	} else {
		output += "1. ✅ FINALIZE: Complete final optimization and documentation\n"
		output += "2. 🚀 DEPLOY: Proceed with production deployment preparation\n"
		output += "3. 📈 MONITOR: Set up production monitoring and alerting\n"
	}
	
	output += "\n========================================================\n"
	output += fmt.Sprintf("Report generated at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	
	return output
}