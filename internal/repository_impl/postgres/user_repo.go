package postgres

import (
"SmartRun/internal/cache"
"SmartRun/internal/model"
"SmartRun/internal/repository"
"SmartRun/pkg/my_errors"
"context"
"encoding/json"
"errors"
"fmt"
"time"

"github.com/jackc/pgx/v5"
)

type userRepository struct {
db    repository.DB
cache cache.Cache
}

func NewUserRepository(db repository.DB, cache cache.Cache) repository.UserRepository {
return &userRepository{db: db, cache: cache}
}

func (r *userRepository) getDB(ctx context.Context) repository.DB {
if tx, ok := GetTx(ctx); ok {
return tx
}
return r.db
}

func (r *userRepository) cacheGet(ctx context.Context, key string) (string, bool) {
if r.cache == nil {
return "", false
}
cached, err := r.cache.Get(ctx, key)
if err != nil {
return "", false
}
return cached, true
}

func (r *userRepository) cacheSet(ctx context.Context, key string, value string, ttl time.Duration) {
if r.cache == nil {
return
}
_ = r.cache.Set(ctx, key, value, ttl)
}

func (r *userRepository) InvalidateUserCache(ctx context.Context, userID int64, email string) {
if r.cache == nil {
return
}
_ = r.cache.Del(ctx, fmt.Sprintf("user:%d", userID))
_ = r.cache.Del(ctx, fmt.Sprintf("user:email:%d", userID))
if email != "" {
_ = r.cache.Del(ctx, fmt.Sprintf("user:email:%s", email))
}
}

func (r *userRepository) CreateUser(ctx context.Context, user *model.User) error {
db := r.getDB(ctx)

query := `
        INSERT INTO users (
name, email, password,
gender, age, weight_kg, height_cm,
resting_hr, max_hr, weekly_runs, threshold_pace_min_km,
created_at
)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
        RETURNING id, created_at
    `
return db.QueryRow(ctx, query,
user.Name,
user.Email,
user.Password,
user.Gender,
user.Age,
user.WeightKg,
user.HeightCm,
user.RestingHR,
user.MaxHR,
user.WeeklyRuns,
user.ThresholdPace,
).Scan(&user.ID, &user.CreatedAt)
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
key := fmt.Sprintf("user:email:%s", email)
if cached, ok := r.cacheGet(ctx, key); ok {
var user model.User
if err := json.Unmarshal([]byte(cached), &user); err == nil {
return &user, nil
}
}

db := r.getDB(ctx)

query := `
SELECT id, name, email, password,
       gender, age, weight_kg, height_cm,
       resting_hr, max_hr, weekly_runs, threshold_pace_min_km,
       created_at
FROM users
WHERE email = $1;
`

var user model.User
err := db.QueryRow(ctx, query, email).Scan(
&user.ID,
&user.Name,
&user.Email,
&user.Password,
&user.Gender,
&user.Age,
&user.WeightKg,
&user.HeightCm,
&user.RestingHR,
&user.MaxHR,
&user.WeeklyRuns,
&user.ThresholdPace,
&user.CreatedAt,
)

if errors.Is(err, pgx.ErrNoRows) {
return nil, my_errors.ErrUserNotFound
}
if err != nil {
return nil, err
}

if data, marshalErr := json.Marshal(&user); marshalErr == nil {
r.cacheSet(ctx, key, string(data), 30*time.Minute)
}

return &user, nil
}

func (r *userRepository) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
key := fmt.Sprintf("user:%d", id)
if cached, ok := r.cacheGet(ctx, key); ok {
var user model.User
if err := json.Unmarshal([]byte(cached), &user); err == nil {
return &user, nil
}
}

db := r.getDB(ctx)

query := `
SELECT id, name, email, password,
       gender, age, weight_kg, height_cm,
       resting_hr, max_hr, weekly_runs, threshold_pace_min_km,
       created_at
FROM users
WHERE id = $1;
`

var user model.User
err := db.QueryRow(ctx, query, id).Scan(
&user.ID,
&user.Name,
&user.Email,
&user.Password,
&user.Gender,
&user.Age,
&user.WeightKg,
&user.HeightCm,
&user.RestingHR,
&user.MaxHR,
&user.WeeklyRuns,
&user.ThresholdPace,
&user.CreatedAt,
)

if errors.Is(err, pgx.ErrNoRows) {
return nil, my_errors.ErrUserNotFound
}
if err != nil {
return nil, err
}

if data, marshalErr := json.Marshal(&user); marshalErr == nil {
r.cacheSet(ctx, key, string(data), 30*time.Minute)
}

return &user, nil
}

func (r *userRepository) GetEmailByID(ctx context.Context, id int64) (string, error) {
key := fmt.Sprintf("user:email:%d", id)
if cached, ok := r.cacheGet(ctx, key); ok {
return cached, nil
}

db := r.getDB(ctx)

query := `
SELECT email FROM users WHERE id = $1;
`

var email string
err := db.QueryRow(ctx, query, id).Scan(&email)
if errors.Is(err, pgx.ErrNoRows) {
return "", my_errors.ErrUserNotFound
}
if err != nil {
return "", err
}

r.cacheSet(ctx, key, email, 30*time.Minute)

return email, nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, userID int64, newHash string) error {
db := r.getDB(ctx)

query := `
UPDATE users
SET password = $1
WHERE id = $2
`

tag, err := db.Exec(ctx, query, newHash, userID)
if err != nil {
return err
}
if tag.RowsAffected() == 0 {
return my_errors.ErrUserNotFound
}

return nil
}

func (r *userRepository) UpdateUser(ctx context.Context, user *model.User) error {
db := r.getDB(ctx)
query := `
UPDATE users
SET name = $1, email = $2, password = $3, gender = $4, age = $5, weight_kg = $6, height_cm = $7, resting_hr = $8, max_hr = $9, weekly_runs = $10, threshold_pace_min_km = $11
WHERE id = $12
`
tag, err := db.Exec(ctx, query,
user.Name,
user.Email,
user.Password,
user.Gender,
user.Age,
user.WeightKg,
user.HeightCm,
user.RestingHR,
user.MaxHR,
user.WeeklyRuns,
user.ThresholdPace,
user.ID,
)
if err != nil {
return err
}
if tag.RowsAffected() == 0 {
return my_errors.ErrUserNotFound
}
return nil
}
