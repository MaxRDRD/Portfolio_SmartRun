package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/internal/repository/postgres"
	"SmartRun/internal/repository/postgres/testutil"
	workpostgr "SmartRun/internal/repository/postgres/workout"
	"SmartRun/pkg/apperrors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testDB      *testutil.PostgresTestDB
	workoutRepo repository.WorkoutRepository
	txManager   repository.TxManager
)

func TestMain(m *testing.M) {
	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var err error
	testDB, err = testutil.SetupTestPostgres(ctx)
	if err != nil {
		log.Fatalf("failed to setup test postgres container: %v", err)
	}

	workoutRepo = workpostgr.NewWorkoutRepository(testDB.Pool)
	txManager = postgres.NewTxManager(testDB.Pool)

	code := m.Run()

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	_ = testDB.Close(cleanupCtx)

	os.Exit(code)
}

func ptr[T any](v T) *T {
	return &v
}

func setupTest(t *testing.T) (context.Context, int64) {
	t.Helper()
	ctx := context.Background()

	err := testDB.TruncateTables(ctx)
	require.NoError(t, err, "failed to truncate tables")

	userID, err := testDB.CreateUser(ctx, "Test Runner", fmt.Sprintf("runner_%d@example.com", time.Now().UnixNano()))
	require.NoError(t, err, "failed to create test user")

	return ctx, userID
}

// -----------------------------------------------------------------------------
// 1. Create Workout Tests
// -----------------------------------------------------------------------------

