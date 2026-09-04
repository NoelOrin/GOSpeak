package handler

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	msgSvc    *service.MessageService
	permSvc   *service.PermissionService
	roomSvc   *service.RoomService
	domainSvc *service.DomainService

	guestIs   func(userUUID string) bool
	guestCaps func(domainUUID string) (listen, speak, message bool)
}

// SetGuestPolicy 注入访客身份与能力开关；nil 表示不启用访客策略。
func (h *MessageHandler) SetGuestPolicy(
	isGuest func(userUUID string) bool,
	caps func(domainUUID string) (listen, speak, message bool),
) {
	h.guestIs = isGuest
	h.guestCaps = caps
}

func NewMessageHandler(msgSvc *service.MessageService, permSvc *service.PermissionService, roomSvc *service.RoomService, domainSvc *service.DomainService) *MessageHandler {
	return &MessageHandler{msgSvc: msgSvc, permSvc: permSvc, roomSvc: roomSvc, domainSvc: domainSvc}
}

func messageActorFromContext(c *gin.Context) (service.MessageActor, bool) {
	usernameVal, ok := c.Get("username")
	if !ok {
		return service.MessageActor{}, false
	}
	username, _ := usernameVal.(string)
	userUUID := currentUserUUID(c)
	if userUUID == "" {
		return service.MessageActor{}, false
	}
	return service.MessageActor{Identity: username, UserUUID: userUUID}, true
}

// guestMessagingAllowed 访客身份下，发消息能力以 GuestCaps 为权威（短路域角色权限）。
func (h *MessageHandler) guestMessagingAllowed(domainUUID, userUUID string) (bool, bool) {
	if h.guestIs == nil || !h.guestIs(userUUID) {
		return false, false
	}
	_, _, msg := h.guestCaps(domainUUID)
	return true, msg
}

func (h *MessageHandler) roomOf(c *gin.Context, roomUUID string) (*model.Room, bool) {
	if h.roomSvc == nil {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "room service unavailable")
		return nil, false
	}
	room, err := h.roomSvc.GetByUUID(roomUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return nil, false
	}
	if room == nil {
		pkg.Fail(c, pkg.NOT_FOUND, "room not found")
		return nil, false
	}
	return room, true
}

// List
// @Summary      消息历史
// @Description  分页获取房间消息历史，最新在前
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,before=string,limit=int}  true  "查询参数"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/list [post]
func (h *MessageHandler) List(c *gin.Context) {
	var req struct {
		RoomUUID string `json:"room_uuid" binding:"required"`
		Before   string `json:"before"`
		Limit    int    `json:"limit"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageRead, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	items, hasMore, nextBefore, err := h.msgSvc.ListHistory(req.RoomUUID, actor, req.Before, req.Limit, req.Password)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "has_more": hasMore, "next_before": nextBefore})
}

// Send
// Search
// @Summary      全文搜索文本房间消息
// @Description  按内容关键词搜索文本房间消息
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,query=string}  true  "搜索参数"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/search [post]
func (h *MessageHandler) Search(c *gin.Context) {
	var req struct {
		RoomUUID string `json:"room_uuid" binding:"required"`
		Query    string `json:"query" binding:"required"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageRead, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	items, err := h.msgSvc.Search(req.RoomUUID, actor, req.Query, req.Password)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items})
}

// @Summary      发送消息
// @Description  在文本房间发送一条消息
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,content=string,reply_to=string,mentions=[]string,client_nonce=string}  true  "消息内容"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/send [post]
func (h *MessageHandler) Send(c *gin.Context) {
	var req struct {
		RoomUUID    string   `json:"room_uuid" binding:"required"`
		Content     string   `json:"content" binding:"required"`
		ReplyTo     string   `json:"reply_to"`
		Mentions    []string `json:"mentions"`
		ClientNonce string   `json:"client_nonce"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if guest, allowed := h.guestMessagingAllowed(room.DomainUUID, actor.UserUUID); guest {
		if !allowed {
			pkg.Fail(c, pkg.FORBIDDEN, "guest messaging disabled")
			return
		}
	} else if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	dto, err := h.msgSvc.Send(req.RoomUUID, actor, req.Content, req.ReplyTo, req.ClientNonce, req.Mentions)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// Edit
// @Summary      编辑消息
// @Description  编辑自己发送的消息内容
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,message_uuid=string,content=string}  true  "编辑内容"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/edit [post]
func (h *MessageHandler) Edit(c *gin.Context) {
	var req struct {
		RoomUUID    string `json:"room_uuid" binding:"required"`
		MessageUUID string `json:"message_uuid" binding:"required"`
		Content     string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if guest, allowed := h.guestMessagingAllowed(room.DomainUUID, actor.UserUUID); guest {
		if !allowed {
			pkg.Fail(c, pkg.FORBIDDEN, "guest messaging disabled")
			return
		}
	} else if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	dto, err := h.msgSvc.Edit(req.RoomUUID, req.MessageUUID, actor, req.Content)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// Delete
// @Summary      删除消息
// @Description  删除一条消息（软删除）；管理员可删除他人消息
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,message_uuid=string}  true  "删除参数"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/delete [post]
func (h *MessageHandler) Delete(c *gin.Context) {
	var req struct {
		RoomUUID    string `json:"room_uuid" binding:"required"`
		MessageUUID string `json:"message_uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	_, isGuest := h.guestMessagingAllowed(room.DomainUUID, actor.UserUUID)
	if guest, allowed := h.guestMessagingAllowed(room.DomainUUID, actor.UserUUID); guest {
		if !allowed {
			pkg.Fail(c, pkg.FORBIDDEN, "guest messaging disabled")
			return
		}
	} else if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	canDeleteOthers := h.guestCaps == nil && domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageDeleteOthers, h.domainSvc, h.permSvc)
	if isGuest {
		canDeleteOthers = false
	}
	if err := h.msgSvc.Delete(req.RoomUUID, req.MessageUUID, actor, canDeleteOthers); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// React
// @Summary      消息反应
// @Description  给消息添加一个 emoji 反应
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,message_uuid=string,emoji=string}  true  "反应参数"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/react [post]
func (h *MessageHandler) React(c *gin.Context) {
	var req struct {
		RoomUUID    string `json:"room_uuid" binding:"required"`
		MessageUUID string `json:"message_uuid" binding:"required"`
		Emoji       string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if guest, allowed := h.guestMessagingAllowed(room.DomainUUID, actor.UserUUID); guest {
		if !allowed {
			pkg.Fail(c, pkg.FORBIDDEN, "guest messaging disabled")
			return
		}
	} else if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	if err := h.msgSvc.React(req.RoomUUID, req.MessageUUID, actor, req.Emoji); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// Unreact
// @Summary      取消消息反应
// @Description  移除消息上的一个 emoji 反应
// @Tags         消息
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{room_uuid=string,message_uuid=string,emoji=string}  true  "取消反应参数"
// @Success      200      {object}  pkg.Response
// @Router       /room/messages/unreact [post]
func (h *MessageHandler) Unreact(c *gin.Context) {
	var req struct {
		RoomUUID    string `json:"room_uuid" binding:"required"`
		MessageUUID string `json:"message_uuid" binding:"required"`
		Emoji       string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := messageActorFromContext(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if guest, allowed := h.guestMessagingAllowed(room.DomainUUID, actor.UserUUID); guest {
		if !allowed {
			pkg.Fail(c, pkg.FORBIDDEN, "guest messaging disabled")
			return
		}
	} else if !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
	if err := h.msgSvc.Unreact(req.RoomUUID, req.MessageUUID, actor, req.Emoji); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
