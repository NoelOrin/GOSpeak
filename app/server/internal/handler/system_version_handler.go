package handler

import (
	"github.com/gin-gonic/gin"

	"GOSpeak/internal/buildinfo"
	"GOSpeak/internal/pkg"
)

// SystemVersionHandler 暴露运行中服务端的版本信息。
//
// 版本号对客户端是公开信息：「关于」页与登录页都需要在未登录状态下展示
// 后端版本。更新检查（/system/update/*）仍保持 admin-only——能否比对并
// 下载新版本属于运维权限，与「现在跑的是哪个版本」不是同一回事。
type SystemVersionHandler struct{}

func NewSystemVersionHandler() *SystemVersionHandler {
	return &SystemVersionHandler{}
}

// Version 返回运行中服务端的版本号与 commit（构建时注入，dev 为源码构建）。
// @Summary      查询服务端版本
// @Description  返回运行中服务端的版本号与 commit（构建时注入，dev 为源码构建）；公开端点，无需鉴权
// @Tags         系统
// @Produce      json
// @Success      200  {object}  pkg.Response
// @Router       /system/version [get]
func (h *SystemVersionHandler) Version(c *gin.Context) {
	pkg.Success(c, gin.H{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
	})
}