func TestWorkoutRepository_Create(t *testing.T) {
	ctx, defaultUserID := setupTest(t)

	tests := []struct {
		name          string
		workout       func() *model.Workouts
		wantErr       bool
		checkResponse func(t *testing.T, w *model.Workouts, err error)
	}{
		{
			name: "successful create with minimal required fields",
			workout: func() *model.Workouts {
				return &model.Workouts{
					UserID:       defaultUserID,
					Date:         time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC),
					Distance:     5.25,
					Duration:     1800,
					Pace:         5.71,
					TypeActivity: "running",
				}
			},
			wantErr: false,
			checkResponse: func(t *testing.T, w *model.Workouts, err error) {
				require.NoError(t, err)
				assert.Greater(t, w.ID, int64(0))
				assert.False(t, w.CreatedAt.IsZero())

				saved, getErr := workoutRepo.GetByID(ctx, w.ID, defaultUserID)
				require.NoError(t, getErr)
				assert.Equal(t, w.ID, saved.ID)
				assert.Equal(t, defaultUserID, saved.UserID)
				assert.InDelta(t, 5.25, saved.Distance, 0.001)
				assert.Equal(t, 1800, saved.Duration)
				assert.InDelta(t, 5.71, saved.Pace, 0.001)
				assert.Equal(t, "running", saved.TypeActivity)
				assert.Empty(t, saved.TimeInHrZone)
				assert.False(t, saved.IsAnomalous)
			},
		},
		{
			name: "successful create with full fields, HR zones and anomaly details",
			workout: func() *model.Workouts {
				return &model.Workouts{
					UserID:                  defaultUserID,
					Date:                    time.Date(2026, 5, 2, 9, 30, 0, 0, time.UTC),
					Distance:                10.50,
					Duration:                3600,
					Pace:                    5.71,
					TypeActivity:            "trail_running",
					Calories:                ptr(650),
					AvgHR:                   ptr(155),
					MaxHR:                   ptr(178),
					AvgCadence:              ptr(168),
					MaxCadence:              ptr(182),
					ElevationGain:           ptr(120.5),
					ElevationLoss:           ptr(115.0),
					VO2MaxEstimate:          ptr(48.5),
					AerobicTrainingEffect:   ptr(3.8),
					AnaerobicTrainingEffect: ptr(1.2),
					PrimaryTrainingFocus:    "Threshold",
					TrainingLoad:            ptr(145.0),
					TrainingStressScore:     ptr(85.0),
					IntensityFactor:         ptr(0.88),
					RecoveryTime:            ptr(24),
					RPE:                     ptr(7),
					AvgStress:               ptr(42),
					SdrrHrv:                 ptr(55),
					RmssdHrv:                ptr(48),
					TimeInHrZone:            []int{120, 480, 1800, 900, 300},
					Notes:                   "Great mountain run",
					Shoes:                   "Hoka Speedgoat",
					Efficiency:              ptr(1.42),
					IsAnomalous:             true,
					AnomalyReason:           ptr("Cadence spike sensor glitch"),
				}
			},
			wantErr: false,
			checkResponse: func(t *testing.T, w *model.Workouts, err error) {
				require.NoError(t, err)
				assert.Greater(t, w.ID, int64(0))

				saved, getErr := workoutRepo.GetByID(ctx, w.ID, defaultUserID)
				require.NoError(t, getErr)
				assert.Equal(t, w.ID, saved.ID)
				assert.Equal(t, "trail_running", saved.TypeActivity)
				assert.Equal(t, 650, *saved.Calories)
				assert.Equal(t, 155, *saved.AvgHR)
				assert.Equal(t, 178, *saved.MaxHR)
				assert.Equal(t, 168, *saved.AvgCadence)
				assert.Equal(t, 182, *saved.MaxCadence)
				assert.InDelta(t, 120.5, *saved.ElevationGain, 0.001)
				assert.InDelta(t, 115.0, *saved.ElevationLoss, 0.001)
				assert.InDelta(t, 48.5, *saved.VO2MaxEstimate, 0.001)
				assert.InDelta(t, 3.8, *saved.AerobicTrainingEffect, 0.001)
				assert.InDelta(t, 1.2, *saved.AnaerobicTrainingEffect, 0.001)
				assert.Equal(t, "Threshold", saved.PrimaryTrainingFocus)
				assert.InDelta(t, 145.0, *saved.TrainingLoad, 0.001)
				assert.InDelta(t, 85.0, *saved.TrainingStressScore, 0.001)
				assert.InDelta(t, 0.88, *saved.IntensityFactor, 0.001)
				assert.Equal(t, 24, *saved.RecoveryTime)
				assert.Equal(t, 7, *saved.RPE)
				assert.Equal(t, 42, *saved.AvgStress)
				assert.Equal(t, 55, *saved.SdrrHrv)
				assert.Equal(t, 48, *saved.RmssdHrv)
				assert.Equal(t, []int{120, 480, 1800, 900, 300}, saved.TimeInHrZone)
				assert.Equal(t, "Great mountain run", saved.Notes)
				assert.Equal(t, "Hoka Speedgoat", saved.Shoes)
				assert.InDelta(t, 1.42, *saved.Efficiency, 0.001)
				assert.True(t, saved.IsAnomalous)
				require.NotNil(t, saved.AnomalyReason)
				assert.Equal(t, "Cadence spike sensor glitch", *saved.AnomalyReason)
			},
		},
		{
			name: "failure on non-existent user id (foreign key constraint violation)",
			workout: func() *model.Workouts {
				return &model.Workouts{
					UserID:       999999, // User does not exist
					Date:         time.Now(),
					Distance:     5.0,
					Duration:     1500,
					Pace:         5.0,
					TypeActivity: "running",
				}
			},
			wantErr: true,
			checkResponse: func(t *testing.T, w *model.Workouts, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.workout()
			err := workoutRepo.Create(ctx, w)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			tt.checkResponse(t, w, err)
		})
	}
}

// -----------------------------------------------------------------------------
// 2. GetByID Tests
// -----------------------------------------------------------------------------

