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
		sources    = flag.String("sources", "contest2025,select2025", "comma-separated source domains used for reporting")
		dryRun     = flag.Bool("dry-run", true, "report only; do not mutate data")
		execute    = flag.Bool("execute", false, "execute migration; overrides --dry-run")
		reportPath = flag.String("report", "", "optional report JSON path")
		mongoURI   = flag.String("mongo-uri", os.Getenv("MONGO_URI"), "MongoDB URI")
	)
	flag.Parse()
	if !*execute && !*dryRun {
		fail("--dry-run=false is not sufficient to execute; pass --execute explicitly")
	}

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

	report, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{
		Sources: strings.Split(*sources, ","),
		DryRun:  *dryRun && !*execute,
	})
	if report != nil {
		data, encodeErr := json.MarshalIndent(report, "", "  ")
		if encodeErr != nil {
			fail("encode report: %v", encodeErr)
		}
		if *reportPath != "" {
			if writeErr := os.WriteFile(*reportPath, append(data, '\n'), 0o640); writeErr != nil {
				fail("write report: %v", writeErr)
			}
		}
		_, _ = os.Stdout.Write(append(data, '\n'))
	}
	if err != nil {
		fail("WebAuthn migration: %v", err)
	}
	if report == nil || !report.OK {
		fail("WebAuthn migration report is not OK")
	}
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
