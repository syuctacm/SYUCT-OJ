package config

import (
	"os"
	"strings"
)

type Config struct {
	Port          string
	MongoURI      string
	MongoDatabase string
	CORSOrigins   []string
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
		Port:          port,
		MongoURI:      mongoURI,
		MongoDatabase: mongoDatabase,
		CORSOrigins:   origins,
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