func TestWorkoutRepository_GetByID(t *testing.T) {
	ctx, user1ID := setupTest(t)
	user2ID, err := testDB.CreateUser(ctx, "User Two", "user2@example.com")
	require.NoError(t, err)

	w1 := &model.Workouts{
		UserID:       user1ID,
		Date:         time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
		Distance:     8.0,
		Duration:     2400,
		Pace:         5.0,
		TypeActivity: "running",
		Notes:        "User 1 Workout",
	}
	require.NoError(t, workoutRepo.Create(ctx, w1))

	tests := []struct {
		name          string
		workoutID     int64
		userID        int64
		wantErr       bool
		expectedErr   error
		checkResponse func(t *testing.T, w *model.Workouts, err error)
	}{
		{
			name:        "successfully get existing workout",
			workoutID:   w1.ID,
			userID:      user1ID,
			wantErr:     false,
			expectedErr: nil,
			checkResponse: func(t *testing.T, w *model.Workouts, err error) {
				require.NoError(t, err)
				require.NotNil(t, w)
				assert.Equal(t, w1.ID, w.ID)
				assert.Equal(t, user1ID, w.UserID)
				assert.InDelta(t, 8.0, w.Distance, 0.001)
				assert.Equal(t, "User 1 Workout", w.Notes)
			},
		},
		{
			name:        "not found for non-existent workout ID",
			workoutID:   888888,
			userID:      user1ID,
			wantErr:     true,
			expectedErr: apperrors.ErrWorkoutNotFound,
			checkResponse: func(t *testing.T, w *model.Workouts, err error) {
				assert.Nil(t, w)
				assert.True(t, errors.Is(err, apperrors.ErrWorkoutNotFound))
			},
		},
		{
			name:        "IDOR protection: not found when workout belongs to another user",
			workoutID:   w1.ID,
			userID:      user2ID, // Wrong user
			wantErr:     true,
			expectedErr: apperrors.ErrWorkoutNotFound,
			checkResponse: func(t *testing.T, w *model.Workouts, err error) {
				assert.Nil(t, w)
				assert.True(t, errors.Is(err, apperrors.ErrWorkoutNotFound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := workoutRepo.GetByID(ctx, tt.workoutID, tt.userID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
				}
			} else {
				require.NoError(t, err)
			}
			tt.checkResponse(t, w, err)
		})
	}
}

// -----------------------------------------------------------------------------
// 3. Update Tests
// -----------------------------------------------------------------------------

func TestWorkoutRepository_Update(t *testing.T) {
	ctx, user1ID := setupTest(t)
	user2ID, err := testDB.CreateUser(ctx, "User Two", "user2_update@example.com")
	require.NoError(t, err)

	w := &model.Workouts{
		UserID:       user1ID,
		Date:         time.Date(2026, 5, 1, 7, 0, 0, 0, time.UTC),
		Distance:     5.0,
		Duration:     1500,
		Pace:         5.0,
		TypeActivity: "running",
		Calories:     ptr(300),
		Notes:        "Initial Note",
		Shoes:        "Nike",
	}
	require.NoError(t, workoutRepo.Create(ctx, w))

	tests := []struct {
		name          string
		modifyWorkout func() *model.Workouts
		wantErr       bool
		expectedErr   error
		checkResponse func(t *testing.T, err error)
	}{
		{
			name: "successfully update workout metrics, notes, shoes, and hr zones",
			modifyWorkout: func() *model.Workouts {
				updated := *w
				updated.Distance = 6.20
				updated.Duration = 1860
				updated.Pace = 5.0
				updated.Calories = ptr(380)
				updated.AvgHR = ptr(162)
				updated.Notes = "Updated Note - Felt energetic"
				updated.Shoes = "Adidas Boston"
				updated.TimeInHrZone = []int{60, 200, 900, 600, 100}
				updated.RPE = ptr(8)
				return &updated
			},
			wantErr: false,
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
				saved, getErr := workoutRepo.GetByID(ctx, w.ID, user1ID)
				require.NoError(t, getErr)
				assert.InDelta(t, 6.20, saved.Distance, 0.001)
				assert.Equal(t, 1860, saved.Duration)
				assert.Equal(t, 380, *saved.Calories)
				assert.Equal(t, 162, *saved.AvgHR)
				assert.Equal(t, "Updated Note - Felt energetic", saved.Notes)
				assert.Equal(t, "Adidas Boston", saved.Shoes)
				assert.Equal(t, []int{60, 200, 900, 600, 100}, saved.TimeInHrZone)
				assert.Equal(t, 8, *saved.RPE)
			},
		},
		{
			name: "not found when updating non-existent workout ID",
			modifyWorkout: func() *model.Workouts {
				return &model.Workouts{
					ID:           999999,
					UserID:       user1ID,
					Distance:     10.0,
					Duration:     3000,
					Pace:         5.0,
					TypeActivity: "running",
				}
			},
			wantErr:     true,
			expectedErr: apperrors.ErrWorkoutNotFound,
			checkResponse: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, apperrors.ErrWorkoutNotFound))
			},
		},
		{
			name: "IDOR protection: not found when trying to update workout belonging to another user",
			modifyWorkout: func() *model.Workouts {
				wrongUserWorkout := *w
				wrongUserWorkout.UserID = user2ID // Attempt to update user1's workout as user2
				wrongUserWorkout.Notes = "Malicious update attempt"
				return &wrongUserWorkout
			},
			wantErr:     true,
			expectedErr: apperrors.ErrWorkoutNotFound,
			checkResponse: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, apperrors.ErrWorkoutNotFound))
				// Verify original workout remains unchanged
				saved, getErr := workoutRepo.GetByID(ctx, w.ID, user1ID)
				require.NoError(t, getErr)
				assert.NotEqual(t, "Malicious update attempt", saved.Notes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := tt.modifyWorkout()
			err := workoutRepo.Update(ctx, item)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
				}
			} else {
				require.NoError(t, err)
			}
			tt.checkResponse(t, err)
		})
	}
}

