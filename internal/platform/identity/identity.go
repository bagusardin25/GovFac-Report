// We keep the authenticated user here so features can read it without importing auth.
package identity

import "context"

type User struct {
	ID    int64
	Name  string
	Email string
	Role  string
}

type ctxKey struct{}

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(ctxKey{}).(*User)
	return u
}
