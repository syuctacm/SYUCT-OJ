package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port            string
	MongoURI        string
	MongoDatabase   string
	CORSOrigins     []string
	JudgeUsername   string
	JudgePassword   string
	JudgeFileRoot   string
	JudgeTaskLease  time.Duration
	SessionTTL      time.Duration
	LoginRateLimit  int
	LoginRateWindow time.Duration
	CookieSecure    bool
}

func Load() Config {
	port := env("SERVER_PORT", "8080")
	mongoURI := env("MONGO_URI", "mongodb://syuct:syuct_dev_password@127.0.0.1:27017/syuct_oj?authSource=admin")
	mongoDatabase := env("MONGO_DATABASE", "syuct_oj")
	origins := strings.Split(env("CORS_ORIGINS", "http://localhost:5173"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return Config{
		Port:            port,
		MongoURI:        mongoURI,
		MongoDatabase:   mongoDatabase,
		CORSOrigins:     origins,
		JudgeUsername:   env("JUDGE_USERNAME", "judge"),
		JudgePassword:   env("JUDGE_PASSWORD", "judge_dev_password"),
		JudgeFileRoot:   env("JUDGE_FILE_ROOT", "./data/judge-files"),
		JudgeTaskLease:  durationEnv("JUDGE_TASK_LEASE", 2*time.Minute),
		SessionTTL:      durationEnv("SESSION_TTL", 24*time.Hour),
		LoginRateLimit:  intEnv("LOGIN_RATE_LIMIT", 10),
		LoginRateWindow: durationEnv("LOGIN_RATE_WINDOW", time.Minute),
		CookieSecure:    boolEnv("COOKIE_SECURE", false),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return fallback
	}
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