// -----------------------------------------------------------------------------
// 4. Delete Tests
// -----------------------------------------------------------------------------

func TestWorkoutRepository_Delete(t *testing.T) {
	ctx, user1ID := setupTest(t)
	user2ID, err := testDB.CreateUser(ctx, "User Two", "user2_delete@example.com")
	require.NoError(t, err)

	w := &model.Workouts{
		UserID:       user1ID,
		Date:         time.Now(),
		Distance:     5.0,
		Duration:     1500,
		Pace:         5.0,
		TypeActivity: "running",
	}
	require.NoError(t, workoutRepo.Create(ctx, w))

	tests := []struct {
		name          string
		workoutID     int64
		userID        int64
		wantErr       bool
		expectedErr   error
		checkResponse func(t *testing.T, err error)
	}{
		{
			name:        "IDOR protection: cannot delete workout belonging to another user",
			workoutID:   w.ID,
			userID:      user2ID, // Wrong user
			wantErr:     true,
			expectedErr: apperrors.ErrWorkoutNotFound,
			checkResponse: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, apperrors.ErrWorkoutNotFound))
				// Verify workout still exists
				saved, getErr := workoutRepo.GetByID(ctx, w.ID, user1ID)
				require.NoError(t, getErr)
				assert.NotNil(t, saved)
			},
		},
		{
			name:        "not found when deleting non-existent workout",
			workoutID:   999999,
			userID:      user1ID,
			wantErr:     true,
			expectedErr: apperrors.ErrWorkoutNotFound,
			checkResponse: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, apperrors.ErrWorkoutNotFound))
			},
		},
		{
			name:        "successfully delete workout",
			workoutID:   w.ID,
			userID:      user1ID,
			wantErr:     false,
			expectedErr: nil,
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
				// Verify workout is gone
				_, getErr := workoutRepo.GetByID(ctx, w.ID, user1ID)
				assert.True(t, errors.Is(getErr, apperrors.ErrWorkoutNotFound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := workoutRepo.DeleteWorkout(ctx, tt.workoutID, tt.userID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedErr != nil {
					assert.True(t, errors.Is(err, tt.expectedErr))
				}
			} else {
				require.NoError(t, err)
			}
			tt.checkResponse(t, err)
		})
	}
}

// -----------------------------------------------------------------------------
// 5. GetAllByUserID with Dynamic Filters and Pagination
// -----------------------------------------------------------------------------

