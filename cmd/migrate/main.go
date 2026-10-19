package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/migration"
)

func main() {
	_ = godotenv.Load()
	var (
		sources    = flag.String("sources", "contest2025,select2025", "comma-separated source domains")
		target     = flag.String("target", migration.DefaultTargetDomain, "target domain")
		dryRun     = flag.Bool("dry-run", true, "report only; do not mutate data")
		execute    = flag.Bool("execute", false, "execute migration; overrides --dry-run")
		reportPath = flag.String("report", "", "optional report JSON path")
		mongoURI   = flag.String("mongo-uri", os.Getenv("MONGO_URI"), "MongoDB URI")
	)
	flag.Parse()

	cfg := config.Load()
	if *mongoURI == "" {
		*mongoURI = cfg.MongoURI
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	client, db, err := database.Connect(ctx, *mongoURI, cfg.MongoDatabase, 10*time.Second)
	if err != nil {
		fail("connect MongoDB: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	report, err := migration.Run(ctx, db, migration.Options{
		Sources: strings.Split(*sources, ","),
		Target:  *target,
		DryRun:  *dryRun && !*execute,
	})
	if err != nil {
		fail("migration: %v", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("encode report: %v", err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, append(data, '\n'), 0o640); err != nil {
			fail("write report: %v", err)
		}
	}
	_, _ = os.Stdout.Write(append(data, '\n'))
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
