package service_test

import (
	"SmartRun/internal/broker"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/ports/outgoing/importer"
	"SmartRun/internal/usecase/service"
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

func ptr[T any](a T) *T {
	return &a
}

// workoutMocks группирует все моки, необходимые сервису WorkoutService
type workoutMocks struct {
	workoutRepo      *MockWorkoutRepo
	dailyMetricsRepo *MockDailyMetricsRepo
	userRepo         *MockUserRepo
	parser           *MockFitParser
	txManager        *MockTxManager
	publisher        *MockPublisher
}

func newWorkoutMocks() workoutMocks {
	return workoutMocks{
		workoutRepo:      new(MockWorkoutRepo),
		dailyMetricsRepo: new(MockDailyMetricsRepo),
		userRepo:         new(MockUserRepo),
		parser:           new(MockFitParser),
		txManager:        new(MockTxManager),
		publisher:        new(MockPublisher),
	}
}

func (m workoutMocks) newService() service.WorkoutService {
	return service.NewWorkoutService(
		m.workoutRepo,
		m.dailyMetricsRepo,
		m.userRepo,
		m.parser,
		validator.New(),
		m.txManager,
		m.publisher,
	)
}

func (m workoutMocks) assertExpectations(t *testing.T) {
	m.workoutRepo.AssertExpectations(t)
	m.dailyMetricsRepo.AssertExpectations(t)
	m.userRepo.AssertExpectations(t)
	m.parser.AssertExpectations(t)
	m.txManager.AssertExpectations(t)
	m.publisher.AssertExpectations(t)
}

func sampleUser() *model.User {
	return &model.User{
		ID:            1,
		WeightKg:      70,
		RestingHR:     50,
		MaxHR:         190,
		ThresholdPace: 4.5,
	}
}

// =============================================================================
// 1. Create
// =============================================================================

func TestWorkoutService_Create(t *testing.T) {
	tests := []struct {
		name        string
		userID      int64
		req         dto.CreateRequest
		setupMocks  func(m workoutMocks)
		wantErr     bool
		expectedErr error
		checkResult func(t *testing.T, w *model.Workouts)
	}{
		{
			name:   "Success: Normal workout",
			userID: 1,
			req: dto.CreateRequest{
				Date:         "2026-10-14",
				Distance:     10.0,
				Duration:     3000, // 50 мин
				TypeActivity: "running",
				AvgHR:        ptr(150),
				MaxHR:        ptr(175),
				Notes:        ptr("Good run"),
				Shoes:        ptr("Pegasus 40"),
				HRZones: &dto.HRZonesRequest{
					Zone1Seconds: 300,
					Zone2Seconds: 1500,
					Zone3Seconds: 900,
					Zone4Seconds: 300,
					Zone5Seconds: 0,
				},
			},
			setupMocks: func(m workoutMocks) {
				m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
				m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()

				m.workoutRepo.On("Create", mock.Anything, mock.MatchedBy(func(w *model.Workouts) bool {
					return w.UserID == 1 && w.Distance == 10.0 && !w.IsAnomalous &&
						w.Notes == "Good run" && w.Shoes == "Pegasus 40" && len(w.TimeInHrZone) == 5
				})).Return(nil).Once()

				m.publisher.On("PublishMetricsRecalculation", mock.Anything, mock.MatchedBy(func(e broker.MetricsRecalculationEvent) bool {
					return e.UserID == 1
				})).Return(nil).Once()
			},
			wantErr: false,
			checkResult: func(t *testing.T, w *model.Workouts) {
				assert.Equal(t, int64(1), w.UserID)
				assert.False(t, w.IsAnomalous)
				require.NotNil(t, w.Calories)
				assert.Greater(t, *w.Calories, 0)
			},
		},
		{
			name:   "Success: Anomalous workout saves but skips queue publishing",
			userID: 1,
			req: dto.CreateRequest{
				Date:         "2026-10-14",
				Distance:     5.0,
				Duration:     1200,
				TypeActivity: "running",
				AvgHR:        ptr(245),
				MaxHR:        ptr(250),
			},
			setupMocks: func(m workoutMocks) {
				m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
				m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()

				m.workoutRepo.On("Create", mock.Anything, mock.MatchedBy(func(w *model.Workouts) bool {
					return w.UserID == 1 && w.IsAnomalous == true
				})).Return(nil).Once()
			},
			wantErr: false,
			checkResult: func(t *testing.T, w *model.Workouts) {
				assert.True(t, w.IsAnomalous)
				assert.NotNil(t, w.AnomalyReason)
			},
		},
		{
			name:   "Error: Distance <= 0",
			userID: 1,
			req: dto.CreateRequest{
				Date:         "2026-10-14",
				Distance:     0,
				Duration:     1500,
				TypeActivity: "running",
			},
			setupMocks: func(m workoutMocks) {},
			wantErr:    true,
		},
		{
			name:   "Error: Invalid date format",
			userID: 1,
			req: dto.CreateRequest{
				Date:         "invalid-date",
				Distance:     5.0,
				Duration:     1500,
				TypeActivity: "running",
			},
			setupMocks:  func(m workoutMocks) {},
			wantErr:     true,
			expectedErr: my_errors.ErrInvalidData,
		},
		{
			name:   "Error: Date before 1970",
			userID: 1,
			req: dto.CreateRequest{
				Date:         "1969-12-31",
				Distance:     5.0,
				Duration:     1500,
				TypeActivity: "running",
			},
			setupMocks: func(m workoutMocks) {},
			wantErr:    true,
		},
		{
			name:   "Error: User not found",
			userID: 99,
			req: dto.CreateRequest{
				Date:         "2026-10-14",
				Distance:     5.0,
				Duration:     1500,
				TypeActivity: "running",
			},
			setupMocks: func(m workoutMocks) {
				m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
				m.userRepo.On("GetUserByID", mock.Anything, int64(99)).Return(nil, errors.New("not found")).Once()
			},
			wantErr:     true,
			expectedErr: my_errors.ErrUserNotFound,
		},
		{
			name:   "Error: Repo failure aborts transaction and skips publishing",
			userID: 1,
			req: dto.CreateRequest{
				Date:         "2026-10-14",
				Distance:     5.0,
				Duration:     1500,
				TypeActivity: "running",
			},
			setupMocks: func(m workoutMocks) {
				m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
				m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
				m.workoutRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db disk full")).Once()
			},
			wantErr: true,
			checkResult: func(t *testing.T, w *model.Workouts) {
				assert.Nil(t, w)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newWorkoutMocks()
			tt.setupMocks(m)
			svc := m.newService()

			res, err := svc.Create(context.Background(), tt.userID, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
			}
			if tt.checkResult != nil {
				tt.checkResult(t, res)
			}
			m.assertExpectations(t)
		})
	}
}

// =============================================================================
// 2. GetByID
// =============================================================================

func TestWorkoutService_GetByID(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := newWorkoutMocks()
		expected := &model.Workouts{ID: 10, UserID: 1, Distance: 5.0}
		m.workoutRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.GetByID(context.Background(), 10, 1)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error: Not Found", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetByID", mock.Anything, int64(99), int64(1)).
			Return(nil, my_errors.ErrWorkoutNotFound).Once()

		svc := m.newService()
		res, err := svc.GetByID(context.Background(), 99, 1)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, my_errors.ErrWorkoutNotFound)
		m.assertExpectations(t)
	})
}