func TestWorkoutRepository_GetAllByUserID_FiltersAndPagination(t *testing.T) {
	ctx, user1ID := setupTest(t)
	user2ID, err := testDB.CreateUser(ctx, "User Two", "user2_filter@example.com")
	require.NoError(t, err)

	seedDate := func(day int) time.Time {
		return time.Date(2026, 5, day, 10, 0, 0, 0, time.UTC)
	}

	workouts := []*model.Workouts{
		// W0: Short easy run
		{
			UserID:       user1ID,
			Date:         seedDate(1),
			Distance:     5.0,
			Duration:     1500,
			Pace:         5.0,
			TypeActivity: "running",
			AvgHR:        ptr(140),
			Calories:     ptr(300),
			TrainingLoad: ptr(50.0),
			RPE:          ptr(4),
			Notes:        "easy recovery morning run",
			Shoes:        "Pegasus",
			TimeInHrZone: []int{200, 800, 500, 0, 0},
		},
		// W1: Medium tempo run
		{
			UserID:       user1ID,
			Date:         seedDate(5),
			Distance:     10.0,
			Duration:     2700,
			Pace:         4.5,
			TypeActivity: "running",
			AvgHR:        ptr(165),
			Calories:     ptr(650),
			TrainingLoad: ptr(110.0),
			RPE:          ptr(8),
			Notes:        "tempo interval session",
			Shoes:        "Vaporfly",
			TimeInHrZone: []int{100, 400, 1200, 800, 200},
		},
		// W2: Cycling ride
		{
			UserID:       user1ID,
			Date:         seedDate(10),
			Distance:     25.0,
			Duration:     3600,
			Pace:         2.4,
			TypeActivity: "cycling",
			AvgHR:        ptr(130),
			Calories:     ptr(500),
			TrainingLoad: ptr(80.0),
			RPE:          ptr(5),
			Notes:        "", // empty notes
			Shoes:        "Shimano",
			TimeInHrZone: []int{}, // empty hr zones
		},
		// W3: Long trail run
		{
			UserID:       user1ID,
			Date:         seedDate(15),
			Distance:     18.0,
			Duration:     6480,
			Pace:         6.0,
			TypeActivity: "trail_running",
			AvgHR:        ptr(155),
			Calories:     ptr(1200),
			TrainingLoad: ptr(180.0),
			RPE:          ptr(7),
			Notes:        "hilly mountain adventure",
			Shoes:        "Speedgoat",
			TimeInHrZone: []int{300, 1000, 3000, 1800, 380},
		},
		// W_User2: Workout belonging to user2 (must never be returned in user1 queries)
		{
			UserID:       user2ID,
			Date:         seedDate(5),
			Distance:     10.0,
			Duration:     2700,
			Pace:         4.5,
			TypeActivity: "running",
			Notes:        "User 2 private workout",
			Shoes:        "Vaporfly",
		},
	}

	for _, w := range workouts {
		require.NoError(t, workoutRepo.Create(ctx, w))
	}

	tests := []struct {
		name          string
		filter        dto.WorkoutFilter
		expectedCount int
		checkResponse func(t *testing.T, results []model.Workouts)
	}{
		{
			name: "filter by type_activity = running",
			filter: dto.WorkoutFilter{
				UserID: user1ID,
				Type:   "running",
			},
			expectedCount: 2,
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.Equal(t, "running", r.TypeActivity)
					assert.Equal(t, user1ID, r.UserID)
				}
			},
		},
		{
			name: "filter by date range (from 2026-05-04 to 2026-05-12)",
			filter: dto.WorkoutFilter{
				UserID: user1ID,
				From:   ptr(seedDate(4)),
				To:     ptr(seedDate(12)),
			},
			expectedCount: 2, // W1 (May 5) and W2 (May 10)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.True(t, r.Date.After(seedDate(3)) && r.Date.Before(seedDate(13)))
				}
			},
		},
		{
			name: "filter by distance range (10km to 20km)",
			filter: dto.WorkoutFilter{
				UserID:      user1ID,
				MinDistance: ptr(10.0),
				MaxDistance: ptr(20.0),
			},
			expectedCount: 2, // W1 (10km) and W3 (18km)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.GreaterOrEqual(t, r.Distance, 10.0)
					assert.LessOrEqual(t, r.Distance, 20.0)
				}
			},
		},
		{
			name: "filter by duration range (min 3000 sec)",
			filter: dto.WorkoutFilter{
				UserID:      user1ID,
				MinDuration: ptr(3000),
			},
			expectedCount: 2, // W2 (3600) and W3 (6480)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.GreaterOrEqual(t, r.Duration, 3000)
				}
			},
		},
		{
			name: "filter by avg_hr (min 150)",
			filter: dto.WorkoutFilter{
				UserID:   user1ID,
				MinAvgHR: ptr(150),
			},
			expectedCount: 2, // W1 (165) and W3 (155)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					require.NotNil(t, r.AvgHR)
					assert.GreaterOrEqual(t, *r.AvgHR, 150)
				}
			},
		},
		{
			name: "filter by pace range (4.0 to 5.0)",
			filter: dto.WorkoutFilter{
				UserID:  user1ID,
				MinPace: ptr(4.0),
				MaxPace: ptr(5.0),
			},
			expectedCount: 2, // W0 (5.0) and W1 (4.5)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.GreaterOrEqual(t, r.Pace, 4.0)
					assert.LessOrEqual(t, r.Pace, 5.0)
				}
			},
		},
		{
			name: "filter by min RPE (min 7)",
			filter: dto.WorkoutFilter{
				UserID: user1ID,
				MinRPE: ptr(7),
			},
			expectedCount: 2, // W1 (8) and W3 (7)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					require.NotNil(t, r.RPE)
					assert.GreaterOrEqual(t, *r.RPE, 7)
				}
			},
		},
		{
			name: "filter by HasNotes = true",
			filter: dto.WorkoutFilter{
				UserID:   user1ID,
				HasNotes: ptr(true),
			},
			expectedCount: 3, // W0, W1, W3 (W2 has empty notes)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.NotEmpty(t, r.Notes)
				}
			},
		},
		{
			name: "filter by HasNotes = false",
			filter: dto.WorkoutFilter{
				UserID:   user1ID,
				HasNotes: ptr(false),
			},
			expectedCount: 1, // W2
			checkResponse: func(t *testing.T, results []model.Workouts) {
				require.Len(t, results, 1)
				assert.Empty(t, results[0].Notes)
				assert.Equal(t, "cycling", results[0].TypeActivity)
			},
		},
		{
			name: "filter by HasHRZones = true",
			filter: dto.WorkoutFilter{
				UserID:     user1ID,
				HasHRZones: ptr(true),
			},
			expectedCount: 3, // W0, W1, W3
			checkResponse: func(t *testing.T, results []model.Workouts) {
				for _, r := range results {
					assert.NotEmpty(t, r.TimeInHrZone)
				}
			},
		},
		{
			name: "filter by HasHRZones = false",
			filter: dto.WorkoutFilter{
				UserID:     user1ID,
				HasHRZones: ptr(false),
			},
			expectedCount: 1, // W2
			checkResponse: func(t *testing.T, results []model.Workouts) {
				require.Len(t, results, 1)
				assert.Empty(t, results[0].TimeInHrZone)
			},
		},
		{
			name: "filter by Shoes",
			filter: dto.WorkoutFilter{
				UserID: user1ID,
				Shoes:  "Vaporfly",
			},
			expectedCount: 1, // W1
			checkResponse: func(t *testing.T, results []model.Workouts) {
				require.Len(t, results, 1)
				assert.Equal(t, "Vaporfly", results[0].Shoes)
				assert.Equal(t, user1ID, results[0].UserID)
			},
		},
		{
			name: "sorting by distance ASC",
			filter: dto.WorkoutFilter{
				UserID:    user1ID,
				SortBy:    "distance",
				SortOrder: "asc",
			},
			expectedCount: 4,
			checkResponse: func(t *testing.T, results []model.Workouts) {
				require.Len(t, results, 4)
				assert.InDelta(t, 5.0, results[0].Distance, 0.001)
				assert.InDelta(t, 10.0, results[1].Distance, 0.001)
				assert.InDelta(t, 18.0, results[2].Distance, 0.001)
				assert.InDelta(t, 25.0, results[3].Distance, 0.001)
			},
		},
		{
			name: "pagination: Limit 2, Offset 1 with default date DESC sort",
			filter: dto.WorkoutFilter{
				UserID: user1ID,
				Limit:  2,
				Offset: 1,
			},
			expectedCount: 2,
			// All sorted by date DESC: W3 (May 15), W2 (May 10), W1 (May 5), W0 (May 1)
			// Offset 1, Limit 2 => W2 (May 10), W1 (May 5)
			checkResponse: func(t *testing.T, results []model.Workouts) {
				require.Len(t, results, 2)
				assert.Equal(t, "cycling", results[0].TypeActivity)
				assert.Equal(t, "running", results[1].TypeActivity)
				assert.InDelta(t, 10.0, results[1].Distance, 0.001)
			},
		},
		{
			name: "user isolation: user2 only sees their single workout",
			filter: dto.WorkoutFilter{
				UserID: user2ID,
			},
			expectedCount: 1,
			checkResponse: func(t *testing.T, results []model.Workouts) {
				require.Len(t, results, 1)
				assert.Equal(t, user2ID, results[0].UserID)
				assert.Equal(t, "User 2 private workout", results[0].Notes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := workoutRepo.GetAllByUserID(ctx, tt.filter)
			require.NoError(t, err)
			assert.Len(t, results, tt.expectedCount)
			tt.checkResponse(t, results)
		})
	}
}

