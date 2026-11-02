package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"
)

const (
	settingBulletin        = "bulletin"
	settingServerName      = "server.name"
	settingNavLogo         = "ui-default.nav_logo_dark"
	settingFooterExtraHTML = "ui-default.footer_extra_html"
	settingScratchpad      = "ui-default.enableScratchpad"
	settingGravatarURL     = "avatar.gravatar_url"
	settingHomepage        = "hydrooj.homepage"
	settingAbout           = "ui-default.about"
)

const DefaultSiteName = "SYUCT ACM OJ"
const DefaultNavLogo = "/nav_logo_dark.png"
const DefaultFooterExtraHTML = "沈阳化工大学 ACM-ICPC 集训队"
const DefaultGravatarURL = "//cn.gravatar.com/avatar/"
const DefaultHomepageYAML = `- width: 9
  bulletin: true
  contest: 5
  homework: 10
  training: 10
  ranking: 10
  discussion: 20
- width: 3
  checkin: true
  countdown: true
  hitokoto: true
  starred_problems: 50
  recent_problems: 10
  discussion_nodes: true
  problem_search: true
  suggestion: true
`
const DefaultAboutMarkdown = `# about 关于 {{ name }}

{{ name }} 是沈阳化工大学 ACM-ICPC 集训队使用的在线评测与训练平台，提供题库、比赛、作业、训练、讨论和提交记录等功能。

# contact 联系我们

如需反馈问题、申请权限或联系管理员，请前往[讨论区](/discuss)。

# admin 管理与维护

站点由集训队管理员维护。管理员可以在控制面板中管理用户、题目、比赛、训练和系统公告。
`

var siteSettingKeys = []string{
	settingBulletin, settingServerName, settingNavLogo, settingFooterExtraHTML,
	settingScratchpad, settingGravatarURL, settingHomepage, settingAbout,
}

type SiteSettings struct {
	Bulletin         string `json:"bulletin"`
	SiteName         string `json:"siteName"`
	NavLogo          string `json:"navLogo"`
	FooterExtraHTML  string `json:"footerExtraHtml"`
	EnableScratchpad bool   `json:"enableScratchpad"`
	GravatarURL      string `json:"gravatarUrl"`
	HomepageYAML     string `json:"homepageYaml"`
	AboutMarkdown    string `json:"aboutMarkdown"`
}

func DefaultSiteSettings() SiteSettings {
	return SiteSettings{
		SiteName: DefaultSiteName, NavLogo: DefaultNavLogo,
		FooterExtraHTML: DefaultFooterExtraHTML, EnableScratchpad: true,
		GravatarURL: DefaultGravatarURL, HomepageYAML: DefaultHomepageYAML,
		AboutMarkdown: DefaultAboutMarkdown,
	}
}

// SettingsHandler serves the public subset of the `system` key/value store,
// which is the same shape Hydro's model/system.ts uses.
type SettingsHandler struct {
	db *mongo.Database
}

func NewSettingsHandler(db *mongo.Database) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/settings", h.publicSettings)
}

