package handler

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/syuctacm/syuct-oj/internal/judge"
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
	settingLanguages       = "hydrooj.langs"

	settingLimitUserFiles       = "limit.user_files"
	settingLimitUserFileSize    = "limit.user_files_size"
	settingLimitContestFiles    = "limit.contest_files"
	settingLimitContestFileSize = "limit.contest_files_size"
	settingLimitProblemFiles    = "limit.problem_files_max"
	settingLimitProblemFileSize = "limit.problem_files_max_size"
	settingLimitCodeLength      = "limit.codelength"

	settingPaginationProblem    = "pagination.problem"
	settingPaginationContest    = "pagination.contest"
	settingPaginationDiscussion = "pagination.discussion"
	settingPaginationRecord     = "pagination.record"
	settingPaginationRanking    = "pagination.ranking"
	settingPaginationSolution   = "pagination.solution"
	settingPaginationTraining   = "pagination.training"
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
	settingScratchpad, settingGravatarURL, settingHomepage, settingAbout, settingLanguages,
	settingPaginationProblem, settingPaginationContest, settingPaginationDiscussion,
	settingPaginationRecord, settingPaginationRanking, settingPaginationSolution,
	settingPaginationTraining,
	settingLimitUserFiles, settingLimitUserFileSize, settingLimitContestFiles,
	settingLimitContestFileSize, settingLimitProblemFiles, settingLimitProblemFileSize,
	settingLimitCodeLength,
}

type SiteSettings struct {
	Bulletin             string `json:"bulletin"`
	SiteName             string `json:"siteName"`
	NavLogo              string `json:"navLogo"`
	FooterExtraHTML      string `json:"footerExtraHtml"`
	EnableScratchpad     bool   `json:"enableScratchpad"`
	GravatarURL          string `json:"gravatarUrl"`
	HomepageYAML         string `json:"homepageYaml"`
	AboutMarkdown        string `json:"aboutMarkdown"`
	LanguageYAML         string `json:"languageYaml"`
	PaginationProblem    int    `json:"paginationProblem"`
	PaginationContest    int    `json:"paginationContest"`
	PaginationDiscussion int    `json:"paginationDiscussion"`
	PaginationRecord     int    `json:"paginationRecord"`
	PaginationRanking    int    `json:"paginationRanking"`
	PaginationSolution   int    `json:"paginationSolution"`
	PaginationTraining   int    `json:"paginationTraining"`
	LimitUserFiles       int    `json:"limitUserFiles"`
	LimitUserFileSize    int64  `json:"limitUserFileSize"`
	LimitContestFiles    int    `json:"limitContestFiles"`
	LimitContestFileSize int64  `json:"limitContestFileSize"`
	LimitProblemFiles    int    `json:"limitProblemFiles"`
	LimitProblemFileSize int64  `json:"limitProblemFileSize"`
	LimitCodeLength      int64  `json:"limitCodeLength"`
}

func DefaultSiteSettings() SiteSettings {
	return SiteSettings{
		SiteName: DefaultSiteName, NavLogo: DefaultNavLogo,
		FooterExtraHTML: DefaultFooterExtraHTML, EnableScratchpad: true,
		GravatarURL: DefaultGravatarURL, HomepageYAML: DefaultHomepageYAML,
		AboutMarkdown:     DefaultAboutMarkdown,
		LanguageYAML:      judge.DefaultLanguagesYAML(),
		PaginationProblem: 100, PaginationContest: 20, PaginationDiscussion: 50,
		PaginationRecord: 100, PaginationRanking: 100, PaginationSolution: 20,
		PaginationTraining: 10,
		LimitUserFiles:     100, LimitUserFileSize: 128 << 20,
		LimitContestFiles: 100, LimitContestFileSize: 128 << 20,
		LimitProblemFiles: 100, LimitProblemFileSize: 256 << 20,
		LimitCodeLength: 128 << 10,
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
	case settingLanguages:
		if isText {
			values.LanguageYAML = text
		} else if value != nil {
			if encoded, err := yaml.Marshal(value); err == nil {
				values.LanguageYAML = string(encoded)
			}
		}
	case settingPaginationProblem:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationProblem = number
		}
	case settingPaginationContest:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationContest = number
		}
	case settingPaginationDiscussion:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationDiscussion = number
		}
	case settingPaginationRecord:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationRecord = number
		}
	case settingPaginationRanking:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationRanking = number
		}
	case settingPaginationSolution:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationSolution = number
		}
	case settingPaginationTraining:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationTraining = number
		}
	case settingLimitUserFiles:
		if number, ok := anyInt(value); ok && number > 0 {
			values.LimitUserFiles = number
		}
	case settingLimitUserFileSize:
		if number, ok := anyInt64(value); ok && number > 0 {
			values.LimitUserFileSize = number
		}
	case settingLimitContestFiles:
		if number, ok := anyInt(value); ok && number > 0 {
			values.LimitContestFiles = number
		}
	case settingLimitContestFileSize:
		if number, ok := anyInt64(value); ok && number > 0 {
			values.LimitContestFileSize = number
		}
	case settingLimitProblemFiles:
		if number, ok := anyInt(value); ok && number > 0 {
			values.LimitProblemFiles = number
		}
	case settingLimitProblemFileSize:
		if number, ok := anyInt64(value); ok && number > 0 {
			values.LimitProblemFileSize = number
		}
	case settingLimitCodeLength:
		if number, ok := anyInt64(value); ok && number > 0 {
			values.LimitCodeLength = number
		}
	}
}