// -----------------------------------------------------------------------------
// 6. GetMonthlyHistory Tests
// -----------------------------------------------------------------------------

func TestWorkoutRepository_GetMonthlyHistory(t *testing.T) {
	ctx, userID := setupTest(t)

	// Workouts in April 2026
	require.NoError(t, workoutRepo.Create(ctx, &model.Workouts{
		UserID:       userID,
		Date:         time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
		Distance:     6.0,
		Duration:     1800,
		Pace:         5.0,
		TypeActivity: "running",
		Notes:        "April 15 run",
	}))
	require.NoError(t, workoutRepo.Create(ctx, &model.Workouts{
		UserID:       userID,
		Date:         time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC),
		Distance:     10.0,
		Duration:     3000,
		Pace:         5.0,
		TypeActivity: "running",
		Notes:        "April 25 run",
	}))

	// Workouts in May 2026
	require.NoError(t, workoutRepo.Create(ctx, &model.Workouts{
		UserID:       userID,
		Date:         time.Date(2026, 5, 2, 7, 30, 0, 0, time.UTC),
		Distance:     12.0,
		Duration:     3600,
		Pace:         5.0,
		TypeActivity: "running",
		Notes:        "May 2 run",
	}))

	tests := []struct {
		name          string
		userID        int64
		limit         int
		offset        int
		expectedLen   int
		checkResponse func(t *testing.T, history []model.WorkoutMonthHistory)
	}{
		{
			name:        "monthly history aggregates workouts for active user",
			userID:      userID,
			limit:       3,
			offset:      0,
			expectedLen: 2, // May 2026 and April 2026
			checkResponse: func(t *testing.T, history []model.WorkoutMonthHistory) {
				require.Len(t, history, 2)

				// Most recent month first: May 2026
				may := history[0]
				assert.Equal(t, 2026, may.Month.Year())
				assert.Equal(t, time.May, may.Month.Month())
				assert.Equal(t, 1, may.WorkoutsCount)
				assert.InDelta(t, 12.0, may.TotalDistance, 0.001)
				assert.Equal(t, 3600, may.TotalDuration)
				require.Len(t, may.Workouts, 1)
				assert.Equal(t, "May 2 run", may.Workouts[0].Place)

				// Next month: April 2026
				april := history[1]
				assert.Equal(t, 2026, april.Month.Year())
				assert.Equal(t, time.April, april.Month.Month())
				assert.Equal(t, 2, april.WorkoutsCount)
				assert.InDelta(t, 16.0, april.TotalDistance, 0.001)
				assert.Equal(t, 4800, april.TotalDuration)
				require.Len(t, april.Workouts, 2)
			},
		},
		{
			name:        "empty history for user with zero workouts",
			userID:      888888,
			limit:       3,
			offset:      0,
			expectedLen: 0,
			checkResponse: func(t *testing.T, history []model.WorkoutMonthHistory) {
				assert.Empty(t, history)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := workoutRepo.GetMonthlyHistory(ctx, tt.userID, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, res, tt.expectedLen)
			tt.checkResponse(t, res)
		})
	}
}

