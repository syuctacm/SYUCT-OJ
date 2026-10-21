package handler

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/message"
	"github.com/syuctacm/syuct-oj/internal/user"
)

// MessageHandler serves the /home/messages messagepad.
type MessageHandler struct {
	repo     *message.Repository
	sessions *auth.SessionManager
	users    *user.Repository
}

func NewMessageHandler(repo *message.Repository, sessions *auth.SessionManager, users *user.Repository) *MessageHandler {
	return &MessageHandler{repo: repo, sessions: sessions, users: users}
}

func (h *MessageHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/messages", h.list)
	router.POST("/api/v1/messages", h.send)
	router.DELETE("/api/v1/messages/:mid", h.delete)
}

type messageDialogue struct {
	ID       int64             `json:"id"`
	User     messagePeer       `json:"user"`
	Messages []message.Message `json:"messages"`
}

type messagePeer struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

// list groups messages into one dialogue per peer and clears the unread
// counter, mirroring HomeMessagesHandler upstream.
func (h *MessageHandler) list(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	items, err := h.repo.ListByUser(c.Request.Context(), uid, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load messages"})
		return
	}
	grouped := make(map[int64]*messageDialogue)
	peerIDs := make([]int64, 0)
	seen := map[int64]bool{}
	addPeer := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			peerIDs = append(peerIDs, id)
		}
	}
	for _, item := range items {
		peers := item.To
		if item.From != uid {
			peers = []int64{item.From}
		}
		for _, peer := range peers {
			if peer == uid {
				// Messages to yourself stay in their own dialogue.
			}
			dialogue, ok := grouped[peer]
			if !ok {
				dialogue = &messageDialogue{ID: peer, Messages: []message.Message{}}
				grouped[peer] = dialogue
			}
			dialogue.Messages = append(dialogue.Messages, item)
			addPeer(peer)
		}
	}
	peers := map[int64]messagePeer{}
	if len(peerIDs) > 0 && h.users != nil {
		accounts, err := h.users.FindByIDs(c.Request.Context(), peerIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load messages"})
			return
		}
		for _, account := range accounts {
			peers[account.ID] = messagePeer{
				ID: account.ID, Username: account.Uname,
				DisplayName: account.DisplayName, Avatar: account.Avatar,
			}
		}
	}
	dialogues := make([]messageDialogue, 0, len(grouped))
	for id, dialogue := range grouped {
		// Items arrive newest first; the pad renders oldest first.
		for left, right := 0, len(dialogue.Messages)-1; left < right; left, right = left+1, right-1 {
			dialogue.Messages[left], dialogue.Messages[right] = dialogue.Messages[right], dialogue.Messages[left]
		}
		dialogue.User = peers[id]
		dialogues = append(dialogues, *dialogue)
	}
	sort.SliceStable(dialogues, func(i, j int) bool {
		left, right := dialogues[i], dialogues[j]
		if len(left.Messages) == 0 || len(right.Messages) == 0 {
			return len(left.Messages) > 0
		}
		return left.Messages[len(left.Messages)-1].ID.Hex() > right.Messages[len(right.Messages)-1].ID.Hex()
	})
	unread, _ := h.repo.Unread(c.Request.Context(), uid)
	_ = h.repo.MarkRead(c.Request.Context(), uid)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": dialogues, "unread": unread,
	}})
}

func (h *MessageHandler) send(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var request struct {
		UID     int64  `json:"uid"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.UID <= 0 || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid and content are required"})
		return
	}
	target, err := h.users.FindByID(c.Request.Context(), request.UID)
	if err != nil || target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	item, err := h.repo.Send(c.Request.Context(), uid, []int64{request.UID}, request.Content, message.FlagUnread)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to send message"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *MessageHandler) delete(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	id, err := bson.ObjectIDFromHex(c.Param("mid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id, uid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MessageHandler) requireUser(c *gin.Context) (int64, bool) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	return session.UserID, true
}
