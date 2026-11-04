package handler

import (
	"context"
	"fmt"
	"math"
	"net/http"
	stdmail "net/mail"
	"net/url"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/syuctacm/syuct-oj/internal/judge"
	mailservice "github.com/syuctacm/syuct-oj/internal/mail"
)

const (
	settingBulletin              = "bulletin"
	settingServerName            = "server.name"
	settingServerURL             = "server.url"
	settingNavLogo               = "ui-default.nav_logo_dark"
	settingFooterExtraHTML       = "ui-default.footer_extra_html"
	settingScratchpad            = "ui-default.enableScratchpad"
	settingGravatarURL           = "avatar.gravatar_url"
	settingHomepage              = "hydrooj.homepage"
	settingAbout                 = "ui-default.about"
	settingLanguages             = "hydrooj.langs"
	settingProblemCategories     = "problem.categories"
	settingTrainingEnrolledUsers = "training.enrolled-users"

	settingSMTPHost     = "smtp.host"
	settingSMTPPort     = "smtp.port"
	settingSMTPUsername = "smtp.user"
	settingSMTPPassword = "smtp.pass"
	settingSMTPFrom     = "smtp.from"
	settingSMTPSecure   = "smtp.secure"

	settingLimitUserFiles       = "limit.user_files"
	settingLimitUserFileSize    = "limit.user_files_size"
	settingLimitContestFiles    = "limit.contest_files"
	settingLimitContestFileSize = "limit.contest_files_size"
	settingLimitProblemFiles    = "limit.problem_files_max"
	settingLimitProblemFileSize = "limit.problem_files_max_size"
	settingLimitCodeLength      = "limit.codelength"
	settingLimitSubmission      = "limit.submission"
	settingLimitSubmissionUser  = "limit.submission_user"
	settingLimitPretest         = "limit.pretest"
	settingLimitByUser          = "limit.by_user"
	settingSessionSavedExpire   = "session.saved_expire_seconds"
	settingSessionUnsavedExpire = "session.unsaved_expire_seconds"

	settingPaginationProblem    = "pagination.problem"
	settingPaginationContest    = "pagination.contest"
	settingPaginationDiscussion = "pagination.discussion"
	settingPaginationReply      = "pagination.reply"
	settingPaginationRecord     = "pagination.record"
	settingPaginationRanking    = "pagination.ranking"
	settingPaginationSolution   = "pagination.solution"
	settingPaginationTraining   = "pagination.training"
)

const DefaultSiteName = "SYUCT ACM OJ"
const DefaultNavLogo = "/nav_logo_dark.png"
const DefaultFooterExtraHTML = "沈阳化工大学 ACM-ICPC 集训队"
const DefaultGravatarURL = "//cn.gravatar.com/avatar/"

