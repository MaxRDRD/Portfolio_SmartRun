package service_test

import (
	"SmartRun/internal/broker"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/ports/outgoing/importer"
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
)

// --- MockWorkoutRepo ---
type MockWorkoutRepo struct {
	mock.Mock
}

func (m *MockWorkoutRepo) Create(ctx context.Context, workout *model.Workouts) error {
	return m.Called(ctx, workout).Error(0)
}

func (m *MockWorkoutRepo) GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error) {
	args := m.Called(ctx, id, userID)
	var w *model.Workouts
	if res := args.Get(0); res != nil {
		w = res.(*model.Workouts)
	}
	return w, args.Error(1)
}

func (m *MockWorkoutRepo) GetAllByUserID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	args := m.Called(ctx, filter)
	var list []model.Workouts
	if res := args.Get(0); res != nil {
		list = res.([]model.Workouts)
	}
	return list, args.Error(1)
}

func (m *MockWorkoutRepo) DeleteWorkout(ctx context.Context, id int64, userID int64) error {
	return m.Called(ctx, id, userID).Error(0)
}

func (m *MockWorkoutRepo) Update(ctx context.Context, workout *model.Workouts) error {
	return m.Called(ctx, workout).Error(0)
}

func (m *MockWorkoutRepo) GetMonthlyHistory(ctx context.Context, userID int64, limit, offset int) ([]model.WorkoutMonthHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	var list []model.WorkoutMonthHistory
	if res := args.Get(0); res != nil {
		list = res.([]model.WorkoutMonthHistory)
	}
	return list, args.Error(1)
}

// --- MockUserRepo ---
type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) CreateUser(ctx context.Context, user *model.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *MockUserRepo) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	args := m.Called(ctx, userID)
	var u *model.User
	if res := args.Get(0); res != nil {
		u = res.(*model.User)
	}
	return u, args.Error(1)
}

func (m *MockUserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	var u *model.User
	if res := args.Get(0); res != nil {
		u = res.(*model.User)
	}
	return u, args.Error(1)
}

func (m *MockUserRepo) InvalidateUserCache(ctx context.Context, userID int64, email string) {
	m.Called(ctx, userID, email)
}

func (m *MockUserRepo) UpdatePassword(ctx context.Context, userID int64, newHash string) error {
	args := m.Called(ctx, userID, newHash)
	return args.Error(0)
}

func (m *MockUserRepo) GetEmailByID(ctx context.Context, id int64) (string, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockUserRepo) UpdateUser(ctx context.Context, user *model.User) error {
	return m.Called(ctx, user).Error(0)
}

// --- MockTxManager (с выполнением замыкания!) ---
type MockTxManager struct {
	mock.Mock
}

func (m *MockTxManager) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, mock.Anything)
	if err := args.Error(0); err != nil {
		return err
	}
	return fn(ctx) // Выполняем логику внутри транзакции
}

// --- MockPublisher ---
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) PublishMetricsRecalculation(ctx context.Context, event broker.MetricsRecalculationEvent) error {
	return m.Called(ctx, event).Error(0)
}

func (m *MockPublisher) PublishPasswordReset(ctx context.Context, event broker.PasswordResetEvent) error {
	return m.Called(ctx, event).Error(0)
}

func (m *MockPublisher) PublishAICoachEvent(ctx context.Context, event broker.MetricsRecalculationEvent) error {
	return m.Called(ctx, event).Error(0)
}

// --- MockDailyMetricsRepo ---
type MockDailyMetricsRepo struct {
	mock.Mock
}

func (m *MockDailyMetricsRepo) Create(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	args := m.Called(ctx, dailyMetric)
	var res *model.DailyMetric
	if args.Get(0) != nil {
		res = args.Get(0).(*model.DailyMetric)
	}
	return res, args.Error(1)
}

func (m *MockDailyMetricsRepo) Update(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	args := m.Called(ctx, dailyMetric)
	var res *model.DailyMetric
	if args.Get(0) != nil {
		res = args.Get(0).(*model.DailyMetric)
	}
	return res, args.Error(1)
}

func (m *MockDailyMetricsRepo) Delete(ctx context.Context, id, userID int64) error {
	return m.Called(ctx, id, userID).Error(0)
}