func (h *SettingsHandler) publicSettings(c *gin.Context) {
	values, err := readSiteSettings(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": values})
}

func readSiteSettings(ctx context.Context, db *mongo.Database) (SiteSettings, error) {
	values := DefaultSiteSettings()
	cursor, err := db.Collection("system").Find(ctx, bson.M{"_id": bson.M{"$in": siteSettingKeys}},
		options.Find().SetProjection(bson.M{"value": 1}))
	if err != nil {
		return values, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Value any    `bson:"value"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return values, err
		}
		setSiteSettingValue(&values, doc.ID, doc.Value)
	}
	return values, cursor.Err()
}

func setSiteSettingValue(values *SiteSettings, key string, value any) {
	text, isText := value.(string)
	switch key {
	case settingBulletin:
		if isText {
			values.Bulletin = text
		}
	case settingServerName:
		if isText && strings.TrimSpace(text) != "" {
			values.SiteName = text
		}
	case settingNavLogo:
		if isText && strings.TrimSpace(text) != "" {
			values.NavLogo = text
		}
	case settingFooterExtraHTML:
		if isText {
			values.FooterExtraHTML = text
		}
	case settingScratchpad:
		if boolValue, ok := anyBool(value); ok {
			values.EnableScratchpad = boolValue
		}
	case settingGravatarURL:
		if isText && strings.TrimSpace(text) != "" {
			values.GravatarURL = text
		}
	case settingHomepage:
		if isText {
			values.HomepageYAML = text
		} else if value != nil {
			if encoded, err := yaml.Marshal(value); err == nil {
				values.HomepageYAML = string(encoded)
			}
		}
	case settingAbout:
		if isText {
			values.AboutMarkdown = text
		}
	}
}

func anyBool(value any) (bool, bool) {
	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Kind() == reflect.Bool {
		return reflected.Bool(), true
	}
	return false, false
}

func siteSettingUpdates(input SiteSettingsInput) map[string]any {
	updates := make(map[string]any)
	if input.Bulletin != nil {
		updates[settingBulletin] = *input.Bulletin
	}
	if input.SiteName != nil {
		updates[settingServerName] = *input.SiteName
	}
	if input.NavLogo != nil {
		updates[settingNavLogo] = *input.NavLogo
	}
	if input.FooterExtraHTML != nil {
		updates[settingFooterExtraHTML] = *input.FooterExtraHTML
	}
	if input.EnableScratchpad != nil {
		updates[settingScratchpad] = *input.EnableScratchpad
	}
	if input.GravatarURL != nil {
		updates[settingGravatarURL] = *input.GravatarURL
	}
	if input.HomepageYAML != nil {
		updates[settingHomepage] = *input.HomepageYAML
	}
	if input.AboutMarkdown != nil {
		updates[settingAbout] = *input.AboutMarkdown
	}
	return updates
}

func writeSiteSettings(ctx context.Context, db *mongo.Database, updates map[string]any) error {
	for key, value := range updates {
		if _, err := db.Collection("system").UpdateOne(ctx, bson.M{"_id": key},
			bson.M{"$set": bson.M{"value": value}}, options.UpdateOne().SetUpsert(true)); err != nil {
			return fmt.Errorf("save setting %s: %w", key, err)
		}
	}
	return nil
}

type SiteSettingsInput struct {
	Bulletin         *string `json:"bulletin"`
	SiteName         *string `json:"siteName"`
	NavLogo          *string `json:"navLogo"`
	FooterExtraHTML  *string `json:"footerExtraHtml"`
	EnableScratchpad *bool   `json:"enableScratchpad"`
	GravatarURL      *string `json:"gravatarUrl"`
	HomepageYAML     *string `json:"homepageYaml"`
	AboutMarkdown    *string `json:"aboutMarkdown"`
}

func validHomepageYAML(value string) bool {
	var columns []map[string]any
	if err := yaml.Unmarshal([]byte(value), &columns); err != nil || len(columns) == 0 {
		return false
	}
	for _, column := range columns {
		width, ok := column["width"]
		if !ok {
			return false
		}
		switch number := width.(type) {
		case int:
			if number < 1 || number > 12 {
				return false
			}
		case int64:
			if number < 1 || number > 12 {
				return false
			}
		default:
			return false
		}
		for key, section := range column {
			if key == "width" {
				continue
			}
			switch section.(type) {
			case bool, int, int64, uint64, float64, string:
			default:
				return false
			}
		}
	}
	return true
}

func validateSiteSettings(input SiteSettingsInput) error {
	checkLength := func(value *string, max int, label string) error {
		if value != nil && len(*value) > max {
			return fmt.Errorf("%s is too long", label)
		}
		return nil
	}
	if input.SiteName != nil && strings.TrimSpace(*input.SiteName) == "" {
		return fmt.Errorf("site name is required")
	}
	if input.NavLogo != nil && strings.TrimSpace(*input.NavLogo) == "" {
		return fmt.Errorf("nav logo is required")
	}
	if input.GravatarURL != nil && strings.TrimSpace(*input.GravatarURL) == "" {
		return fmt.Errorf("gravatar url is required")
	}
	if input.NavLogo != nil && !validAssetURL(*input.NavLogo) {
		return fmt.Errorf("invalid nav logo")
	}
	if input.GravatarURL != nil && !validAssetURL(*input.GravatarURL) {
		return fmt.Errorf("invalid gravatar url")
	}
	if input.HomepageYAML != nil && !validHomepageYAML(*input.HomepageYAML) {
		return fmt.Errorf("invalid homepage yaml")
	}
	for _, item := range []struct {
		value *string
		max   int
		label string
	}{
		{input.Bulletin, 20000, "bulletin"}, {input.SiteName, 100, "site name"},
		{input.NavLogo, 512, "nav logo"}, {input.FooterExtraHTML, 20000, "footer html"},
		{input.GravatarURL, 512, "gravatar url"}, {input.HomepageYAML, 20000, "homepage yaml"},
		{input.AboutMarkdown, 50000, "about markdown"},
	} {
		if err := checkLength(item.value, item.max, item.label); err != nil {
			return err
		}
	}
	return nil
}

func validAssetURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n\t") {
		return false
	}
	if strings.HasPrefix(value, "//") {
		parsed, err := url.Parse("https:" + value)
		return err == nil && parsed.Host != "" && parsed.User == nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.IsAbs() {
		return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
	}
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//")
}
