package http

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
)

// ExportToDailyMetricResponse экспортирует toDailyMetricResponse для тестирования из package http_test.
func ExportToDailyMetricResponse(metric *model.DailyMetric) dto.DailyMetricResponse {
	return toDailyMetricResponse(metric)
}

// ExportToMetricsResponse экспортирует toMetricsResponse для тестирования из package http_test.
func ExportToMetricsResponse(metric *model.Metrics) dto.MetricsResponse {
	return toMetricsResponse(metric)
}