func (m *MockDailyMetricsRepo) GetByID(ctx context.Context, id, userID int64) (*model.DailyMetric, error) {
	args := m.Called(ctx, id, userID)
	var res *model.DailyMetric
	if args.Get(0) != nil {
		res = args.Get(0).(*model.DailyMetric)
	}
	return res, args.Error(1)
}

func (m *MockDailyMetricsRepo) GetAllByUserID(ctx context.Context, userId int64) ([]model.DailyMetric, error) {
	args := m.Called(ctx, userId)
	var res []model.DailyMetric
	if args.Get(0) != nil {
		res = args.Get(0).([]model.DailyMetric)
	}
	return res, args.Error(1)
}

func (m *MockDailyMetricsRepo) GetByUserIDAndDate(ctx context.Context, userID int64, date time.Time) (*model.DailyMetric, error) {
	args := m.Called(ctx, userID, date)
	var res *model.DailyMetric
	if args.Get(0) != nil {
		res = args.Get(0).(*model.DailyMetric)
	}
	return res, args.Error(1)
}

func (m *MockDailyMetricsRepo) UpdateOrCreate(ctx context.Context, dailyMetric *model.DailyMetric) error {
	return m.Called(ctx, dailyMetric).Error(0)
}

// --- MockFitParser ---
type MockFitParser struct {
	mock.Mock
}

func (m *MockFitParser) Parse(ctx context.Context, data []byte) (*importer.ActivityData, error) {
	args := m.Called(ctx, data)
	var res *importer.ActivityData
	if args.Get(0) != nil {
		res = args.Get(0).(*importer.ActivityData)
	}
	return res, args.Error(1)
}

// --- MockSessionRepo ---
type MockSessionRepo struct {
	mock.Mock
}

func (m *MockSessionRepo) DeleteSessionByHash(ctx context.Context, refresh_hash string) error {
	return m.Called(ctx, refresh_hash).Error(0)
}

func (m *MockSessionRepo) CreateSession(ctx context.Context, session *model.Session) error {
	return m.Called(ctx, session).Error(0)
}

func (m *MockSessionRepo) FindSessionByHash(ctx context.Context, hash string) (*model.Session, error) {
	args := m.Called(ctx, hash)
	var s *model.Session
	if res := args.Get(0); res != nil {
		s = res.(*model.Session)
	}
	return s, args.Error(1)
}

func (m *MockSessionRepo) ConsumeSessionByHash(ctx context.Context, hash string) (int64, error) {
	args := m.Called(ctx, hash)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepo) CleanupExpiredSessions(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockSessionRepo) DeleteAllSessionsForUser(ctx context.Context, userID int64) error {
	return m.Called(ctx, userID).Error(0)
}

// --- MockTotpRepo ---
type MockTotpRepo struct {
	mock.Mock
}

func (m *MockTotpRepo) GetTOTPSecret(ctx context.Context, userID int64) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *MockTotpRepo) UpdateTOTPSecret(ctx context.Context, userID int64, secret string, enabled bool) error {
	return m.Called(ctx, userID, secret, enabled).Error(0)
}

func (m *MockTotpRepo) IsTOTPEnabled(ctx context.Context, userID int64) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

// --- MockPasswordResetRepo ---
type MockPasswordResetRepo struct {
	mock.Mock
}

func (m *MockPasswordResetRepo) CreateResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	return m.Called(ctx, userID, tokenHash, expiresAt).Error(0)
}

func (m *MockPasswordResetRepo) FindResetByTokenHash(ctx context.Context, tokenHash string) (int64, bool, error) {
	args := m.Called(ctx, tokenHash)
	return args.Get(0).(int64), args.Bool(1), args.Error(2)
}

func (m *MockPasswordResetRepo) MarkAsUsed(ctx context.Context, tokenHash string) error {
	return m.Called(ctx, tokenHash).Error(0)
}

func (m *MockPasswordResetRepo) ConsumeResetToken(ctx context.Context, tokenHash string, userID int64) error {
	return m.Called(ctx, tokenHash, userID).Error(0)
}

func (m *MockPasswordResetRepo) DeleteResetToken(ctx context.Context, tokenHash string) error {
	return m.Called(ctx, tokenHash).Error(0)
}

func (m *MockPasswordResetRepo) CleanupExpiredResetTokens(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// --- MockEmailService ---
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendPasswordResetEmail(ctx context.Context, to string, resetLink string, userName string) error {
	return m.Called(ctx, to, resetLink, userName).Error(0)
}