// =============================================================================
// 3. GetAllByID
// =============================================================================

func TestWorkoutService_GetAllByID(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := newWorkoutMocks()
		filter := dto.WorkoutFilter{UserID: 1}
		expected := []model.Workouts{
			{ID: 1, UserID: 1, Distance: 5.0},
			{ID: 2, UserID: 1, Distance: 10.0},
		}
		m.workoutRepo.On("GetAllByUserID", mock.Anything, filter).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.GetAllByID(context.Background(), filter)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newWorkoutMocks()
		filter := dto.WorkoutFilter{UserID: 1}
		m.workoutRepo.On("GetAllByUserID", mock.Anything, filter).Return(nil, errors.New("db error")).Once()

		svc := m.newService()
		res, err := svc.GetAllByID(context.Background(), filter)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

// =============================================================================
// 4. Delete
// =============================================================================

func TestWorkoutService_Delete(t *testing.T) {
	workoutDate := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	t.Run("Success: Deletes and publishes recalculation event", func(t *testing.T) {
		m := newWorkoutMocks()
		workout := &model.Workouts{ID: 5, UserID: 1, Date: workoutDate}

		m.workoutRepo.On("GetByID", mock.Anything, int64(5), int64(1)).Return(workout, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.workoutRepo.On("DeleteWorkout", mock.Anything, int64(5), int64(1)).Return(nil).Once()
		m.publisher.On("PublishMetricsRecalculation", mock.Anything, broker.MetricsRecalculationEvent{
			UserID: 1,
			Date:   workoutDate,
		}).Return(nil).Once()

		svc := m.newService()
		err := svc.Delete(context.Background(), 5, 1)
		require.NoError(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error: Workout not found before deletion", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetByID", mock.Anything, int64(99), int64(1)).Return(nil, my_errors.ErrWorkoutNotFound).Once()

		svc := m.newService()
		err := svc.Delete(context.Background(), 99, 1)
		require.Error(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error: Tx failure on DeleteWorkout", func(t *testing.T) {
		m := newWorkoutMocks()
		workout := &model.Workouts{ID: 5, UserID: 1, Date: workoutDate}

		m.workoutRepo.On("GetByID", mock.Anything, int64(5), int64(1)).Return(workout, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.workoutRepo.On("DeleteWorkout", mock.Anything, int64(5), int64(1)).Return(errors.New("db error")).Once()

		svc := m.newService()
		err := svc.Delete(context.Background(), 5, 1)
		require.Error(t, err)
		m.assertExpectations(t)
	})
}

// =============================================================================
// 5. Update
// =============================================================================

func TestWorkoutService_Update(t *testing.T) {
	origDate := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	t.Run("Success PATCH: Partial update recalculates and publishes", func(t *testing.T) {
		m := newWorkoutMocks()
		origWorkout := &model.Workouts{
			ID:           10,
			UserID:       1,
			Date:         origDate,
			Distance:     5.0,
			Duration:     1500,
			TypeActivity: "running",
		}

		req := dto.UpdateRequest{
			Distance: 8.0, // обновляем только дистанцию
		}

		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(origWorkout, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.workoutRepo.On("Update", mock.Anything, mock.MatchedBy(func(w *model.Workouts) bool {
			return w.ID == 10 && w.Distance == 8.0
		})).Return(nil).Once()
		m.publisher.On("PublishMetricsRecalculation", mock.Anything, broker.MetricsRecalculationEvent{
			UserID: 1,
			Date:   origDate,
		}).Return(nil).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 1, 10, req, false)
		require.NoError(t, err)
		assert.Equal(t, 8.0, res.Distance)
		m.assertExpectations(t)
	})

	t.Run("Success PUT: Full replace", func(t *testing.T) {
		m := newWorkoutMocks()
		origWorkout := &model.Workouts{
			ID:           10,
			UserID:       1,
			Date:         origDate,
			Distance:     5.0,
			Duration:     1500,
			TypeActivity: "running",
		}

		req := dto.UpdateRequest{
			Date:         "2026-10-14",
			Distance:     10.0,
			Duration:     3000,
			TypeActivity: "running",
		}

		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(origWorkout, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.workoutRepo.On("Update", mock.Anything, mock.MatchedBy(func(w *model.Workouts) bool {
			return w.ID == 10 && w.Distance == 10.0 && w.Duration == 3000
		})).Return(nil).Once()
		m.publisher.On("PublishMetricsRecalculation", mock.Anything, broker.MetricsRecalculationEvent{
			UserID: 1,
			Date:   origDate,
		}).Return(nil).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 1, 10, req, true)
		require.NoError(t, err)
		assert.Equal(t, 10.0, res.Distance)
		m.assertExpectations(t)
	})

	t.Run("Error PUT: Missing required fields", func(t *testing.T) {
		m := newWorkoutMocks()
		origWorkout := &model.Workouts{ID: 10, UserID: 1, Date: origDate}
		req := dto.UpdateRequest{
			Distance: 10.0, // нет Date, Duration, TypeActivity
		}

		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(origWorkout, nil).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 1, 10, req, true)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "PUT requires all required fields")
		m.assertExpectations(t)
	})

	t.Run("Success: Date changed triggers recalculation for BOTH dates", func(t *testing.T) {
		m := newWorkoutMocks()
		origWorkout := &model.Workouts{
			ID:           10,
			UserID:       1,
			Date:         origDate,
			Distance:     5.0,
			Duration:     1500,
			TypeActivity: "running",
		}

		newDateStr := "2026-10-15"
		newDate := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
		req := dto.UpdateRequest{
			Date: newDateStr,
		}

		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(origWorkout, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.workoutRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

		// Публикация для новой даты:
		m.publisher.On("PublishMetricsRecalculation", mock.Anything, broker.MetricsRecalculationEvent{
			UserID: 1,
			Date:   newDate,
		}).Return(nil).Once()

		// Публикация для старой даты (так как дата изменилась!):
		m.publisher.On("PublishMetricsRecalculation", mock.Anything, broker.MetricsRecalculationEvent{
			UserID: 1,
			Date:   origDate,
		}).Return(nil).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 1, 10, req, false)
		require.NoError(t, err)
		assert.Equal(t, newDate, res.Date)
		m.assertExpectations(t)
	})

	t.Run("Success: With HR zones calculates training effect", func(t *testing.T) {
		m := newWorkoutMocks()
		origWorkout := &model.Workouts{
			ID:           10,
			UserID:       1,
			Date:         origDate,
			Distance:     10.0,
			Duration:     3000,
			TypeActivity: "running",
		}

		req := dto.UpdateRequest{
			HRZones: &dto.HRZonesRequest{
				Zone1Seconds: 300,
				Zone2Seconds: 1500,
				Zone3Seconds: 900,
				Zone4Seconds: 300,
				Zone5Seconds: 0,
				Zone6Seconds: 0,
			},
		}

		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("GetByID", mock.Anything, int64(10), int64(1)).Return(origWorkout, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.workoutRepo.On("Update", mock.Anything, mock.MatchedBy(func(w *model.Workouts) bool {
			return w.AerobicTrainingEffect != nil && w.PrimaryTrainingFocus != ""
		})).Return(nil).Once()
		m.publisher.On("PublishMetricsRecalculation", mock.Anything, mock.Anything).Return(nil).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 1, 10, req, false)
		require.NoError(t, err)
		assert.NotNil(t, res.AerobicTrainingEffect)
		m.assertExpectations(t)
	})

	t.Run("Error: User not found", func(t *testing.T) {
		m := newWorkoutMocks()
		m.userRepo.On("GetUserByID", mock.Anything, int64(99)).Return(nil, errors.New("not found")).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 99, 10, dto.UpdateRequest{}, false)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, my_errors.ErrUserNotFound)
		m.assertExpectations(t)
	})

	t.Run("Error: Workout not found", func(t *testing.T) {
		m := newWorkoutMocks()
		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("GetByID", mock.Anything, int64(99), int64(1)).Return(nil, errors.New("not found")).Once()

		svc := m.newService()
		res, err := svc.Update(context.Background(), 1, 99, dto.UpdateRequest{}, false)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

// =============================================================================
// 6. GetMonthlyHistory
// =============================================================================

func TestWorkoutService_GetMonthlyHistory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := newWorkoutMocks()
		monthDate := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		expected := []model.WorkoutMonthHistory{
			{
				Month:         monthDate,
				TotalDistance: 120.5,
				WorkoutsCount: 12,
			},
		}
		m.workoutRepo.On("GetMonthlyHistory", mock.Anything, int64(1), 6, 0).Return(expected, nil).Once()

		svc := m.newService()
		res, err := svc.GetMonthlyHistory(context.Background(), 1, 6, 0)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
		m.assertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetMonthlyHistory", mock.Anything, int64(1), 6, 0).
			Return(nil, errors.New("query error")).Once()

		svc := m.newService()
		res, err := svc.GetMonthlyHistory(context.Background(), 1, 6, 0)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})
}