// DefaultProblemCategoriesYAML matches Hydro 4.58.5 builtin.CATEGORIES.
const DefaultProblemCategoriesYAML = `动态规划:
  - LCS
  - LIS
  - 背包
  - 单调性DP
  - 环形DP
  - 树形DP
  - 状态压缩DP
搜索:
  - 枚举
  - 搜索与剪枝
  - 启发式搜索
  - DLX
  - 双向搜索
  - 折半搜索
  - 记忆化搜索
  - 模拟退火
计算几何:
  - 半平面交
  - 凸包
  - 几何图形的交与并
  - 旋转卡壳
  - 点定位
  - 坐标变换
  - 离散化与扫描
  - 反演
  - Voronoi图
  - 平面图的对偶图
  - 三角剖分
  - 梯形剖分
  - 几何知识
贪心: []
树结构:
  - 最近公共祖先
  - 生成树
  - 次小生成树
  - DFS序列
  - 树上倍增
  - 树的分治
  - 树链剖分
  - Link-Cut-Tree
图结构:
  - Floyd
  - Dijkstra
  - SPFA
  - 负权边
  - 平面图
  - 二分图
  - 二分图匹配
  - 最短路
  - 差分约束
  - 拓扑排序
  - 网络流
  - 强连通分量
  - 割点割边
  - 欧拉回路
  - 2-SAT
数论:
  - 素数判定
  - 欧几里得算法
  - 不定方程
  - 数位统计
  - 解线性同余方程
  - baby-step-giant-step
  - Pell方程
  - 大整数质因数分解
  - 勾股方程
  - 积性函数
  - Fibonacci数列
模拟: []
数据结构:
  - 栈
  - 队列
  - 链表
  - 单调队列
  - 并查集
  - 堆
  - 平衡树
  - 线段树
  - 树状数组
  - 树套树
  - 四分树
  - 划分树
  - 归并树
  - 替罪羊树
  - 左偏树
  - k-d树
  - 块状链表
  - Hashing
  - 函数式编程
博弈论:
  - 巴什博弈
  - 尼姆博弈
  - 威佐夫博弈
  - 斐波那契博弈
  - SG定理
字符串:
  - KMP
  - 后缀数据结构
  - Trie树
  - AC自动机
  - Manacher
  - 表达式处理
  - 最小表示法
组合数学:
  - 鸽笼
  - 生成函数
  - 容斥原理
  - 康托展开
  - Catalan数列
  - Stirling数
  - 差分
  - Polya定理
线性代数:
  - 矩阵乘法
  - 高斯消元
  - 线性规划
高精度:
  - FFT
递推: []
概率论:
  - 随机化
NPC: []
其他:
  - 二分查找
  - 三分查找
  - 双指针扫描
  - 分治
  - 分块
  - RMQ
  - 快速幂
  - 数学
  - 排序
  - 构造
  - 打表
  - 位运算
  - 离散化
  - 莫队
`
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
	settingServerURL,
	settingScratchpad, settingGravatarURL, settingHomepage, settingAbout, settingLanguages,
	settingProblemCategories,
	settingTrainingEnrolledUsers,
	settingPaginationProblem, settingPaginationContest, settingPaginationDiscussion,
	settingPaginationReply, settingPaginationRecord, settingPaginationRanking, settingPaginationSolution,
	settingPaginationTraining,
	settingLimitUserFiles, settingLimitUserFileSize, settingLimitContestFiles,
	settingLimitContestFileSize, settingLimitProblemFiles, settingLimitProblemFileSize,
	settingLimitCodeLength, settingLimitSubmission, settingLimitSubmissionUser,
	settingLimitPretest, settingLimitByUser,
	settingSessionSavedExpire, settingSessionUnsavedExpire,
}

var smtpSettingKeys = []string{
	settingSMTPHost, settingSMTPPort, settingSMTPUsername,
	settingSMTPPassword, settingSMTPFrom, settingSMTPSecure,
}

type SiteSettings struct {
	Bulletin              string `json:"bulletin"`
	SiteName              string `json:"siteName"`
	ServerURL             string `json:"serverUrl"`
	NavLogo               string `json:"navLogo"`
	FooterExtraHTML       string `json:"footerExtraHtml"`
	EnableScratchpad      bool   `json:"enableScratchpad"`
	GravatarURL           string `json:"gravatarUrl"`
	HomepageYAML          string `json:"homepageYaml"`
	AboutMarkdown         string `json:"aboutMarkdown"`
	LanguageYAML          string `json:"languageYaml"`
	ProblemCategoriesYAML string `json:"problemCategoriesYaml"`
	TrainingEnrolledUsers bool   `json:"trainingEnrolledUsers"`
	PaginationProblem     int    `json:"paginationProblem"`
	PaginationContest     int    `json:"paginationContest"`
	PaginationDiscussion  int    `json:"paginationDiscussion"`
	PaginationReply       int    `json:"paginationReply"`
	PaginationRecord      int    `json:"paginationRecord"`
	PaginationRanking     int    `json:"paginationRanking"`
	PaginationSolution    int    `json:"paginationSolution"`
	PaginationTraining    int    `json:"paginationTraining"`
	LimitUserFiles        int    `json:"limitUserFiles"`
	LimitUserFileSize     int64  `json:"limitUserFileSize"`
	LimitContestFiles     int    `json:"limitContestFiles"`
	LimitContestFileSize  int64  `json:"limitContestFileSize"`
	LimitProblemFiles     int    `json:"limitProblemFiles"`
	LimitProblemFileSize  int64  `json:"limitProblemFileSize"`
	LimitCodeLength       int64  `json:"limitCodeLength"`
	LimitSubmission       int    `json:"limitSubmission"`
	LimitSubmissionUser   int    `json:"limitSubmissionUser"`
	LimitPretest          int    `json:"limitPretest"`
	LimitByUser           bool   `json:"limitByUser"`
	SessionSavedExpire    int64  `json:"sessionSavedExpireSeconds"`
	SessionUnsavedExpire  int64  `json:"sessionUnsavedExpireSeconds"`
	SMTPConfigured        bool   `json:"smtpConfigured"`
}