// -----------------------------------------------------------------------------
// 7. Cascade Deletion Scenarios
// -----------------------------------------------------------------------------

func TestWorkoutRepository_CascadeDelete(t *testing.T) {
	ctx, userID := setupTest(t)

	w1 := &model.Workouts{
		UserID:       userID,
		Date:         time.Now(),
		Distance:     5.0,
		Duration:     1500,
		Pace:         5.0,
		TypeActivity: "running",
	}
	w2 := &model.Workouts{
		UserID:       userID,
		Date:         time.Now(),
		Distance:     10.0,
		Duration:     3000,
		Pace:         5.0,
		TypeActivity: "running",
	}
	require.NoError(t, workoutRepo.Create(ctx, w1))
	require.NoError(t, workoutRepo.Create(ctx, w2))

	// Verify workouts exist
	list, err := workoutRepo.GetAllByUserID(ctx, dto.WorkoutFilter{UserID: userID})
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Delete user directly from users table
	tag, err := testDB.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1;", userID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), tag.RowsAffected())

	// Verify PostgreSQL cascade deleted all associated workouts
	listAfter, err := workoutRepo.GetAllByUserID(ctx, dto.WorkoutFilter{UserID: userID})
	require.NoError(t, err)
	assert.Empty(t, listAfter)

	// Verify GetByID returns ErrWorkoutNotFound for both deleted workouts
	_, err1 := workoutRepo.GetByID(ctx, w1.ID, userID)
	assert.True(t, errors.Is(err1, apperrors.ErrWorkoutNotFound))

	_, err2 := workoutRepo.GetByID(ctx, w2.ID, userID)
	assert.True(t, errors.Is(err2, apperrors.ErrWorkoutNotFound))
}

