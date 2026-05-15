// Command backfill-embeddings walks the questions table in batches and writes
// a Vertex AI embedding for any row whose `embedding` column is NULL. Safe to
// re-run: rows that already have a vector are skipped. Typical use:
//
//	docker-compose run --rm backend go run ./cmd/backfill-embeddings
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/config"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/database"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/embeddings"
)

func main() {
	batchSize := flag.Int("batch", 50, "rows to embed per round trip (max 200)")
	flag.Parse()

	cfg := config.Load()
	if cfg.GCPProject == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT is required for the backfill command")
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	emb, err := embeddings.New(ctx, cfg.GCPProject, cfg.GCPLocation, embeddings.DefaultModel)
	if err != nil {
		log.Fatalf("embeddings init: %v", err)
	}

	repo := &repository.QuestionRepo{DB: db}
	start := time.Now()
	total := 0
	for {
		batch, err := repo.ListNeedingEmbedding(ctx, *batchSize)
		if err != nil {
			log.Fatalf("list rows: %v", err)
		}
		if len(batch) == 0 {
			break
		}

		texts := make([]string, len(batch))
		for i, row := range batch {
			texts[i] = row.Text
		}
		vecs, err := emb.Embed(ctx, texts, embeddings.TaskRetrievalDocument)
		if err != nil {
			log.Fatalf("embed batch: %v", err)
		}

		for i, row := range batch {
			if err := repo.UpdateEmbedding(ctx, row.ID, vecs[i]); err != nil {
				log.Fatalf("update %s: %v", row.ID, err)
			}
		}
		total += len(batch)
		log.Printf("embedded %d / %d so far", len(batch), total)
	}
	log.Printf("done: embedded %d question(s) in %s", total, time.Since(start).Truncate(time.Millisecond))
}