type AdminSiteSettings struct {
	SiteSettings
	SMTPHost               string `json:"smtpHost"`
	SMTPPort               int    `json:"smtpPort"`
	SMTPUsername           string `json:"smtpUsername"`
	SMTPPasswordConfigured bool   `json:"smtpPasswordConfigured"`
	SMTPFrom               string `json:"smtpFrom"`
	SMTPSecure             bool   `json:"smtpSecure"`
}

func DefaultSiteSettings() SiteSettings {
	return SiteSettings{
		SiteName: DefaultSiteName, NavLogo: DefaultNavLogo,
		ServerURL:       "/",
		FooterExtraHTML: DefaultFooterExtraHTML, EnableScratchpad: true,
		GravatarURL: DefaultGravatarURL, HomepageYAML: DefaultHomepageYAML,
		AboutMarkdown:         DefaultAboutMarkdown,
		LanguageYAML:          judge.DefaultLanguagesYAML(),
		ProblemCategoriesYAML: DefaultProblemCategoriesYAML,
		TrainingEnrolledUsers: true,
		PaginationProblem:     100, PaginationContest: 20, PaginationDiscussion: 50, PaginationReply: 50,
		PaginationRecord: 100, PaginationRanking: 100, PaginationSolution: 20,
		PaginationTraining: 10,
		LimitUserFiles:     100, LimitUserFileSize: 128 << 20,
		LimitContestFiles: 100, LimitContestFileSize: 128 << 20,
		LimitProblemFiles: 100, LimitProblemFileSize: 256 << 20,
		LimitCodeLength: 128 << 10,
		LimitSubmission: 60, LimitSubmissionUser: 15, LimitPretest: 60,
		SessionSavedExpire: 30 * 24 * 60 * 60, SessionUnsavedExpire: 3 * 60 * 60,
	}
}

// SettingsHandler serves the public subset of the `system` key/value store,
// which is the same shape Hydro's model/system.ts uses.
type SettingsHandler struct {
	db           *mongo.Database
	mailFallback mailservice.Config
}

