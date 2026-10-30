package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/message"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type DiscussionHandler struct {
	repo     *discussion.Repository
	sessions *auth.SessionManager
	users    *user.Repository
	messages *message.Repository
}

func NewDiscussionHandler(repo *discussion.Repository, sessions *auth.SessionManager, users *user.Repository, messages *message.Repository) *DiscussionHandler {
	return &DiscussionHandler{repo: repo, sessions: sessions, users: users, messages: messages}
}

var (
	mentionPattern = regexp.MustCompile(`@\[\]\(/user/([0-9]+)\)`)
	nodeIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
)

func mentionedUIDs(content string) []int64 {
	matches := mentionPattern.FindAllStringSubmatch(content, -1)
	seen := make(map[int64]bool, len(matches))
	ids := make([]int64, 0, len(matches))
	for _, match := range matches {
		uid, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || uid <= 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		ids = append(ids, uid)
	}
	return ids
}

// notifyMentions mirrors Hydro's discussion reply notification: only real
// users are notified, the sender is not notified about their own mention, and
// the message is stored in the existing message collection as a richtext
// unread system message.
func (h *DiscussionHandler) notifyMentions(ctx context.Context, did bson.ObjectID, senderID int64, content string) error {
	if h.messages == nil || h.users == nil {
		return nil
	}
	ids := mentionedUIDs(content)
	if len(ids) == 0 {
		return nil
	}
	accounts, err := h.users.FindByIDs(ctx, ids)
	if err != nil {
		return err
	}
	recipients := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		if account.ID != senderID {
			recipients = append(recipients, account.ID)
		}
	}
	if len(recipients) == 0 {
		return nil
	}
	sender, err := h.users.FindByID(ctx, senderID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"message": "User {0} mentioned you in {1:link}",
		"params":  []string{sender.Uname, "/discuss/" + did.Hex()},
	})
	if err != nil {
		return err
	}
	_, err = h.messages.Send(ctx, 1, recipients, string(payload), message.FlagRichtext|message.FlagUnread)
	return err
}

func (h *DiscussionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/discussions", h.list)
	router.GET("/api/v1/discussion-nodes", h.listNodes)
	router.GET("/api/v1/discussion-nodes/:type/:name", h.getNode)
	router.GET("/api/v1/admin/discussion-nodes", h.adminListNodes)
	router.POST("/api/v1/admin/discussion-nodes", h.createNode)
	router.PATCH("/api/v1/admin/discussion-nodes/:id", h.updateNode)
	router.DELETE("/api/v1/admin/discussion-nodes/:id", h.deleteNode)
	router.POST("/api/v1/discussions", h.create)
	router.PATCH("/api/v1/discussions/:did", h.update)
	router.DELETE("/api/v1/discussions/:did", h.delete)
	router.PATCH("/api/v1/discussions/:did/lock", h.setLock)
	router.POST("/api/v1/discussions/:did/star", h.star)
	router.POST("/api/v1/discussions/:did/reaction", h.react)
	router.GET("/api/v1/discussions/:did", h.get)
	router.GET("/api/v1/discussions/:did/raw", h.raw)
	router.GET("/api/v1/discussions/:did/replies", h.replies)
	router.POST("/api/v1/discussions/:did/replies", h.createReply)
	router.PATCH("/api/v1/discussions/:did/replies/:rid", h.updateReply)
	router.DELETE("/api/v1/discussions/:did/replies/:rid", h.deleteReply)
}

func (h *DiscussionHandler) list(c *gin.Context) {
	parentType, _ := strconv.ParseInt(c.Query("parentType"), 10, 64)
	page, limit := parsePaging(c)
	items, total, err := h.repo.ListFiltered(c.Request.Context(), parentType, c.Query("parentId"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit},
	})
}

func (h *DiscussionHandler) listNodes(c *gin.Context) {
	items, err := h.repo.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussion nodes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *DiscussionHandler) adminListNodes(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	items, err := h.repo.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussion nodes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func nodeInput(c *gin.Context, requireID bool) (discussion.NodeInput, bool) {
	var input discussion.NodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return input, false
	}
	input.ID = strings.TrimSpace(input.ID)
	input.Title = strings.TrimSpace(input.Title)
	input.Category = strings.TrimSpace(input.Category)
	input.Pic = strings.TrimSpace(input.Pic)
	if requireID && !nodeIDPattern.MatchString(input.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node id must use 1-32 letters, digits, underscores or hyphens"})
		return input, false
	}
	if input.Title == "" || input.Category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and category are required"})
		return input, false
	}
	if len(input.Title) > 64 || len(input.Category) > 64 || len(input.Pic) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node fields are too long"})
		return input, false
	}
	return input, true
}

