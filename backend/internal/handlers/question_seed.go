package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/embeddings"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
)

// seedAIQuestionsForCategories generates `count` AI questions tagged to the
// given category slugs and persists them as public catalog rows
// (source='ai-generated', owner_id NULL, is_public=true). Dedup runs
// title-first against the last 100 existing titles in those categories, plus
// in-batch so the model can't emit two paraphrases of the same prompt.
//
// Used by both:
//   - QuestionHandler.Generate (synchronous; results returned to the caller)
//   - CategoryHandler.Create (fire-and-forget; called from a goroutine after
//     an admin creates a new topic so it ships with starter content)
//
// Per-item failures are logged and skipped — one bad title shouldn't kill the
// rest of the batch. Returns the successfully-persisted rows.
func seedAIQuestionsForCategories(
	ctx context.Context,
	repo *repository.QuestionRepo,
	gen agent.QuestionGenerator,
	embedder embeddings.Embedder,
	synth tts.Synthesizer,
	slugs []string,
	count int,
	difficulty string,
) ([]models.Question, error) {
	if gen == nil {
		return nil, nil
	}
	if count <= 0 {
		count = 5
	}
	if count > 10 {
		count = 10
	}

	existing, err := repo.ListTitlesByCategories(ctx, slugs, 100)
	if err != nil {
		log.Printf("seed questions: list titles failed: %v", err)
	}

	genCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	plan, err := gen.Generate(genCtx, agent.GenerateInput{
		Categories:     slugs,
		Difficulty:     difficulty,
		Count:          count,
		ExistingTitles: existing,
	})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(existing)+len(plan.Questions))
	for _, t := range existing {
		seen[normalizeTitle(t)] = true
	}

	created := make([]models.Question, 0, len(plan.Questions))
	for _, pq := range plan.Questions {
		title := strings.TrimSpace(pq.Title)
		answer := strings.TrimSpace(pq.Answer)
		if title == "" || answer == "" {
			continue
		}
		key := normalizeTitle(title)
		if seen[key] {
			continue
		}
		seen[key] = true

		diff := pq.Difficulty
		switch diff {
		case "easy", "medium", "hard":
		default:
			diff = "medium"
		}
		linkCats := pq.Categories
		if len(linkCats) == 0 {
			linkCats = slugs
		}

		q, qErr := repo.Create(ctx, repository.CreateQuestionInput{
			Title:         title,
			Body:          pq.Body,
			Answer:        answer,
			Difficulty:    diff,
			OwnerID:       "",
			Source:        "ai-generated",
			Intent:        pq.Intent,
			CategorySlugs: linkCats,
			IsPublic:      true,
		})
		if qErr != nil {
			log.Printf("seed questions: create failed for %q: %v", title, qErr)
			continue
		}
		embedAndStore(embedder, repo, q)
		synthesizeAndStore(synth, repo, q)
		created = append(created, *q)
	}

	return created, nil
}