// =============================================================================
// 7. UploadFit
// =============================================================================

func TestWorkoutService_UploadFit(t *testing.T) {
	fitDate := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	rawBytes := []byte("fake-fit-binary-content")

	t.Run("Success: Parses and creates workout", func(t *testing.T) {
		m := newWorkoutMocks()
		act := &importer.ActivityData{
			StartTime:    fitDate,
			Distance:     10.0,
			Duration:     3000,
			TypeActivity: "running",
			AvgHR:        ptr(150),
			MaxHR:        ptr(175),
		}

		m.parser.On("Parse", mock.Anything, rawBytes).Return(act, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()

		m.workoutRepo.On("Create", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				w := args.Get(1).(*model.Workouts)
				w.ID = 42 // имитируем генерацию ID базой
			}).Return(nil).Once()

		m.publisher.On("PublishMetricsRecalculation", mock.Anything, broker.MetricsRecalculationEvent{
			UserID: 1,
			Date:   fitDate,
		}).Return(nil).Once()

		svc := m.newService()
		res, err := svc.UploadFit(context.Background(), 1, rawBytes)
		require.NoError(t, err)
		assert.Equal(t, int64(42), res.ID)
		assert.False(t, res.IsAnomalous)
		m.assertExpectations(t)
	})

	t.Run("Success: Anomalous activity saves but skips publishing", func(t *testing.T) {
		m := newWorkoutMocks()
		act := &importer.ActivityData{
			StartTime:    fitDate,
			Distance:     5.0,
			Duration:     1200,
			TypeActivity: "running",
			AvgHR:        ptr(245),
			MaxHR:        ptr(250),
		}

		m.parser.On("Parse", mock.Anything, rawBytes).Return(act, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()

		m.workoutRepo.On("Create", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				w := args.Get(1).(*model.Workouts)
				w.ID = 43
			}).Return(nil).Once()

		svc := m.newService()
		res, err := svc.UploadFit(context.Background(), 1, rawBytes)
		require.NoError(t, err)
		assert.True(t, res.IsAnomalous)
		m.assertExpectations(t)
	})

	t.Run("Error: Parser error", func(t *testing.T) {
		m := newWorkoutMocks()
		m.parser.On("Parse", mock.Anything, rawBytes).Return(nil, errors.New("corrupt fit file")).Once()

		svc := m.newService()
		res, err := svc.UploadFit(context.Background(), 1, rawBytes)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "parse FIT")
		m.assertExpectations(t)
	})

	t.Run("Error: Start time before 1970", func(t *testing.T) {
		m := newWorkoutMocks()
		act := &importer.ActivityData{
			StartTime: time.Date(1969, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		m.parser.On("Parse", mock.Anything, rawBytes).Return(act, nil).Once()

		svc := m.newService()
		res, err := svc.UploadFit(context.Background(), 1, rawBytes)
		require.Error(t, err)
		assert.Nil(t, res)
		m.assertExpectations(t)
	})

	t.Run("Error: Workout was not persisted (ID <= 0)", func(t *testing.T) {
		m := newWorkoutMocks()
		act := &importer.ActivityData{
			StartTime:    fitDate,
			Distance:     5.0,
			Duration:     1500,
			TypeActivity: "running",
		}

		m.parser.On("Parse", mock.Anything, rawBytes).Return(act, nil).Once()
		m.txManager.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Once()
		m.userRepo.On("GetUserByID", mock.Anything, int64(1)).Return(sampleUser(), nil).Once()
		m.workoutRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once() // ID не выставлен (ID == 0)

		svc := m.newService()
		res, err := svc.UploadFit(context.Background(), 1, rawBytes)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "workout was not persisted")
		m.assertExpectations(t)
	})
}