func (h *DiscussionHandler) createNode(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	input, ok := nodeInput(c, true)
	if !ok {
		return
	}
	node, err := h.repo.CreateNode(c.Request.Context(), u.ID, input)
	if err != nil {
		if errors.Is(err, discussion.ErrNodeExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "discussion node already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create discussion node"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": node})
}

func (h *DiscussionHandler) updateNode(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	input, ok := nodeInput(c, false)
	if !ok {
		return
	}
	input.ID = strings.TrimSpace(c.Param("id"))
	if !nodeIDPattern.MatchString(input.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid discussion node id"})
		return
	}
	node, err := h.repo.UpdateNode(c.Request.Context(), input.ID, input)
	if err != nil {
		if errors.Is(err, discussion.ErrNodeNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "discussion node not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update discussion node"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": node})
}

func (h *DiscussionHandler) deleteNode(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	nodeID := strings.TrimSpace(c.Param("id"))
	if !nodeIDPattern.MatchString(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid discussion node id"})
		return
	}
	if err := h.repo.DeleteNode(c.Request.Context(), nodeID); err != nil {
		switch {
		case errors.Is(err, discussion.ErrNodeNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "discussion node not found"})
		case errors.Is(err, discussion.ErrNodeInUse):
			c.JSON(http.StatusConflict, gin.H{"error": "discussion node is still in use"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete discussion node"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DiscussionHandler) requireAdmin(c *gin.Context) (*user.User, bool) {
	if h.sessions == nil || h.users == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil || u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return u, true
}

func (h *DiscussionHandler) getNode(c *gin.Context) {
	node, err := h.repo.ResolveNode(c.Request.Context(), c.Param("type"), c.Param("name"))
	if err != nil {
		if errors.Is(err, discussion.ErrNodeNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "discussion node not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load discussion node"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": node})
}

func (h *DiscussionHandler) create(c *gin.Context) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		NodeType  string `json:"nodeType"`
		NodeID    string `json:"nodeId"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Pin       bool   `json:"pin"`
		Highlight bool   `json:"highlight"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.NodeType = strings.TrimSpace(request.NodeType)
	request.NodeID = strings.TrimSpace(request.NodeID)
	request.Title = strings.TrimSpace(request.Title)
	if request.NodeID == "" || request.Title == "" || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node, title and content are required"})
		return
	}
	if len(request.Title) > discussion.MaxTitleLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is too long"})
		return
	}
	if len(request.Content) > discussion.MaxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is too long"})
		return
	}
	if (request.Pin || request.Highlight) && !h.isAdmin(c.Request.Context(), session.UserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	node, err := h.repo.ResolveNode(c.Request.Context(), request.NodeType, request.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "discussion node not found"})
		return
	}
	item, err := h.repo.Create(c.Request.Context(), node, session.UserID, request.Title, request.Content, request.Pin, request.Highlight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create discussion"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *DiscussionHandler) update(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	admin := h.isAdmin(c.Request.Context(), uid)
	if item.Owner != uid && !admin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var request discussion.EditInput
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and content are required"})
		return
	}
	if len(request.Title) > discussion.MaxTitleLength || len(request.Content) > discussion.MaxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title or content is too long"})
		return
	}
	if !admin && (request.Pin != item.Pin || request.Highlight != item.Highlight) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	request.Editor = uid
	updated, err := h.repo.Update(c.Request.Context(), c.Param("did"), request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update discussion"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": updated})
}

func (h *DiscussionHandler) delete(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	if item.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), c.Param("did")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete discussion"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DiscussionHandler) setLock(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	var request struct {
		Lock bool `json:"lock"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := h.repo.SetLock(c.Request.Context(), c.Param("did"), request.Lock)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

const (
	reactionNodeDiscussion = "did"
	reactionNodeReply      = "drid"
	maxReactionEmojiLength = 32
)

func validReactionEmoji(emoji string) bool {
	if emoji == "" || len(emoji) > maxReactionEmojiLength {
		return false
	}
	for _, char := range emoji {
		if char == '.' || char == '$' || char < 0x20 {
			return false
		}
	}
	return true
}

// react toggles one emoji reaction on a discussion or one of its replies,
// mirroring the upstream `operation=reaction` request.
func (h *DiscussionHandler) react(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	var request struct {
		ID       string `json:"id"`
		NodeType string `json:"nodeType"`
		Emoji    string `json:"emoji"`
		Reverse  bool   `json:"reverse"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	emoji := strings.TrimSpace(request.Emoji)
	if !validReactionEmoji(emoji) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid emoji"})
		return
	}
	target, err := bson.ObjectIDFromHex(request.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reaction target"})
		return
	}
	docType := int64(discussion.DocumentTypeDiscussion)
	switch request.NodeType {
	case reactionNodeDiscussion:
		if target != item.DocID {
			c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
			return
		}
	case reactionNodeReply:
		ok, err := h.repo.ReplyBelongsTo(c.Request.Context(), item.DocID, target)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load reply"})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
			return
		}
		docType = discussion.DocumentTypeDiscussionReply
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node type"})
		return
	}
	doc, self, err := h.repo.React(c.Request.Context(), docType, target, uid, emoji, request.Reverse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save reaction"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"doc":  gin.H{"react": doc},
		"sdoc": gin.H{"react": self},
	}})
}