func NewSettingsHandler(db *mongo.Database) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) SetMailFallback(config mailservice.Config) {
	h.mailFallback = config
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
	if config, err := ReadMailConfig(c.Request.Context(), h.db, h.mailFallback); err == nil {
		values.SMTPConfigured = mailservice.IsConfigured(config)
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

func ReadMailConfig(ctx context.Context, db *mongo.Database, fallback mailservice.Config) (mailservice.Config, error) {
	values, found, err := readSystemValues(ctx, db, smtpSettingKeys)
	if err != nil {
		return fallback, err
	}
	if !found {
		if fallback.Port <= 0 {
			fallback.Port = 587
		}
		return fallback, nil
	}
	config := fallback
	if config.Port <= 0 {
		config.Port = 587
	}
	if value, ok := values[settingSMTPHost].(string); ok {
		config.Host = strings.TrimSpace(value)
	}
	if value, ok := anyInt(values[settingSMTPPort]); ok {
		config.Port = value
	}
	if value, ok := values[settingSMTPUsername].(string); ok {
		config.Username = strings.TrimSpace(value)
	}
	if value, ok := values[settingSMTPPassword].(string); ok {
		config.Password = value
	}
	if value, ok := values[settingSMTPFrom].(string); ok {
		config.From = strings.TrimSpace(value)
	}
	if value, ok := anyBool(values[settingSMTPSecure]); ok {
		config.Secure = value
	}
	return config, nil
}

func readAdminSiteSettings(ctx context.Context, db *mongo.Database, fallback mailservice.Config) (AdminSiteSettings, error) {
	site, err := readSiteSettings(ctx, db)
	if err != nil {
		return AdminSiteSettings{}, err
	}
	config, err := ReadMailConfig(ctx, db, fallback)
	if err != nil {
		return AdminSiteSettings{}, err
	}
	site.SMTPConfigured = mailservice.IsConfigured(config)
	return AdminSiteSettings{
		SiteSettings:           site,
		SMTPHost:               config.Host,
		SMTPPort:               config.Port,
		SMTPUsername:           config.Username,
		SMTPPasswordConfigured: config.Password != "",
		SMTPFrom:               config.From,
		SMTPSecure:             config.Secure,
	}, nil
}

func readSystemValues(ctx context.Context, db *mongo.Database, keys []string) (map[string]any, bool, error) {
	cursor, err := db.Collection("system").Find(ctx, bson.M{"_id": bson.M{"$in": keys}},
		options.Find().SetProjection(bson.M{"value": 1}))
	if err != nil {
		return nil, false, err
	}
	defer cursor.Close(ctx)
	values := make(map[string]any, len(keys))
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Value any    `bson:"value"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, false, err
		}
		values[doc.ID] = doc.Value
	}
	if err := cursor.Err(); err != nil {
		return nil, false, err
	}
	return values, len(values) > 0, nil
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
	case settingServerURL:
		if normalized, ok := normalizeServerURL(text); ok {
			values.ServerURL = normalized
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
	case settingProblemCategories:
		if isText {
			values.ProblemCategoriesYAML = text
		} else if value != nil {
			if encoded, err := yaml.Marshal(value); err == nil {
				values.ProblemCategoriesYAML = string(encoded)
			}
		}
	case settingTrainingEnrolledUsers:
		if boolean, ok := anyBool(value); ok {
			values.TrainingEnrolledUsers = boolean
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
	case settingPaginationReply:
		if number, ok := anyInt(value); ok && number > 0 {
			values.PaginationReply = number
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
	case settingLimitSubmission:
		if number, ok := anyInt(value); ok && number > 0 {
			values.LimitSubmission = number
		}
	case settingLimitSubmissionUser:
		if number, ok := anyInt(value); ok && number > 0 {
			values.LimitSubmissionUser = number
		}
	case settingLimitPretest:
		if number, ok := anyInt(value); ok && number > 0 {
			values.LimitPretest = number
		}
	case settingLimitByUser:
		if boolean, ok := anyBool(value); ok {
			values.LimitByUser = boolean
		}
	case settingSessionSavedExpire:
		if number, ok := anyInt64(value); ok && number > 0 {
			values.SessionSavedExpire = number
		}
	case settingSessionUnsavedExpire:
		if number, ok := anyInt64(value); ok && number > 0 {
			values.SessionUnsavedExpire = number
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

func hasSMTPChanges(input SiteSettingsInput) bool {
	return input.SMTPHost != nil || input.SMTPPort != nil || input.SMTPUsername != nil ||
		input.SMTPPassword != nil || input.SMTPFrom != nil || input.SMTPSecure != nil ||
		input.SMTPClearPassword != nil
}

func mergeSMTPConfig(current mailservice.Config, input SiteSettingsInput) mailservice.Config {
	if input.SMTPHost != nil {
		current.Host = strings.TrimSpace(*input.SMTPHost)
	}
	if input.SMTPPort != nil {
		current.Port = *input.SMTPPort
	}
	if input.SMTPUsername != nil {
		current.Username = strings.TrimSpace(*input.SMTPUsername)
	}
	if input.SMTPFrom != nil {
		current.From = strings.TrimSpace(*input.SMTPFrom)
	}
	if input.SMTPSecure != nil {
		current.Secure = *input.SMTPSecure
	}
	if input.SMTPClearPassword != nil && *input.SMTPClearPassword {
		current.Password = ""
	} else if input.SMTPPassword != nil && *input.SMTPPassword != "" {
		current.Password = *input.SMTPPassword
	}
	if parsed, err := stdmail.ParseAddress(current.From); err == nil {
		current.From = parsed.Address
	}
	if current.Port <= 0 && input.SMTPPort == nil {
		current.Port = 587
	}
	return current
}

func validateSMTPConfig(config mailservice.Config) error {
	host := strings.TrimSpace(config.Host)
	from := strings.TrimSpace(config.From)
	if host == "" && from == "" {
		return nil
	}
	if host == "" || from == "" {
		return fmt.Errorf("SMTP host and From address must both be set or both be empty")
	}
	if strings.ContainsAny(host, " \t\r\n") {
		return fmt.Errorf("invalid SMTP host")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	if strings.ContainsAny(config.Username, "\r\n") {
		return fmt.Errorf("invalid SMTP username")
	}
	if _, err := stdmail.ParseAddress(from); err != nil {
		return fmt.Errorf("invalid SMTP From address")
	}
	return nil
}

func smtpSettingUpdates(config mailservice.Config) map[string]any {
	return map[string]any{
		settingSMTPHost:     strings.TrimSpace(config.Host),
		settingSMTPPort:     config.Port,
		settingSMTPUsername: strings.TrimSpace(config.Username),
		settingSMTPPassword: config.Password,
		settingSMTPFrom:     strings.TrimSpace(config.From),
		settingSMTPSecure:   config.Secure,
	}
}

func siteSettingUpdates(input SiteSettingsInput) map[string]any {
	updates := make(map[string]any)
	if input.Bulletin != nil {
		updates[settingBulletin] = *input.Bulletin
	}
	if input.SiteName != nil {
		updates[settingServerName] = *input.SiteName
	}
	if input.ServerURL != nil {
		if normalized, ok := normalizeServerURL(*input.ServerURL); ok {
			updates[settingServerURL] = normalized
		}
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
	if input.ProblemCategoriesYAML != nil {
		updates[settingProblemCategories] = *input.ProblemCategoriesYAML
	}
	if input.TrainingEnrolledUsers != nil {
		updates[settingTrainingEnrolledUsers] = *input.TrainingEnrolledUsers
	}
	if input.LimitByUser != nil {
		updates[settingLimitByUser] = *input.LimitByUser
	}
	if input.SessionSavedExpire != nil {
		updates[settingSessionSavedExpire] = *input.SessionSavedExpire
	}
	if input.SessionUnsavedExpire != nil {
		updates[settingSessionUnsavedExpire] = *input.SessionUnsavedExpire
	}
	for key, value := range map[string]*int{
		settingPaginationProblem: input.PaginationProblem, settingPaginationContest: input.PaginationContest,
		settingPaginationDiscussion: input.PaginationDiscussion, settingPaginationReply: input.PaginationReply,
		settingPaginationRecord:  input.PaginationRecord,
		settingPaginationRanking: input.PaginationRanking, settingPaginationSolution: input.PaginationSolution,
		settingPaginationTraining: input.PaginationTraining, settingLimitUserFiles: input.LimitUserFiles,
		settingLimitContestFiles: input.LimitContestFiles, settingLimitProblemFiles: input.LimitProblemFiles,
		settingLimitSubmission: input.LimitSubmission, settingLimitSubmissionUser: input.LimitSubmissionUser,
		settingLimitPretest: input.LimitPretest,
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
	Bulletin              *string `json:"bulletin"`
	SiteName              *string `json:"siteName"`
	ServerURL             *string `json:"serverUrl"`
	NavLogo               *string `json:"navLogo"`
	FooterExtraHTML       *string `json:"footerExtraHtml"`
	EnableScratchpad      *bool   `json:"enableScratchpad"`
	GravatarURL           *string `json:"gravatarUrl"`
	HomepageYAML          *string `json:"homepageYaml"`
	AboutMarkdown         *string `json:"aboutMarkdown"`
	LanguageYAML          *string `json:"languageYaml"`
	ProblemCategoriesYAML *string `json:"problemCategoriesYaml"`
	TrainingEnrolledUsers *bool   `json:"trainingEnrolledUsers"`
	PaginationProblem     *int    `json:"paginationProblem"`
	PaginationContest     *int    `json:"paginationContest"`
	PaginationDiscussion  *int    `json:"paginationDiscussion"`
	PaginationReply       *int    `json:"paginationReply"`
	PaginationRecord      *int    `json:"paginationRecord"`
	PaginationRanking     *int    `json:"paginationRanking"`
	PaginationSolution    *int    `json:"paginationSolution"`
	PaginationTraining    *int    `json:"paginationTraining"`
	LimitUserFiles        *int    `json:"limitUserFiles"`
	LimitUserFileSize     *int64  `json:"limitUserFileSize"`
	LimitContestFiles     *int    `json:"limitContestFiles"`
	LimitContestFileSize  *int64  `json:"limitContestFileSize"`
	LimitProblemFiles     *int    `json:"limitProblemFiles"`
	LimitProblemFileSize  *int64  `json:"limitProblemFileSize"`
	LimitCodeLength       *int64  `json:"limitCodeLength"`
	LimitSubmission       *int    `json:"limitSubmission"`
	LimitSubmissionUser   *int    `json:"limitSubmissionUser"`
	LimitPretest          *int    `json:"limitPretest"`
	LimitByUser           *bool   `json:"limitByUser"`
	SessionSavedExpire    *int64  `json:"sessionSavedExpireSeconds"`
	SessionUnsavedExpire  *int64  `json:"sessionUnsavedExpireSeconds"`
	SMTPHost              *string `json:"smtpHost"`
	SMTPPort              *int    `json:"smtpPort"`
	SMTPUsername          *string `json:"smtpUsername"`
	SMTPPassword          *string `json:"smtpPassword"`
	SMTPFrom              *string `json:"smtpFrom"`
	SMTPSecure            *bool   `json:"smtpSecure"`
	SMTPClearPassword     *bool   `json:"smtpClearPassword"`
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
	if input.ServerURL != nil {
		if _, ok := normalizeServerURL(*input.ServerURL); !ok {
			return fmt.Errorf("invalid server URL")
		}
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
	if input.ProblemCategoriesYAML != nil && !validProblemCategoriesYAML(*input.ProblemCategoriesYAML) {
		return fmt.Errorf("invalid problem categories yaml")
	}
	for _, item := range []struct {
		value *int
		max   int
	}{
		{input.PaginationProblem, 1000}, {input.PaginationContest, 1000},
		{input.PaginationDiscussion, 1000}, {input.PaginationReply, 1000}, {input.PaginationRecord, 1000},
		{input.PaginationRanking, 1000}, {input.PaginationSolution, 1000},
		{input.PaginationTraining, 1000}, {input.LimitUserFiles, 10000},
		{input.LimitContestFiles, 10000}, {input.LimitProblemFiles, 10000},
		{input.LimitSubmission, 10000}, {input.LimitSubmissionUser, 10000}, {input.LimitPretest, 10000},
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
		value *int64
		label string
	}{
		{input.SessionSavedExpire, "session.saved_expire_seconds"}, {input.SessionUnsavedExpire, "session.unsaved_expire_seconds"},
	} {
		if item.value != nil && (*item.value < 60 || *item.value > 365*24*60*60) {
			return fmt.Errorf("%s must be between 60 and 31536000 seconds", item.label)
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
		{input.ProblemCategoriesYAML, 100000, "problem categories yaml"},
	} {
		if err := checkLength(item.value, item.max, item.label); err != nil {
			return err
		}
	}
	return nil
}

func validProblemCategoriesYAML(value string) bool {
	var categories map[string][]string
	if err := yaml.Unmarshal([]byte(value), &categories); err != nil || categories == nil {
		return false
	}
	for category, subcategories := range categories {
		if strings.TrimSpace(category) == "" {
			return false
		}
		for _, subcategory := range subcategories {
			if strings.TrimSpace(subcategory) == "" {
				return false
			}
		}
	}
	return true
}

func normalizeServerURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/", true
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	if strings.Contains(parsed.Path, "..") {
		return "", false
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
			return "", false
		}
		return strings.TrimRight(parsed.String(), "/"), true
	}
	if parsed.Host != "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "", false
	}
	return strings.TrimRight(value, "/"), true
}

func configuredServerBaseURL(c *gin.Context) string {
	rawDB, ok := c.Get(RequestDatabaseKey)
	if !ok {
		return ""
	}
	db, ok := rawDB.(*mongo.Database)
	if !ok || db == nil {
		return ""
	}
	var doc struct {
		Value any `bson:"value"`
	}
	if err := db.Collection("system").FindOne(c.Request.Context(), bson.M{"_id": settingServerURL}).Decode(&doc); err != nil {
		return ""
	}
	value, ok := doc.Value.(string)
	if !ok {
		return ""
	}
	normalized, ok := normalizeServerURL(value)
	if !ok || normalized == "/" {
		return ""
	}
	if strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://") {
		return normalized
	}
	return requestOriginURL(c) + normalized
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
