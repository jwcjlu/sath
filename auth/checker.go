package auth

import "context"

// Checker 定义数据查询/修改前的权限检查接口。
// 未来可由业务方实现细粒度权限策略。
type Checker interface {
	// CanQuery 判断用户是否有在指定数据源上执行只读查询的权限。
	CanQuery(ctx context.Context, userID, datasourceID, dsl string) bool
	// CanExecute 判断用户是否有在指定数据源上执行写/改操作的权限。
	CanExecute(ctx context.Context, userID, datasourceID, dsl string) bool
}

// PermissiveChecker 是一个始终放行的默认实现，适用于本地开发或未接权限系统时的占位。
type PermissiveChecker struct{}

func (PermissiveChecker) CanQuery(ctx context.Context, userID, datasourceID, dsl string) bool {
	return true
}

func (PermissiveChecker) CanExecute(ctx context.Context, userID, datasourceID, dsl string) bool {
	return true
}