// raw serves the current markdown of a topic, reply or tail reply, the list of
// stored revisions (`all=1`) and one revision (`time=<unix ms>`), mirroring the
// upstream discussion_raw handlers.
func (h *DiscussionHandler) raw(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	target := item.DocID
	if raw := strings.TrimSpace(c.Query("docId")); raw != "" {
		parsed, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid docId"})
			return
		}
		target = parsed
	}
	if c.Query("all") == "1" || c.Query("all") == "true" {
		entries, err := h.repo.History(c.Request.Context(), target, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load history"})
			return
		}
		history := make([]gin.H, 0, len(entries))
		for _, entry := range entries {
			history = append(history, gin.H{"uid": entry.UID, "time": entry.Time})
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"history": history}})
		return
	}
	if raw := strings.TrimSpace(c.Query("time")); raw != "" {
		millis, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time"})
			return
		}
		at := time.UnixMilli(millis)
		entries, err := h.repo.History(c.Request.Context(), target, &at)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load history"})
			return
		}
		if len(entries) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "revision not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"content": entries[0].Content}})
		return
	}
	content, err := h.repo.TargetContent(c.Request.Context(), item.DocID, target)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"content": content}})
}

func (h *DiscussionHandler) requireUser(c *gin.Context) (int64, bool) {
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

func (h *DiscussionHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	if h.sessions != nil {
		if session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" && session.UserID > 0 {
			firstView, err := h.repo.MarkViewed(c.Request.Context(), item.DocID, session.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to record discussion view"})
				return
			}
			if firstView {
				if err := h.repo.IncrementViews(c.Request.Context(), item.DocID); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to record discussion view"})
					return
				}
				item.Views++
			}
			starred, err := h.repo.Starred(c.Request.Context(), item.DocID, session.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load discussion status"})
				return
			}
			item.Starred = starred
			ids := []bson.ObjectID{item.DocID}
			replies, err := h.repo.ListReplies(c.Request.Context(), item.DocID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load discussion replies"})
				return
			}
			for _, reply := range replies {
				ids = append(ids, reply.ID)
			}
			reactions, err := h.repo.Reactions(c.Request.Context(), session.UserID, ids)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load reactions"})
				return
			}
			item.Reactions = reactions
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *DiscussionHandler) star(c *gin.Context) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	var request struct {
		Star bool `json:"star"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.repo.SetStar(c.Request.Context(), item.DocID, session.UserID, request.Star); err != nil {
		if err == discussion.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update discussion status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"starred": request.Star},
	})
}

func (h *DiscussionHandler) replies(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	items, err := h.repo.ListReplies(c.Request.Context(), item.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list replies"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *DiscussionHandler) createReply(c *gin.Context) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	if item.Lock {
		c.JSON(http.StatusConflict, gin.H{"error": "discussion is locked"})
		return
	}
	var request struct {
		Content string `json:"content"`
		ReplyTo string `json:"replyTo"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is required"})
		return
	}
	if len(request.Content) > discussion.MaxReplyLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is too long"})
		return
	}
	if request.ReplyTo != "" {
		replyID, err := bson.ObjectIDFromHex(request.ReplyTo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replyTo"})
			return
		}
		belongs, err := h.repo.ReplyBelongsTo(c.Request.Context(), item.ID, replyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load reply"})
			return
		}
		if !belongs {
			c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
			return
		}
		if err := h.notifyMentions(c.Request.Context(), item.ID, session.UserID, request.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to send mention notification"})
			return
		}
		if _, tail, err := h.repo.AddTailReply(c.Request.Context(), item.ID, replyID, session.UserID, request.Content); err != nil {
			if err == discussion.ErrNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create reply"})
			return
		} else {
			c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": tail})
			return
		}
	}
	if err := h.notifyMentions(c.Request.Context(), item.ID, session.UserID, request.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to send mention notification"})
		return
	}
	reply, err := h.repo.AddReply(c.Request.Context(), item.ID, session.UserID, request.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create reply"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": reply})
}

func (h *DiscussionHandler) updateReply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, replyID, ok := h.replyContext(c)
	if !ok {
		return
	}
	owner, err := h.repo.ReplyOwner(c.Request.Context(), item.ID, replyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var request struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is required"})
		return
	}
	if len(request.Content) > discussion.MaxReplyLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is too long"})
		return
	}
	reply, tail, err := h.repo.UpdateReply(c.Request.Context(), item.ID, replyID, uid, request.Content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if tail != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tail})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": reply})
}

func (h *DiscussionHandler) deleteReply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, replyID, ok := h.replyContext(c)
	if !ok {
		return
	}
	owner, err := h.repo.ReplyOwner(c.Request.Context(), item.ID, replyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if owner != uid && item.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.DeleteReply(c.Request.Context(), item.ID, replyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete reply"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DiscussionHandler) replyContext(c *gin.Context) (*discussion.Discussion, bson.ObjectID, bool) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return nil, bson.NilObjectID, false
	}
	replyID, err := bson.ObjectIDFromHex(c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reply id"})
		return nil, bson.NilObjectID, false
	}
	return item, replyID, true
}

func (h *DiscussionHandler) isAdmin(ctx context.Context, uid int64) bool {
	if h.users == nil {
		return false
	}
	u, err := h.users.FindByID(ctx, uid)
	return err == nil && u.Priv&1 != 0
}
