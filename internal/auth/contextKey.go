package auth

import "context"

type contextKey string

const UserIDKey contextKey = "userID"

func SetUserID(ctx context.Context, id int) context.Context {
	return context.WithValue(ctx, UserIDKey, id)
}

func GetUserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(UserIDKey).(int64)
	return id, ok
}