func anyInt(value any) (int, bool) {
	if number, ok := anyInt64(value); ok && number >= math.MinInt && number <= math.MaxInt {
		return int(number), true
	}
	return 0, false
}

func anyInt64(value any) (int64, bool) {
	switch number := value.(type) {
	case int:
		return int64(number), true
	case int32:
		return int64(number), true
	case int64:
		return number, true
	case float64:
		if number != math.Trunc(number) || number < math.MinInt64 || number > math.MaxInt64 {
			return 0, false
		}
		return int64(number), true
	default:
		return 0, false
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
	if input.LanguageYAML != nil {
		updates[settingLanguages] = *input.LanguageYAML
	}
	for key, value := range map[string]*int{
		settingPaginationProblem: input.PaginationProblem, settingPaginationContest: input.PaginationContest,
		settingPaginationDiscussion: input.PaginationDiscussion, settingPaginationRecord: input.PaginationRecord,
		settingPaginationRanking: input.PaginationRanking, settingPaginationSolution: input.PaginationSolution,
		settingPaginationTraining: input.PaginationTraining, settingLimitUserFiles: input.LimitUserFiles,
		settingLimitContestFiles: input.LimitContestFiles, settingLimitProblemFiles: input.LimitProblemFiles,
	} {
		if value != nil {
			updates[key] = *value
		}
	}
	for key, value := range map[string]*int64{
		settingLimitUserFileSize: input.LimitUserFileSize, settingLimitContestFileSize: input.LimitContestFileSize,
		settingLimitProblemFileSize: input.LimitProblemFileSize, settingLimitCodeLength: input.LimitCodeLength,
	} {
		if value != nil {
			updates[key] = *value
		}
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
	Bulletin             *string `json:"bulletin"`
	SiteName             *string `json:"siteName"`
	NavLogo              *string `json:"navLogo"`
	FooterExtraHTML      *string `json:"footerExtraHtml"`
	EnableScratchpad     *bool   `json:"enableScratchpad"`
	GravatarURL          *string `json:"gravatarUrl"`
	HomepageYAML         *string `json:"homepageYaml"`
	AboutMarkdown        *string `json:"aboutMarkdown"`
	LanguageYAML         *string `json:"languageYaml"`
	PaginationProblem    *int    `json:"paginationProblem"`
	PaginationContest    *int    `json:"paginationContest"`
	PaginationDiscussion *int    `json:"paginationDiscussion"`
	PaginationRecord     *int    `json:"paginationRecord"`
	PaginationRanking    *int    `json:"paginationRanking"`
	PaginationSolution   *int    `json:"paginationSolution"`
	PaginationTraining   *int    `json:"paginationTraining"`
	LimitUserFiles       *int    `json:"limitUserFiles"`
	LimitUserFileSize    *int64  `json:"limitUserFileSize"`
	LimitContestFiles    *int    `json:"limitContestFiles"`
	LimitContestFileSize *int64  `json:"limitContestFileSize"`
	LimitProblemFiles    *int    `json:"limitProblemFiles"`
	LimitProblemFileSize *int64  `json:"limitProblemFileSize"`
	LimitCodeLength      *int64  `json:"limitCodeLength"`
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
	if input.LanguageYAML != nil {
		if _, err := judge.ParseLanguages(*input.LanguageYAML); err != nil {
			return fmt.Errorf("invalid language yaml: %w", err)
		}
	}
	for _, item := range []struct {
		value *int
		max   int
	}{
		{input.PaginationProblem, 1000}, {input.PaginationContest, 1000},
		{input.PaginationDiscussion, 1000}, {input.PaginationRecord, 1000},
		{input.PaginationRanking, 1000}, {input.PaginationSolution, 1000},
		{input.PaginationTraining, 1000}, {input.LimitUserFiles, 10000},
		{input.LimitContestFiles, 10000}, {input.LimitProblemFiles, 10000},
	} {
		if item.value != nil && (*item.value < 1 || *item.value > item.max) {
			return fmt.Errorf("setting value must be between 1 and %d", item.max)
		}
	}
	for _, item := range []struct {
		value *int64
		label string
	}{
		{input.LimitUserFileSize, "limit.user_files_size"}, {input.LimitContestFileSize, "limit.contest_files_size"},
		{input.LimitProblemFileSize, "limit.problem_files_max_size"}, {input.LimitCodeLength, "limit.codelength"},
	} {
		if item.value != nil && (*item.value < 1 || *item.value > 10<<30) {
			return fmt.Errorf("%s must be between 1 and %d", item.label, int64(10<<30))
		}
	}
	for _, item := range []struct {
		value *string
		max   int
		label string
	}{
		{input.Bulletin, 20000, "bulletin"}, {input.SiteName, 100, "site name"},
		{input.NavLogo, 512, "nav logo"}, {input.FooterExtraHTML, 20000, "footer html"},
		{input.GravatarURL, 512, "gravatar url"}, {input.HomepageYAML, 20000, "homepage yaml"},
		{input.AboutMarkdown, 50000, "about markdown"}, {input.LanguageYAML, 100000, "language yaml"},
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