// =============================================================================
// 8. RecalculateDailyMetricsForDate
// =============================================================================

func TestWorkoutService_RecalculateDailyMetricsForDate(t *testing.T) {
	targetDate := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	normalizedDate := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	yesterday := normalizedDate.AddDate(0, 0, -1)

	t.Run("Success: Normal recalculation with previous metrics and merge", func(t *testing.T) {
		m := newWorkoutMocks()
		workouts := []model.Workouts{
			{
				ID:                  1,
				UserID:              1,
				Date:                normalizedDate,
				Distance:            10.0,
				Duration:            3000,
				Calories:            ptr(750),
				TrainingStressScore: ptr(65.0),
				IsAnomalous:         false,
			},
		}

		prevMetrics := &model.DailyMetric{
			UserID:     1,
			Date:       yesterday,
			CTL:        50.0,
			ATL:        45.0,
			StreakDays: 3,
		}

		existingMetric := &model.DailyMetric{
			ID:             100,
			UserID:         1,
			Date:           normalizedDate,
			SleepScore:     88, // пользователь ввел вручную
			StressAvg:      25,
			BodyBatteryAvg: 70,
		}

		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).Return(workouts, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), yesterday).Return(prevMetrics, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), normalizedDate).Return(existingMetric, nil).Once()

		m.dailyMetricsRepo.On("UpdateOrCreate", mock.Anything, mock.MatchedBy(func(dm *model.DailyMetric) bool {
			// Проверяем, что ID сохранился и ручной ввод пользователя (SleepScore, StressAvg) не стерт:
			return dm.ID == 100 && dm.SleepScore == 88 && dm.StressAvg == 25
		})).Return(nil).Once()

		svc := m.newService()
		err := svc.RecalculateDailyMetricsForDate(context.Background(), 1, targetDate)
		require.NoError(t, err)
		m.assertExpectations(t)
	})

	t.Run("Success: First day for user (previous metrics nil)", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).Return([]model.Workouts{}, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), yesterday).Return(nil, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), normalizedDate).Return(nil, nil).Once()
		m.dailyMetricsRepo.On("UpdateOrCreate", mock.Anything, mock.Anything).Return(nil).Once()

		svc := m.newService()
		err := svc.RecalculateDailyMetricsForDate(context.Background(), 1, targetDate)
		require.NoError(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error: GetAllByUserID failure", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).Return(nil, errors.New("db error")).Once()

		svc := m.newService()
		err := svc.RecalculateDailyMetricsForDate(context.Background(), 1, targetDate)
		require.Error(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error: GetByUserIDAndDate yesterday failure", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).Return([]model.Workouts{}, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), yesterday).
			Return(nil, errors.New("db timeout")).Once()

		svc := m.newService()
		err := svc.RecalculateDailyMetricsForDate(context.Background(), 1, targetDate)
		require.Error(t, err)
		m.assertExpectations(t)
	})

	t.Run("Error: UpdateOrCreate failure", func(t *testing.T) {
		m := newWorkoutMocks()
		m.workoutRepo.On("GetAllByUserID", mock.Anything, mock.Anything).Return([]model.Workouts{}, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), yesterday).Return(nil, nil).Once()
		m.dailyMetricsRepo.On("GetByUserIDAndDate", mock.Anything, int64(1), normalizedDate).Return(nil, nil).Once()
		m.dailyMetricsRepo.On("UpdateOrCreate", mock.Anything, mock.Anything).Return(errors.New("db insert error")).Once()

		svc := m.newService()
		err := svc.RecalculateDailyMetricsForDate(context.Background(), 1, targetDate)
		require.Error(t, err)
		m.assertExpectations(t)
	})
}

