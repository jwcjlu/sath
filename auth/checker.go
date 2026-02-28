package auth

import "context"

// Checker 数据源访问权限校验；首版为占位，可扩展为 RBAC。
type Checker interface {
	CanQuery(ctx context.Context, userID, datasourceID string) bool
	CanExecute(ctx context.Context, userID, datasourceID string, dsl string) bool
}

// PermissiveChecker 占位实现：始终允许。
var PermissiveChecker Checker = &permissiveChecker{}

type permissiveChecker struct{}

func (*permissiveChecker) CanQuery(ctx context.Context, userID, datasourceID string) bool {
	return true
}
func (*permissiveChecker) CanExecute(ctx context.Context, userID, datasourceID string, dsl string) bool {
	return true
}