// -----------------------------------------------------------------------------
// 8. Transactional Scenarios (Commit & Rollback)
// -----------------------------------------------------------------------------

func TestWorkoutRepository_Transactions(t *testing.T) {
	ctx, userID := setupTest(t)

	t.Run("transaction commit persists changes", func(t *testing.T) {
		var workoutID int64

		err := txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			w := &model.Workouts{
				UserID:       userID,
				Date:         time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
				Distance:     7.5,
				Duration:     2250,
				Pace:         5.0,
				TypeActivity: "running",
				Notes:        "Transaction Commit Run",
			}
			if err := workoutRepo.Create(txCtx, w); err != nil {
				return err
			}
			workoutID = w.ID
			return nil
		})

		require.NoError(t, err)
		assert.Greater(t, workoutID, int64(0))

		// Workout must exist in DB outside transaction
		saved, getErr := workoutRepo.GetByID(ctx, workoutID, userID)
		require.NoError(t, getErr)
		require.NotNil(t, saved)
		assert.Equal(t, "Transaction Commit Run", saved.Notes)
	})

	t.Run("transaction rollback discards changes on error", func(t *testing.T) {
		var workoutID int64
		simulatedErr := errors.New("simulated business logic failure")

		err := txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			w := &model.Workouts{
				UserID:       userID,
				Date:         time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
				Distance:     15.0,
				Duration:     4500,
				Pace:         5.0,
				TypeActivity: "running",
				Notes:        "Rollback Run",
			}
			if err := workoutRepo.Create(txCtx, w); err != nil {
				return err
			}
			workoutID = w.ID
			assert.Greater(t, workoutID, int64(0))

			// Return error to trigger rollback
			return simulatedErr
		})

		require.Error(t, err)
		assert.True(t, errors.Is(err, simulatedErr))

		// Workout must NOT exist in DB because transaction was rolled back
		saved, getErr := workoutRepo.GetByID(ctx, workoutID, userID)
		assert.Nil(t, saved)
		assert.True(t, errors.Is(getErr, apperrors.ErrWorkoutNotFound))
	})
}
