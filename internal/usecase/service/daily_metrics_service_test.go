package service_test

import (
	"SmartRun/internal/broker"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type dailyMetricMocks struct {
	dailyRepo   *MockDailyMetricsRepo
	workoutRepo *MockWorkoutRepo
	txManager   *MockTxManager
	publisher   *MockPublisher
}

func newDailyMetricMocks() dailyMetricMocks {
	return dailyMetricMocks{
		dailyRepo:   new(MockDailyMetricsRepo),
		workoutRepo: new(MockWorkoutRepo),
		txManager:   new(MockTxManager),
		publisher:   new(MockPublisher),
	}
}

func (m dailyMetricMocks) newService() service.DailyMetricService {
	return service.NewDailyMetricService(
		m.dailyRepo,
		m.workoutRepo,
		validator.New(),
		m.txManager,
		m.publisher,
	)
}

func (m dailyMetricMocks) assertExpectations(t *testing.T) {
	m.dailyRepo.AssertExpectations(t)
	m.workoutRepo.AssertExpectations(t)
	m.txManager.AssertExpectations(t)
	m.publisher.AssertExpectations(t)
}

// =============================================================================
// DailyMetricService Tests
// =============================================================================

func TestDailyMetricService_CreateDailyMetric(t *testing.T) {
	date := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	input := model.DailyMetric{UserID: 1, Date: date, SleepScore: 85}
	expected := &model.DailyMetric{ID: 10, UserID: 1, Date: date, SleepScore: 85}

	t.Run("Success", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("Create", mock.Anything, input).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.CreateDailyMetric(context.Background(), input)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("Create", mock.Anything, input).Return(nil, errors.New("db error")).Once()

		svc := m.newService()
		res, err := svc.CreateDailyMetric(context.Background(), input)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

func TestDailyMetricService_UpdateDailyMetric(t *testing.T) {
	anchorDate := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	existing := &model.DailyMetric{
		ID:             10,
		UserID:         1,
		Date:           anchorDate,
		SleepScore:     70,
		BodyBatteryAvg: 50,
	}

	req := dto.UpdateDailyMetricRequest{
		ID:             10,
		UserID:         1,
		Date:           anchorDate,
		SleepScore:     90, // обновленный сон
		SleepHours:     8.0,
		StressAvg:      20,
		BodyBatteryAvg: 80,
		Steps:          10000,
	}

	t.Run("Success: Fetches workouts, recalculates and updates", func(t *testing.T) {
		m := newDailyMetricMocks()

		m.dailyRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(existing, nil).Once()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.MatchedBy(func(f dto.WorkoutFilter) bool {
			return f.UserID == 1 && f.From != nil && f.To != nil
		})).Return([]model.Workouts{}, nil).Once()

		m.dailyRepo.On("Update", mock.Anything, mock.MatchedBy(func(dm model.DailyMetric) bool {
			return dm.ID == 10 && dm.UserID == 1 && dm.SleepScore == 90
		})).Return(&model.DailyMetric{ID: 10, UserID: 1, SleepScore: 90}, nil).Once()

		svc := m.newService()
		res, err := svc.UpdateDailyMetric(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 90, res.SleepScore)
		m.assertExpectations(t)
	})

	t.Run("Error: GetByID failure", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("GetByID", mock.Anything, int64(10), int64(1)).
			Return(nil, errors.New("not found")).Once()

		svc := m.newService()
		res, err := svc.UpdateDailyMetric(context.Background(), req)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})

	t.Run("Error: WorkoutRepo failure", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(existing, nil).Once()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).
			Return(nil, errors.New("workout db error")).Once()

		svc := m.newService()
		res, err := svc.UpdateDailyMetric(context.Background(), req)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})

	t.Run("Error: DailyRepo Update failure", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(existing, nil).Once()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).Return([]model.Workouts{}, nil).Once()
		m.dailyRepo.On("Update", mock.Anything, mock.Anything).
			Return(nil, errors.New("update error")).Once()

		svc := m.newService()
		res, err := svc.UpdateDailyMetric(context.Background(), req)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

func TestDailyMetricService_DeleteDailyMetric(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("Delete", mock.Anything, int64(10), int64(1)).Return(nil).Once()

		svc := m.newService()
		err := svc.DeleteDailyMetric(context.Background(), 10, 1)
		require.NoError(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("Delete", mock.Anything, int64(10), int64(1)).
			Return(errors.New("delete error")).Once()

		svc := m.newService()
		err := svc.DeleteDailyMetric(context.Background(), 10, 1)
		require.Error(t, err)
		m.assertExpectations(t)
	})
}

func TestDailyMetricService_GetDailyMetricByID(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := newDailyMetricMocks()
		expected := &model.DailyMetric{ID: 10, UserID: 1}
		m.dailyRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.GetDailyMetricByID(context.Background(), 10, 1)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("GetByID", mock.Anything, int64(10), int64(1)).
			Return(nil, errors.New("not found")).Once()

		svc := m.newService()
		res, err := svc.GetDailyMetricByID(context.Background(), 10, 1)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

func TestDailyMetricService_GetByUserIDAndDate(t *testing.T) {
	date := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	t.Run("Success", func(t *testing.T) {
		m := newDailyMetricMocks()
		expected := &model.DailyMetric{ID: 10, UserID: 1, Date: date}
		m.dailyRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), date).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.GetByUserIDAndDate(context.Background(), 1, date)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), date).
			Return(nil, errors.New("db error")).Once()

		svc := m.newService()
		res, err := svc.GetByUserIDAndDate(context.Background(), 1, date)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

func TestDailyMetricService_GetAllDailyMetrics(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := newDailyMetricMocks()
		expected := []model.DailyMetric{{ID: 1, UserID: 1}, {ID: 2, UserID: 1}}
		m.dailyRepo.On("GetAllByUserID", mock.Anything, int64(1)).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.GetAllDailyMetrics(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.dailyRepo.On("GetAllByUserID", mock.Anything, int64(1)).
			Return(nil, errors.New("db error")).Once()

		svc := m.newService()
		res, err := svc.GetAllDailyMetrics(context.Background(), 1)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

func TestDailyMetricService_TriggerAdviceGeneration(t *testing.T) {
	date := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	event := broker.MetricsRecalculationEvent{UserID: 1, Date: date}

	t.Run("Success", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.publisher.On("PublishAICoachEvent", mock.Anything, event).Return(nil).Once()

		svc := m.newService()
		err := svc.TriggerAdviceGeneration(context.Background(), 1, date)
		require.NoError(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newDailyMetricMocks()
		m.publisher.On("PublishAICoachEvent", mock.Anything, event).
			Return(errors.New("broker connection closed")).Once()

		svc := m.newService()
		err := svc.TriggerAdviceGeneration(context.Background(), 1, date)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TriggerAdviceGeneration")
		m.assertExpectations(t)
	})
}
