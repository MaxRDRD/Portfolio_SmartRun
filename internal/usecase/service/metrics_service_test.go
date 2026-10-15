package service_test

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/internal/usecase/service/mocks"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MetricsService Tests (использующие автосгенерированный мок mocks.MetricsRepository)
// =============================================================================

func TestMetricService_CreateMetrics(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		input := model.Metrics{UserID: 1, TotalWorkouts: 5, TotalDistance: 50.0}
		expected := &model.Metrics{ID: 10, UserID: 1, TotalWorkouts: 5, TotalDistance: 50.0}

		mockRepo.On("CreateMetrics", mock.Anything, input).Return(expected, nil).Once()

		res, err := svc.CreateMetrics(context.Background(), input)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		input := model.Metrics{UserID: 1}
		mockRepo.On("CreateMetrics", mock.Anything, input).Return(nil, errors.New("db insert failure")).Once()

		res, err := svc.CreateMetrics(context.Background(), input)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "create metrics")
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricService_GetMetrics(t *testing.T) {
	t.Run("Success: Normalizes default dates and returns metrics", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		filter := dto.MetricsFilter{UserID: 1}
		expected := &model.Metrics{ID: 10, UserID: 1, TotalWorkouts: 12}

		// Сервис нормализует filter.From и filter.To (по умолчанию за 30 дней)
		mockRepo.On("GetMetricsByID", mock.Anything, mock.MatchedBy(func(f dto.MetricsFilter) bool {
			return f.UserID == 1 && f.From != nil && f.To != nil && f.From.Before(*f.To)
		})).Return(expected, nil).Once()

		res, err := svc.GetMetrics(context.Background(), filter)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Success: Custom date range where From is after To gets clamped", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		from := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
		filter := dto.MetricsFilter{UserID: 1, From: &from, To: &to}

		mockRepo.On("GetMetricsByID", mock.Anything, mock.MatchedBy(func(f dto.MetricsFilter) bool {
			return f.UserID == 1 && f.From.Equal(*f.To)
		})).Return(&model.Metrics{UserID: 1}, nil).Once()

		res, err := svc.GetMetrics(context.Background(), filter)
		require.NoError(t, err)
		assert.NotNil(t, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error: Not Found", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		mockRepo.On("GetMetricsByID", mock.Anything, mock.Anything).
			Return(nil, my_errors.ErrMetricNotFound).Once()

		res, err := svc.GetMetrics(context.Background(), dto.MetricsFilter{UserID: 1})
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, my_errors.ErrMetricNotFound)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricService_GetAllMetrics(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		expected := &model.Metrics{ID: 10, UserID: 1, TotalDistance: 100.0}
		mockRepo.On("GetAllMetricsByID", mock.Anything, mock.Anything).Return(expected, nil).Once()

		res, err := svc.GetAllMetrics(context.Background(), dto.MetricsFilter{UserID: 1})
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error: Not Found", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		mockRepo.On("GetAllMetricsByID", mock.Anything, mock.Anything).
			Return(nil, my_errors.ErrMetricNotFound).Once()

		res, err := svc.GetAllMetrics(context.Background(), dto.MetricsFilter{UserID: 1})
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, my_errors.ErrMetricNotFound)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricService_UpdateMetrics(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		input := model.Metrics{ID: 5, UserID: 1, TotalDistance: 75.0}
		expected := &model.Metrics{ID: 5, UserID: 1, TotalDistance: 75.0}

		mockRepo.On("UpdateMetrics", mock.Anything, input).Return(expected, nil).Once()

		res, err := svc.UpdateMetrics(context.Background(), input)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		input := model.Metrics{ID: 5, UserID: 1}
		mockRepo.On("UpdateMetrics", mock.Anything, input).Return(nil, errors.New("update failure")).Once()

		res, err := svc.UpdateMetrics(context.Background(), input)
		require.Error(t, err)
		assert.Nil(t, res)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricService_DeleteMetrics(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		mockRepo.On("DeleteMetrics", mock.Anything, int64(10), int64(1)).Return(nil).Once()

		err := svc.DeleteMetrics(context.Background(), 10, 1)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo := new(mocks.MetricsRepository)
		svc := service.NewMetricsService(mockRepo, validator.New())

		mockRepo.On("DeleteMetrics", mock.Anything, int64(99), int64(1)).
			Return(errors.New("db delete error")).Once()

		err := svc.DeleteMetrics(context.Background(), 99, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete metrics")
		mockRepo.AssertExpectations(t)
	})
}
