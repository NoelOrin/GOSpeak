package handler

import (
	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// DeleteObject
// @Summary      删除存储对象
// @Description  管理员删除存储中的文件
// @Tags         存储
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{key=string}  true  "对象键"
// @Success      200   {object}  pkg.Response
// @Router       /storage/delete [post]
func (h *StorageHandler) DeleteObject(c *gin.Context) {
	var req struct {
		Key string `json:"key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.storageService.DeleteObject(req.Key); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
