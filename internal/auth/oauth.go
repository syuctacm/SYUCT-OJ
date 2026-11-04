package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/user"
)

const oauthStateCookie = "oauth_state"

// OAuthProviderInfo is the public provider metadata rendered by the upstream
// login and security templates.
type OAuthProviderInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Text        string `json:"text"`
	Icon        string `json:"icon,omitempty"`
	CanRegister bool   `json:"canRegister"`
}

// OAuthLink is one persisted external identity in Hydro's `oauth` collection.
type OAuthLink struct {
	Platform string `bson:"platform" json:"platform"`
	ID       string `bson:"id" json:"id"`
	UID      int64  `bson:"uid" json:"uid"`
}

type oauthState struct {
	ID        string    `bson:"_id"`
	Provider  string    `bson:"provider"`
	Redirect  string    `bson:"redirect"`
	BindUID   int64     `bson:"bindUid,omitempty"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type oauthProvider struct {
	OAuthProviderInfo
	AuthorizeURL string
	TokenURL     string
	UserURL      string
	EmailsURL    string
	Scope        string
	ClientID     string
	ClientSecret string
}

type oauthProfile struct {
	ID          string
	Email       string
	Username    string
	DisplayName string
	Avatar      string
	Bio         string
}

// OAuthHandler implements a deliberately small provider interface. GitHub is
// the first provider; the same flow can add OIDC providers without changing
// the account/session model.
type OAuthHandler struct {
	db        *mongo.Database
	users     *user.Repository
	sessions  *SessionManager
	links     *mongo.Collection
	states    *mongo.Collection
	providers map[string]oauthProvider
	client    *http.Client
	baseURL   string
}

func NewOAuthHandler(db *mongo.Database, users *user.Repository, sessions *SessionManager, cfg config.Config) *OAuthHandler {
	providers := make(map[string]oauthProvider)
	if cfg.OAuthGitHubClientID != "" && cfg.OAuthGitHubClientSecret != "" {
		providers["github"] = oauthProvider{
			OAuthProviderInfo: OAuthProviderInfo{
				ID: "github", Name: cfg.OAuthGitHubName, Text: "Login with " + cfg.OAuthGitHubName,
				CanRegister: cfg.OAuthGitHubCanRegister,
			},
			AuthorizeURL: cfg.OAuthGitHubAuthorizeURL, TokenURL: cfg.OAuthGitHubTokenURL,
			UserURL: cfg.OAuthGitHubUserURL, EmailsURL: cfg.OAuthGitHubEmailsURL,
			Scope: cfg.OAuthGitHubScope, ClientID: cfg.OAuthGitHubClientID, ClientSecret: cfg.OAuthGitHubClientSecret,
		}
	}
	return &OAuthHandler{
		db: db, users: users, sessions: sessions,
		links: db.Collection("oauth"), states: db.Collection("oauth_state"),
		providers: providers, client: &http.Client{Timeout: 15 * time.Second},
		baseURL: strings.TrimRight(cfg.OAuthCallbackBaseURL, "/"),
	}
}

func (h *OAuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/auth/oauth/providers", h.listProviders)
	router.GET("/api/v1/auth/oauth/:provider/login", h.login)
	router.GET("/api/v1/auth/oauth/:provider/callback", h.callback)
	router.GET("/api/v1/user/security/oauth", h.listLinks)
	router.DELETE("/api/v1/user/security/oauth/:provider", h.unlink)
}

func (h *OAuthHandler) listProviders(c *gin.Context) {
	items := make([]OAuthProviderInfo, 0, len(h.providers))
	for _, provider := range h.providers {
		items = append(items, provider.OAuthProviderInfo)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": items})
}

func (h *OAuthHandler) login(c *gin.Context) {
	provider, ok := h.providers[c.Param("provider")]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "oauth provider is not configured"})
		return
	}
	bind := c.Query("bind") == "1"
	bindUID := int64(0)
	if bind {
		uid, ok := h.currentUserID(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		bindUID = uid
	}
	state, err := randomOAuthState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create oauth state"})
		return
	}
	redirect := safeOAuthRedirect(c.Query("redirect"))
	if bind {
		redirect = "/home/security"
	}
	if _, err := h.states.InsertOne(c.Request.Context(), oauthState{
		ID: state, Provider: provider.ID, Redirect: redirect, BindUID: bindUID,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save oauth state"})
		return
	}
	h.setStateCookie(c, state)
	u, err := url.Parse(provider.AuthorizeURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid oauth provider endpoint"})
		return
	}
	query := u.Query()
	query.Set("client_id", provider.ClientID)
	query.Set("redirect_uri", h.callbackURL(c, provider.ID))
	query.Set("state", state)
	if provider.Scope != "" {
		query.Set("scope", provider.Scope)
	}
	u.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func (h *OAuthHandler) callback(c *gin.Context) {
	provider, ok := h.providers[c.Param("provider")]
	if !ok {
		h.fail(c, "oauth provider is not configured")
		return
	}
	state := c.Query("state")
	cookie, err := c.Cookie(oauthStateCookie)
	if err != nil || subtle.ConstantTimeCompare([]byte(state), []byte(cookie)) != 1 {
		h.fail(c, "oauth state is invalid")
		return
	}
	h.clearStateCookie(c)
	var saved oauthState
	err = h.states.FindOneAndDelete(c.Request.Context(), bson.M{
		"_id": state, "provider": provider.ID, "expiresAt": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&saved)
	if err != nil {
		h.fail(c, "oauth state is invalid or expired")
		return
	}
	if strings.TrimSpace(c.Query("code")) == "" {
		h.fail(c, "oauth code is missing")
		return
	}
	profile, err := h.fetchGitHubProfile(c.Request.Context(), provider, c.Query("code"), state, h.callbackURL(c, provider.ID))
	if err != nil {
		h.fail(c, err.Error())
		return
	}
	if saved.BindUID > 0 {
		current, ok := h.currentUserID(c)
		if !ok || current != saved.BindUID {
			h.fail(c, "oauth binding session is invalid")
			return
		}
		if err := h.link(c.Request.Context(), provider.ID, profile.ID, saved.BindUID); err != nil {
			h.fail(c, err.Error())
			return
		}
		c.Redirect(http.StatusFound, safeOAuthRedirect(saved.Redirect))
		return
	}
	account, err := h.accountForProfile(c.Request.Context(), provider, profile)
	if err != nil {
		h.fail(c, err.Error())
		return
	}
	if user.IsBanned(account) || account.Deleted || (account.DeleteAt != nil && !account.DeleteAt.After(time.Now())) {
		h.fail(c, "account is unavailable")
		return
	}
	if err := h.link(c.Request.Context(), provider.ID, profile.ID, account.ID); err != nil {
		h.fail(c, err.Error())
		return
	}
	token, err := h.sessions.IssueForUserMeta(c.Request.Context(), account.ID, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.fail(c, "unable to create session")
		return
	}
	SetSessionCookie(c, token, h.sessions.TTL())
	_ = h.users.RecordLogin(c.Request.Context(), account.ID, c.ClientIP())
	c.Redirect(http.StatusFound, safeOAuthRedirect(saved.Redirect))
}

func (h *OAuthHandler) listLinks(c *gin.Context) {
	uid, ok := h.currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	cursor, err := h.links.Find(c.Request.Context(), bson.M{"uid": uid})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load oauth links"})
		return
	}
	defer cursor.Close(c.Request.Context())
	items := make([]OAuthLink, 0)
	if err := cursor.All(c.Request.Context(), &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load oauth links"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items}})
}

func (h *OAuthHandler) unlink(c *gin.Context) {
	uid, ok := h.currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if _, ok := h.providers[c.Param("provider")]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "oauth provider is not configured"})
		return
	}
	if _, err := h.links.DeleteOne(c.Request.Context(), bson.M{"platform": c.Param("provider"), "uid": uid}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to unlink oauth account"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *OAuthHandler) accountForProfile(ctx context.Context, provider oauthProvider, profile oauthProfile) (*user.User, error) {
	var link OAuthLink
	err := h.links.FindOne(ctx, bson.M{"platform": provider.ID, "id": profile.ID}).Decode(&link)
	if err == nil {
		if account, findErr := h.users.FindByID(ctx, link.UID); findErr == nil {
			return account, nil
		}
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	if account, findErr := h.users.FindByMail(ctx, profile.Email); findErr == nil {
		return account, nil
	} else if !errors.Is(findErr, user.ErrNotFound) {
		return nil, findErr
	}
	if !provider.CanRegister {
		return nil, fmt.Errorf("no bound account found")
	}
	username := h.availableUsername(ctx, profile)
	password, err := randomOAuthState()
	if err != nil {
		return nil, err
	}
	return h.users.Create(ctx, user.RegisterInput{
		Mail: profile.Email, Username: username, Password: password,
		DisplayName: profile.DisplayName, Avatar: profile.Avatar, Bio: profile.Bio,
	})
}

func (h *OAuthHandler) availableUsername(ctx context.Context, profile oauthProfile) string {
	base := sanitizeUsername(profile.Username)
	if base == "" {
		base = "oauth"
	}
	if len([]rune(base)) < 3 {
		base = "user_" + base
	}
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d", base, i+1)
		}
		if _, err := h.users.FindByUsername(ctx, candidate); errors.Is(err, user.ErrNotFound) {
			return candidate
		}
	}
	return "oauth_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (h *OAuthHandler) link(ctx context.Context, platform, id string, uid int64) error {
	var existing OAuthLink
	err := h.links.FindOne(ctx, bson.M{"platform": platform, "id": id}).Decode(&existing)
	if err == nil && existing.UID != uid {
		return fmt.Errorf("this oauth account is already bound to another user")
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	_, err = h.links.UpdateOne(ctx,
		bson.M{"platform": platform, "id": id},
		bson.M{"$set": bson.M{"uid": uid}},
		options.UpdateOne().SetUpsert(true))
	return err
}

func (h *OAuthHandler) fetchGitHubProfile(ctx context.Context, provider oauthProvider, code, state, redirectURI string) (oauthProfile, error) {
	form := url.Values{
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"state":         {state},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthProfile{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := h.doJSON(request, &tokenResponse); err != nil {
		return oauthProfile{}, fmt.Errorf("oauth token exchange failed: %w", err)
	}
	if tokenResponse.Error != "" {
		return oauthProfile{}, fmt.Errorf("oauth token exchange failed: %s", tokenResponse.Description)
	}
	if tokenResponse.AccessToken == "" {
		return oauthProfile{}, fmt.Errorf("oauth token exchange returned no access token")
	}
	userRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, provider.UserURL, nil)
	userRequest.Header.Set("Accept", "application/json")
	userRequest.Header.Set("Authorization", "token "+tokenResponse.AccessToken)
	var githubUser struct {
		ID        json.Number `json:"id"`
		Login     string      `json:"login"`
		Name      string      `json:"name"`
		Email     string      `json:"email"`
		AvatarURL string      `json:"avatar_url"`
		Bio       string      `json:"bio"`
	}
	if err := h.doJSON(userRequest, &githubUser); err != nil {
		return oauthProfile{}, fmt.Errorf("oauth user request failed: %w", err)
	}
	email := strings.TrimSpace(githubUser.Email)
	if email == "" && provider.EmailsURL != "" {
		emailRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, provider.EmailsURL, nil)
		emailRequest.Header.Set("Accept", "application/json")
		emailRequest.Header.Set("Authorization", "token "+tokenResponse.AccessToken)
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := h.doJSON(emailRequest, &emails); err == nil {
			for _, item := range emails {
				if item.Primary && item.Verified {
					email = item.Email
					break
				}
			}
			if email == "" {
				for _, item := range emails {
					if item.Verified {
						email = item.Email
						break
					}
				}
			}
		}
	}
	if !user.ValidEmail(email) {
		return oauthProfile{}, fmt.Errorf("oauth provider did not return a verified email")
	}
	return oauthProfile{
		ID: githubUser.ID.String(), Email: email, Username: githubUser.Login,
		DisplayName: githubUser.Name, Avatar: githubUser.AvatarURL, Bio: githubUser.Bio,
	}, nil
}

func (h *OAuthHandler) doJSON(request *http.Request, target any) error {
	response, err := h.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("provider returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func (h *OAuthHandler) currentUserID(c *gin.Context) (int64, bool) {
	session, ok := h.sessions.GetContext(c.Request.Context(), Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	account, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil || user.IsBanned(account) || account.Deleted || (account.DeleteAt != nil && !account.DeleteAt.After(time.Now())) {
		return 0, false
	}
	return session.UserID, true
}

func (h *OAuthHandler) callbackURL(c *gin.Context, provider string) string {
	base := h.configuredServerURL(c)
	if base == "" {
		base = h.baseURL
	}
	if base == "" {
		base = oauthRequestOrigin(c)
	}
	return base + "/api/v1/auth/oauth/" + url.PathEscape(provider) + "/callback"
}

func (h *OAuthHandler) configuredServerURL(c *gin.Context) string {
	if h.db == nil || c == nil {
		return ""
	}
	var doc struct {
		Value any `bson:"value"`
	}
	if err := h.db.Collection("system").FindOne(c.Request.Context(), bson.M{"_id": "server.url"}).Decode(&doc); err != nil {
		return ""
	}
	value, ok := doc.Value.(string)
	if !ok {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		parsed, err := url.Parse(value)
		if err == nil && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.Contains(parsed.Path, "..") {
			return strings.TrimRight(value, "/")
		}
		return ""
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.Contains(value, "..") {
		return oauthRequestOrigin(c) + strings.TrimRight(value, "/")
	}
	return ""
}

func oauthRequestOrigin(c *gin.Context) string {
	scheme := "http"
	if c != nil && (c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")) {
		scheme = "https"
	}
	if c == nil {
		return scheme + "://localhost"
	}
	return scheme + "://" + c.Request.Host
}

func (h *OAuthHandler) setStateCookie(c *gin.Context, state string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: oauthStateCookie, Value: state, Path: "/", MaxAge: 600,
		HttpOnly: true, Secure: CookieSecure(), SameSite: http.SameSiteLaxMode,
	})
}

func (h *OAuthHandler) clearStateCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: CookieSecure(), SameSite: http.SameSiteLaxMode,
	})
}

func (h *OAuthHandler) fail(c *gin.Context, message string) {
	c.Redirect(http.StatusFound, "/login?oauth_error="+url.QueryEscape(message))
}

func safeOAuthRedirect(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/login") {
		return "/"
	}
	return value
}

func sanitizeUsername(value string) string {
	value = strings.TrimSpace(value)
	var output strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '-' {
			output.WriteRune(char)
		} else if unicode.IsSpace(char) {
			output.WriteByte('_')
		}
	}
	result := strings.Trim(output.String(), "_-")
	if len([]rune(result)) > 24 {
		result = string([]rune(result)[:24])
	}
	return result
}

func randomOAuthState() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
