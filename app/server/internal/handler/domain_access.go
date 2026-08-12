package handler

import (
	"reflect"

	"github.com/gin-gonic/gin"
)

// domainPermissionChecker 校验用户在指定 Domain 的角色是否具备权限码。
type domainPermissionChecker interface {
	HasDomainPermission(domainUUID, userUUID, permCode string) bool
}

// globalPermissionChecker 校验用户全局角色是否具备权限码。
type globalPermissionChecker interface {
	HasPermission(role, permCode string) bool
}

// isNilChecker 判断 checker 是否为 nil 接口或 typed nil 的可 nil 类型。
func isNilChecker(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

// domainPermissionGranted 判定资源权限，作为 room/message handler 共享的资源权限判定入口：
// Domain 房间只认域角色权限；平台资源（domainUUID 为空）回退全局角色权限。
func domainPermissionGranted(
	c *gin.Context,
	domainUUID, permCode string,
	domainSvc domainPermissionChecker,
	permSvc globalPermissionChecker,
) bool {
	if domainUUID != "" {
		if isNilChecker(domainSvc) {
			return false
		}
		return domainSvc.HasDomainPermission(domainUUID, currentUserUUID(c), permCode)
	}
	if isNilChecker(permSvc) {
		return false
	}
	return permSvc.HasPermission(roleFromContext(c), permCode)
}
