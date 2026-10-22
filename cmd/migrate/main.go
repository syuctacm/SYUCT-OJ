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
		sources       = flag.String("sources", "contest2025,select2025", "comma-separated source domains")
		target        = flag.String("target", migration.DefaultTargetDomain, "target domain")
		dryRun        = flag.Bool("dry-run", true, "report only; do not mutate data")
		execute       = flag.Bool("execute", false, "execute migration; overrides --dry-run")
		reportPath    = flag.String("report", "", "optional report JSON path")
		renumberDocID = flag.Bool("renumber-doc-ids", false, "renumber colliding problem docIds and rewrite every reference")
		fileRoot      = flag.String("file-root", "", "optional local problem file root; moves problem/<domain>/<docId> directories")
		mongoURI      = flag.String("mongo-uri", os.Getenv("MONGO_URI"), "MongoDB URI")
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
		Sources:        strings.Split(*sources, ","),
		Target:         *target,
		DryRun:         *dryRun && !*execute,
		RenumberDocIDs: *renumberDocID,
		FileRoot:       *fileRoot,
	})
	// Write the report even when Run aborts (for example on problem id
	// collisions) so the operator can inspect report.conflicts.
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
		fail("migration: %v", err)
	}
	if report.Validation != nil && !report.Validation.OK {
		fail("migration finished with residual source-domain data; see validation.residuals")
	}
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
