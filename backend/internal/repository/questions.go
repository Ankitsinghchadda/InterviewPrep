package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

type QuestionRepo struct {
	DB *sql.DB
}

type ListQuestionsFilter struct {
	CategorySlugs []string // any-of match (OR across slugs)
	Difficulty    string
	OwnerID       string // when set, also includes questions owned by this user (private)
	OnlyMine      bool   // when true, return ONLY this user's questions
	Limit         int
}

// selectColumns is shared across SELECTs so all paths populate the same fields.
const selectColumns = `
	q.id, q.slug, q.title, q.body, q.answer, q.difficulty, q.answer_audio_url,
	q.explanation_summary, q.explanation_markdown,
	q.owner_id, q.is_public, q.source, q.intent, q.created_at,
	COALESCE(
	    (SELECT array_agg(c.slug ORDER BY c.sort_order)
	     FROM question_categories qc
	     JOIN categories c ON c.id = qc.category_id
	     WHERE qc.question_id = q.id),
	    ARRAY[]::text[]
	) AS categories
`

// buildScopeFilters appends the standard scope conditions (source, ownership,
// difficulty, categories) into conds and args. The starting placeholder index
// is len(args)+1; conds and args are mutated in place. Returns the empty
// flag = true when the filter is unsatisfiable (OnlyMine without an OwnerID).
func buildScopeFilters(f ListQuestionsFilter, args *[]any, conds *[]string) (empty bool) {
	*conds = append(*conds, `q.source NOT IN ('adaptive', 'live')`)

	if f.OnlyMine {
		if f.OwnerID == "" {
			return true
		}
		*args = append(*args, f.OwnerID)
		*conds = append(*conds, `q.owner_id = $`+itoa(len(*args)))
	} else if f.OwnerID != "" {
		*args = append(*args, f.OwnerID)
		*conds = append(*conds, `(q.is_public = TRUE OR q.owner_id = $`+itoa(len(*args))+`)`)
	} else {
		*conds = append(*conds, `q.is_public = TRUE AND q.owner_id IS NULL`)
	}

	if f.Difficulty != "" {
		*args = append(*args, f.Difficulty)
		*conds = append(*conds, `q.difficulty = $`+itoa(len(*args)))
	}

	if len(f.CategorySlugs) > 0 {
		*args = append(*args, pq.Array(f.CategorySlugs))
		*conds = append(*conds, `q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = ANY($`+itoa(len(*args))+`)
		)`)
	}
	return false
}

func clampListLimit(n int) int {
	if n <= 0 || n > 200 {
		return 100
	}
	return n
}

// List returns curated + user questions matching the filter. Adaptive questions
// are intentionally excluded — they only live inside their owning interview.
func (r *QuestionRepo) List(ctx context.Context, f ListQuestionsFilter) ([]models.Question, error) {
	args := []any{}
	conds := []string{}
	if empty := buildScopeFilters(f, &args, &conds); empty {
		return []models.Question{}, nil
	}

	args = append(args, clampListLimit(f.Limit))
	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY q.created_at DESC
		LIMIT $` + itoa(len(args))

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func (r *QuestionRepo) Get(ctx context.Context, id string) (*models.Question, error) {
	const query = `SELECT ` + selectColumns + ` FROM questions q WHERE q.id = $1`
	row := r.DB.QueryRowContext(ctx, query, id)
	q, err := scanQuestion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return q, err
}

// GetPublicByIDOrSlug looks up a question by slug OR UUID id, but only returns
// it when is_public = TRUE. Used by the no-auth public read path (SEO,
// shareable links). Curated/seeded rows have slugs; user-generated public rows
// may also have one. UUID fallback exists so old links keep working.
func (r *QuestionRepo) GetPublicByIDOrSlug(ctx context.Context, idOrSlug string) (*models.Question, error) {
	const query = `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE q.is_public = TRUE
		  AND (q.slug = $1 OR q.id::text = $1)
		LIMIT 1
	`
	row := r.DB.QueryRowContext(ctx, query, idOrSlug)
	q, err := scanQuestion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return q, err
}

// SitemapEntry is the minimal payload the /sitemap.xml handler needs per
// question: a slug for the URL and a timestamp for <lastmod>. We deliberately
// don't reuse models.Question here — it would pull in body/answer/embedding
// across thousands of rows and blow up memory.
type SitemapEntry struct {
	Slug      string
	UpdatedAt time.Time
}

// ListPublicByCategorySlug returns every is_public question linked to the
// given category slug. Used by the public /topics/:slug landing page (the
// page itself enforces the category exists). Output is ordered by difficulty
// then title so the SEO page renders a consistent, learner-friendly index.
func (r *QuestionRepo) ListPublicByCategorySlug(ctx context.Context, slug string, limit int) ([]models.Question, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	const query = `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE q.is_public = TRUE
		  AND q.source NOT IN ('adaptive', 'live')
		  AND EXISTS (
		    SELECT 1
		    FROM question_categories qc
		    JOIN categories c ON c.id = qc.category_id
		    WHERE qc.question_id = q.id AND c.slug = $1
		  )
		ORDER BY
		  CASE q.difficulty WHEN 'easy' THEN 0 WHEN 'medium' THEN 1 WHEN 'hard' THEN 2 ELSE 3 END,
		  q.title
		LIMIT $2
	`
	rows, err := r.DB.QueryContext(ctx, query, slug, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

// ListPublicForSitemap returns every public question that has a slug, ordered
// by created_at DESC. Caller should set a sensible cap (sitemap.xml protocol
// limit is 50,000 URLs per file).
func (r *QuestionRepo) ListPublicForSitemap(ctx context.Context, limit int) ([]SitemapEntry, error) {
	if limit <= 0 || limit > 50000 {
		limit = 50000
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT slug, created_at
		FROM   questions
		WHERE  is_public = TRUE
		  AND  slug IS NOT NULL
		  AND  source NOT IN ('adaptive', 'live')
		ORDER  BY created_at DESC
		LIMIT  $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SitemapEntry{}
	for rows.Next() {
		var e SitemapEntry
		if err := rows.Scan(&e.Slug, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type CreateQuestionInput struct {
	Title         string
	Body          string
	Answer        string
	Difficulty    string
	OwnerID       string   // empty for public catalog rows (curated / ai-generated)
	Source        string   // 'user' (default) | 'adaptive' | 'ai-generated'
	Intent        string   // free-form tag (e.g. "introduction", "behavioral", "technical")
	CategorySlugs []string // resolved to ids server-side
	IsPublic      bool     // user questions default to FALSE
}

// Create inserts a question and links it to the given categories. When
// OwnerID is empty the row is public catalog content (owner_id NULL).
func (r *QuestionRepo) Create(ctx context.Context, in CreateQuestionInput) (*models.Question, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	source := in.Source
	if source == "" {
		source = "user"
	}

	var ownerArg any
	if in.OwnerID == "" {
		ownerArg = nil
	} else {
		ownerArg = in.OwnerID
	}

	id := uuid.NewString()
	const insertQ = `
		INSERT INTO questions (id, title, body, answer, difficulty, owner_id, is_public, source, intent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.ExecContext(ctx, insertQ,
		id, in.Title, in.Body, in.Answer, in.Difficulty,
		ownerArg, in.IsPublic, source, in.Intent,
	); err != nil {
		return nil, err
	}

	if len(in.CategorySlugs) > 0 {
		const linkQ = `
			INSERT INTO question_categories (question_id, category_id)
			SELECT $1, c.id FROM categories c WHERE c.slug = ANY($2)
			ON CONFLICT DO NOTHING
		`
		if _, err := tx.ExecContext(ctx, linkQ, id, pq.Array(in.CategorySlugs)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// ListTitlesByCategories returns up to `limit` existing question titles from
// the public catalog (curated + ai-generated) that belong to any of the given
// category slugs. Used by the question generator to seed an "avoid these"
// list in the prompt and for post-generation dedup.
func (r *QuestionRepo) ListTitlesByCategories(ctx context.Context, slugs []string, limit int) ([]string, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const query = `
		SELECT q.title
		FROM   questions q
		WHERE  q.owner_id IS NULL
		       AND q.is_public = TRUE
		       AND q.source IN ('curated', 'ai-generated')
		       AND q.id IN (
		           SELECT qc.question_id FROM question_categories qc
		           JOIN   categories c ON c.id = qc.category_id
		           WHERE  c.slug = ANY($1)
		       )
		ORDER  BY LENGTH(q.title) DESC
		LIMIT  $2
	`
	rows, err := r.DB.QueryContext(ctx, query, pq.Array(slugs), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateAudioURL persists the public URL for a question's synthesized
// reference-answer audio. Idempotent — runs even if the row is missing.
func (r *QuestionRepo) UpdateAudioURL(ctx context.Context, id, url string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE questions SET answer_audio_url = $1 WHERE id = $2`, url, id)
	return err
}

// UpdateExplanation persists the learner-facing explanation (a short
// conversational summary plus a markdown body that may contain mermaid
// diagrams). Both fields are written atomically.
func (r *QuestionRepo) UpdateExplanation(ctx context.Context, id, summary, markdown string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE questions SET explanation_summary = $1, explanation_markdown = $2 WHERE id = $3`,
		summary, markdown, id)
	return err
}

// Delete removes a question, but only if it belongs to the caller. Returns
// ErrNotFound when the row doesn't exist OR the owner doesn't match — those
// callers shouldn't be able to distinguish the two.
func (r *QuestionRepo) Delete(ctx context.Context, id, ownerID string) error {
	res, err := r.DB.ExecContext(ctx,
		`DELETE FROM questions WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			return ErrInUse
		}
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PickRandom selects N random curated + user-owned questions matching the
// given category slugs. Adaptive questions are excluded.
func (r *QuestionRepo) PickRandom(ctx context.Context, categorySlugs []string, ownerID string, n int) ([]models.Question, error) {
	if n <= 0 {
		return nil, nil
	}
	args := []any{}
	conds := []string{`q.source NOT IN ('adaptive', 'live')`}

	if ownerID != "" {
		args = append(args, ownerID)
		conds = append(conds, `(q.is_public = TRUE OR q.owner_id = $`+itoa(len(args))+`)`)
	} else {
		conds = append(conds, `q.is_public = TRUE AND q.owner_id IS NULL`)
	}

	if len(categorySlugs) > 0 {
		args = append(args, pq.Array(categorySlugs))
		conds = append(conds, `q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = ANY($`+itoa(len(args))+`)
		)`)
	}

	args = append(args, n)
	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY random()
		LIMIT $` + itoa(len(args))

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

// RecommendationItem pairs a question with a human-readable rationale that
// explains why it was surfaced (e.g. "You averaged 42% in databases").
type RecommendationItem struct {
	Question models.Question `json:"question"`
	Reason   string          `json:"reason"`
}

type RecommendationBuckets struct {
	WeakAreas []RecommendationItem `json:"weakAreas"`
	LevelUp   []RecommendationItem `json:"levelUp"`
	GoalGaps  []RecommendationItem `json:"goalGaps"`
}

type RecommendInput struct {
	UserID    string
	GoalSlugs []string // target_role + slugified tech_stack
	PerBucket int      // questions per bucket (default 4, max 6)
}

// Recommendations returns three performance-aware buckets of curated questions:
//   - weakAreas: pick from categories where the user's avg score is < 60 (≥3 attempts)
//   - levelUp: next-difficulty questions in categories the user is acing (avg ≥ 80, ≥3 attempts)
//   - goalGaps: questions in target_role/tech_stack categories the user hasn't tried
//
// Each item carries a Reason string so the dashboard can explain why it's
// recommending the question. Cross-bucket dedup: weakAreas IDs are excluded
// from levelUp and goalGaps; levelUp IDs are excluded from goalGaps.
func (r *QuestionRepo) Recommendations(ctx context.Context, in RecommendInput) (*RecommendationBuckets, error) {
	per := in.PerBucket
	if per <= 0 {
		per = 4
	}
	if per > 6 {
		per = 6
	}

	out := &RecommendationBuckets{
		WeakAreas: []RecommendationItem{},
		LevelUp:   []RecommendationItem{},
		GoalGaps:  []RecommendationItem{},
	}

	// Compute per-category stats once (slug → {avg, attempts, max difficulty ordinal}).
	catStats, err := r.userCategoryStats(ctx, in.UserID)
	if err != nil {
		return nil, err
	}

	exclude := map[string]bool{}

	// Bucket 1: weakAreas — categories where avg < 60 & attempts >= 3, worst first.
	weakCats := filterCats(catStats, func(c userCategoryStat) bool {
		return c.attempts >= 3 && c.avgScore < 60
	})
	sort.Slice(weakCats, func(i, j int) bool { return weakCats[i].avgScore < weakCats[j].avgScore })

	for _, c := range weakCats {
		if len(out.WeakAreas) >= per {
			break
		}
		qs, err := r.pickForCategory(ctx, in.UserID, c.slug, ordinalToDifficulty(c.maxDifficulty), exclude, per-len(out.WeakAreas))
		if err != nil {
			return nil, err
		}
		reason := fmt.Sprintf("You averaged %d%% in %s (%d attempts)", int(c.avgScore), c.name, c.attempts)
		for _, q := range qs {
			out.WeakAreas = append(out.WeakAreas, RecommendationItem{Question: q, Reason: reason})
			exclude[q.ID] = true
		}
	}

	// Bucket 2: levelUp — categories with avg >= 80 & attempts >= 3, jump to next difficulty.
	strongCats := filterCats(catStats, func(c userCategoryStat) bool {
		return c.attempts >= 3 && c.avgScore >= 80
	})
	sort.Slice(strongCats, func(i, j int) bool { return strongCats[i].avgScore > strongCats[j].avgScore })

	for _, c := range strongCats {
		if len(out.LevelUp) >= per {
			break
		}
		current := ordinalToDifficulty(c.maxDifficulty)
		next := nextDifficulty(current)
		if next == "" {
			continue
		}
		qs, err := r.pickForCategory(ctx, in.UserID, c.slug, next, exclude, per-len(out.LevelUp))
		if err != nil {
			return nil, err
		}
		reason := fmt.Sprintf("Averaging %d%% on %s %s — try %s", int(c.avgScore), current, c.name, next)
		for _, q := range qs {
			out.LevelUp = append(out.LevelUp, RecommendationItem{Question: q, Reason: reason})
			exclude[q.ID] = true
		}
	}

	// Bucket 3: goalGaps — never-practiced questions in the user's goal categories.
	if len(in.GoalSlugs) > 0 {
		// Exclude categories the user has already practiced in (so this stays
		// truly "gaps"); fall back to all goal slugs if every goal category has
		// some submissions already.
		practiced := map[string]bool{}
		for _, c := range catStats {
			practiced[c.slug] = true
		}
		gapSlugs := []string{}
		for _, s := range in.GoalSlugs {
			if !practiced[s] {
				gapSlugs = append(gapSlugs, s)
			}
		}
		if len(gapSlugs) == 0 {
			gapSlugs = in.GoalSlugs
		}
		qs, err := r.pickGoalGaps(ctx, in.UserID, gapSlugs, exclude, per)
		if err != nil {
			return nil, err
		}
		for _, q := range qs {
			reason := buildGoalReason(q.Categories, in.GoalSlugs)
			out.GoalGaps = append(out.GoalGaps, RecommendationItem{Question: q, Reason: reason})
			exclude[q.ID] = true
		}
	}

	// Fallback: brand-new user (no submissions yet, no goal matches). Surface
	// a random curated sample so the dashboard never feels empty.
	if len(out.WeakAreas) == 0 && len(out.LevelUp) == 0 && len(out.GoalGaps) == 0 {
		qs, err := r.PickRandom(ctx, nil, in.UserID, per)
		if err == nil {
			for _, q := range qs {
				out.GoalGaps = append(out.GoalGaps, RecommendationItem{
					Question: q,
					Reason:   "Start with a popular curated question",
				})
			}
		}
	}

	return out, nil
}

type userCategoryStat struct {
	slug          string
	name          string
	attempts      int
	avgScore      float64
	maxDifficulty int // 1=easy, 2=medium, 3=hard
}

// userCategoryStats aggregates the user's submission performance by category.
// Only counts scored, complete submissions. The max-difficulty ordinal lets
// callers pick the user's current working level per category.
func (r *QuestionRepo) userCategoryStats(ctx context.Context, userID string) ([]userCategoryStat, error) {
	const query = `
		SELECT c.slug, c.name, COUNT(*)::int, AVG(s.score)::float,
		       MAX(CASE q.difficulty
		           WHEN 'easy' THEN 1 WHEN 'medium' THEN 2 WHEN 'hard' THEN 3
		           ELSE 0 END)::int
		FROM answer_submissions s
		JOIN questions q            ON q.id = s.question_id
		JOIN question_categories qc ON qc.question_id = q.id
		JOIN categories c           ON c.id = qc.category_id
		WHERE s.user_id = $1 AND s.status = 'complete' AND s.score IS NOT NULL
		GROUP BY c.slug, c.name
	`
	rows, err := r.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []userCategoryStat{}
	for rows.Next() {
		var c userCategoryStat
		if err := rows.Scan(&c.slug, &c.name, &c.attempts, &c.avgScore, &c.maxDifficulty); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// pickForCategory selects up to `limit` curated, never-attempted questions in
// the given category. When difficulty is non-empty it's used as a hard filter;
// callers fall back to no-difficulty by passing "".
func (r *QuestionRepo) pickForCategory(
	ctx context.Context,
	userID, categorySlug, difficulty string,
	exclude map[string]bool,
	limit int,
) ([]models.Question, error) {
	if limit <= 0 {
		return nil, nil
	}
	args := []any{userID, categorySlug}
	conds := []string{
		`q.source = 'curated'`,
		`q.is_public = TRUE`,
		`q.id NOT IN (SELECT question_id FROM answer_submissions WHERE user_id = $1)`,
		`q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = $2
		)`,
	}
	if difficulty != "" {
		args = append(args, difficulty)
		conds = append(conds, `q.difficulty = $`+itoa(len(args)))
	}
	if len(exclude) > 0 {
		args = append(args, pq.Array(excludeIDs(exclude)))
		conds = append(conds, `q.id <> ALL($`+itoa(len(args))+`::uuid[])`)
	}
	args = append(args, limit)
	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY random()
		LIMIT $` + itoa(len(args))
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

// pickGoalGaps selects curated questions in any of the given category slugs
// that the user has never submitted an answer for.
func (r *QuestionRepo) pickGoalGaps(
	ctx context.Context,
	userID string,
	slugs []string,
	exclude map[string]bool,
	limit int,
) ([]models.Question, error) {
	if limit <= 0 || len(slugs) == 0 {
		return nil, nil
	}
	args := []any{userID, pq.Array(slugs)}
	conds := []string{
		`q.source = 'curated'`,
		`q.is_public = TRUE`,
		`q.id NOT IN (SELECT question_id FROM answer_submissions WHERE user_id = $1)`,
		`q.id IN (
			SELECT qc.question_id FROM question_categories qc
			JOIN categories c ON c.id = qc.category_id
			WHERE c.slug = ANY($2)
		)`,
	}
	if len(exclude) > 0 {
		args = append(args, pq.Array(excludeIDs(exclude)))
		conds = append(conds, `q.id <> ALL($`+itoa(len(args))+`::uuid[])`)
	}
	args = append(args, limit)
	query := `
		SELECT ` + selectColumns + `
		FROM questions q
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY random()
		LIMIT $` + itoa(len(args))
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func filterCats(cats []userCategoryStat, pred func(userCategoryStat) bool) []userCategoryStat {
	out := []userCategoryStat{}
	for _, c := range cats {
		if pred(c) {
			out = append(out, c)
		}
	}
	return out
}

func excludeIDs(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func ordinalToDifficulty(n int) string {
	switch n {
	case 1:
		return "easy"
	case 2:
		return "medium"
	case 3:
		return "hard"
	}
	return ""
}

func nextDifficulty(d string) string {
	switch d {
	case "easy":
		return "medium"
	case "medium":
		return "hard"
	case "hard":
		return "hard"
	}
	return ""
}

// buildGoalReason picks the first category on the question that matches a
// goal slug, for a friendly "Targets your goal: kubernetes" annotation.
func buildGoalReason(qCats, goalSlugs []string) string {
	goals := map[string]bool{}
	for _, g := range goalSlugs {
		goals[g] = true
	}
	for _, c := range qCats {
		if goals[c] {
			return "Targets your goal: " + c + " (not yet practiced)"
		}
	}
	return "Aligned with your target role"
}

// rowScanner abstracts *sql.Row and *sql.Rows so scanQuestion accepts both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanQuestion(row rowScanner) (*models.Question, error) {
	var (
		q     models.Question
		slug  sql.NullString
		owner sql.NullString
		cats  pq.StringArray
	)
	if err := row.Scan(
		&q.ID, &slug, &q.Title, &q.Body, &q.Answer, &q.Difficulty, &q.AnswerAudioURL,
		&q.ExplanationSummary, &q.ExplanationMarkdown,
		&owner, &q.IsPublic, &q.Source, &q.Intent, &q.CreatedAt, &cats,
	); err != nil {
		return nil, err
	}
	if slug.Valid {
		s := slug.String
		q.Slug = &s
	}
	if owner.Valid {
		s := owner.String
		q.OwnerID = &s
	}
	q.Categories = []string(cats)
	if q.Categories == nil {
		q.Categories = []string{}
	}
	return &q, nil
}

func itoa(i int) string { return strconv.Itoa(i) }

// ---- Search (hybrid: pgvector + full-text + RRF) --------------------------

type SearchFilter struct {
	ListQuestionsFilter
	Query          string    // raw user text (required)
	QueryEmbedding []float32 // optional; when nil falls back to keyword-only
}

// SearchHit wraps a Question with a relevance score and an optional HTML
// snippet (already escaped, with <mark> tags around matched tokens).
type SearchHit struct {
	models.Question
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet,omitempty"`
}

// Search runs a hybrid keyword + semantic search and returns hits ranked by
// Reciprocal Rank Fusion. If QueryEmbedding is nil (e.g. embedding service
// unavailable), it degrades to keyword-only.
func (r *QuestionRepo) Search(ctx context.Context, f SearchFilter) ([]SearchHit, error) {
	query := strings.TrimSpace(f.Query)
	if query == "" {
		return nil, errors.New("search: empty query")
	}
	args := []any{}
	conds := []string{}
	if empty := buildScopeFilters(f.ListQuestionsFilter, &args, &conds); empty {
		return []SearchHit{}, nil
	}
	scopeSQL := strings.Join(conds, " AND ")

	args = append(args, query)
	tsqIdx := itoa(len(args))

	hasVec := len(f.QueryEmbedding) == embeddings_dim
	var vecIdx string
	if hasVec {
		args = append(args, pgvector.NewVector(f.QueryEmbedding))
		vecIdx = itoa(len(args))
	}

	args = append(args, clampListLimit(f.Limit))
	limitIdx := itoa(len(args))

	// Two CTEs (semantic, keyword) feeding a fused CTE via RRF (k=60).
	// When embeddings aren't available, the semantic CTE returns no rows
	// (the WHERE clause filters them all out) and fusion equals pure keyword.
	semanticCTE := `semantic AS (SELECT NULL::uuid AS id, 0::bigint AS rank WHERE FALSE)`
	if hasVec {
		semanticCTE = `semantic AS (
			SELECT q.id,
			       ROW_NUMBER() OVER (ORDER BY q.embedding <=> $` + vecIdx + `) AS rank
			FROM   questions q
			WHERE  q.embedding IS NOT NULL AND ` + scopeSQL + `
			ORDER  BY q.embedding <=> $` + vecIdx + `
			LIMIT  50
		)`
	}

	sqlStr := `
		WITH ` + semanticCTE + `,
		keyword AS (
			SELECT q.id,
			       ROW_NUMBER() OVER (
			         ORDER BY ts_rank(q.search_text, plainto_tsquery('english', $` + tsqIdx + `)) DESC
			       ) AS rank
			FROM   questions q
			WHERE  q.search_text @@ plainto_tsquery('english', $` + tsqIdx + `)
			       AND ` + scopeSQL + `
			LIMIT  50
		),
		fused AS (
			SELECT id, SUM(1.0 / (60 + rank))::float AS score
			FROM (
				SELECT * FROM semantic
				UNION ALL
				SELECT * FROM keyword
			) u
			WHERE id IS NOT NULL
			GROUP BY id
			ORDER BY score DESC
			LIMIT $` + limitIdx + `
		)
		SELECT ` + selectColumns + `,
		       f.score,
		       COALESCE(
		         ts_headline('english',
		           NULLIF(q.body, ''),
		           plainto_tsquery('english', $` + tsqIdx + `),
		           'MaxFragments=1,MaxWords=24,MinWords=10,StartSel=<mark>,StopSel=</mark>'
		         ),
		         ''
		       ) AS snippet
		FROM   fused f
		JOIN   questions q ON q.id = f.id
		ORDER  BY f.score DESC
	`

	rows, err := r.DB.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SearchHit{}
	for rows.Next() {
		hit, err := scanSearchHit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *hit)
	}
	return out, rows.Err()
}

// embeddings_dim must match the vector(N) column type and the model's output
// dimensionality. Kept here (not imported) to avoid a repo↔services cycle.
const embeddings_dim = 768

func scanSearchHit(row rowScanner) (*SearchHit, error) {
	var (
		hit     SearchHit
		slug    sql.NullString
		owner   sql.NullString
		cats    pq.StringArray
		snippet sql.NullString
	)
	if err := row.Scan(
		&hit.ID, &slug, &hit.Title, &hit.Body, &hit.Answer, &hit.Difficulty, &hit.AnswerAudioURL,
		&hit.ExplanationSummary, &hit.ExplanationMarkdown,
		&owner, &hit.IsPublic, &hit.Source, &hit.Intent, &hit.CreatedAt, &cats,
		&hit.Score, &snippet,
	); err != nil {
		return nil, err
	}
	if slug.Valid {
		s := slug.String
		hit.Slug = &s
	}
	if owner.Valid {
		s := owner.String
		hit.OwnerID = &s
	}
	hit.Categories = []string(cats)
	if hit.Categories == nil {
		hit.Categories = []string{}
	}
	if snippet.Valid {
		hit.Snippet = snippet.String
	}
	return &hit, nil
}

// ---- Semantic dedup (FindSimilar) ----------------------------------------

// FindSimilarFilter describes a pure-semantic similarity search used to flag
// near-duplicate questions before/at create time. Unlike Search, this path
// doesn't blend in a keyword signal — the user is still typing, so we only
// trust the embedding.
type FindSimilarFilter struct {
	Embedding []float32
	OwnerID   string  // includes caller's own private questions when set
	Limit     int     // top-K; clamps to [1, 20]
	MinScore  float64 // drop rows with cosine similarity below this
}

// SimilarQuestion wraps a Question with its cosine similarity (0..1) to the
// query vector. Frontends display the score as a "92% match" badge.
type SimilarQuestion struct {
	models.Question
	Similarity float64 `json:"similarity"`
}

// FindSimilar returns the top-K rows nearest to the query vector under cosine
// distance, after applying the standard visibility filter. Rows below
// MinScore are dropped. Caller is responsible for choosing a reasonable
// threshold (typical: 0.78 warn, 0.88 block).
func (r *QuestionRepo) FindSimilar(ctx context.Context, f FindSimilarFilter) ([]SimilarQuestion, error) {
	if len(f.Embedding) != embeddings_dim {
		return nil, fmt.Errorf("FindSimilar: expected vector of dim %d, got %d", embeddings_dim, len(f.Embedding))
	}
	limit := f.Limit
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	args := []any{pgvector.NewVector(f.Embedding)}
	vis := `q.is_public = TRUE AND q.owner_id IS NULL`
	if f.OwnerID != "" {
		args = append(args, f.OwnerID)
		vis = `(q.is_public = TRUE OR q.owner_id = $` + itoa(len(args)) + `)`
	}
	args = append(args, limit)
	limitIdx := itoa(len(args))

	sqlStr := `
		SELECT ` + selectColumns + `,
		       1 - (q.embedding <=> $1) AS similarity
		FROM   questions q
		WHERE  q.embedding IS NOT NULL
		       AND q.source NOT IN ('adaptive', 'live')
		       AND ` + vis + `
		ORDER  BY q.embedding <=> $1
		LIMIT  $` + limitIdx

	rows, err := r.DB.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SimilarQuestion{}
	for rows.Next() {
		sim, err := scanSimilarQuestion(rows)
		if err != nil {
			return nil, err
		}
		if sim.Similarity < f.MinScore {
			continue
		}
		out = append(out, *sim)
	}
	return out, rows.Err()
}

func scanSimilarQuestion(row rowScanner) (*SimilarQuestion, error) {
	var (
		sim   SimilarQuestion
		slug  sql.NullString
		owner sql.NullString
		cats  pq.StringArray
	)
	if err := row.Scan(
		&sim.ID, &slug, &sim.Title, &sim.Body, &sim.Answer, &sim.Difficulty, &sim.AnswerAudioURL,
		&sim.ExplanationSummary, &sim.ExplanationMarkdown,
		&owner, &sim.IsPublic, &sim.Source, &sim.Intent, &sim.CreatedAt, &cats,
		&sim.Similarity,
	); err != nil {
		return nil, err
	}
	if slug.Valid {
		s := slug.String
		sim.Slug = &s
	}
	if owner.Valid {
		s := owner.String
		sim.OwnerID = &s
	}
	sim.Categories = []string(cats)
	if sim.Categories == nil {
		sim.Categories = []string{}
	}
	return &sim, nil
}

// UpdateEmbedding writes a 768-dim vector for the given question. Idempotent.
func (r *QuestionRepo) UpdateEmbedding(ctx context.Context, id string, vec []float32) error {
	if len(vec) != embeddings_dim {
		return fmt.Errorf("expected vector of dim %d, got %d", embeddings_dim, len(vec))
	}
	_, err := r.DB.ExecContext(ctx,
		`UPDATE questions SET embedding = $1 WHERE id = $2`,
		pgvector.NewVector(vec), id,
	)
	return err
}

// QuestionEmbeddingInput is one row of the backfill batch — id + the text to
// embed (typically title + body + answer concatenation).
type QuestionEmbeddingInput struct {
	ID   string
	Text string
}

// ListNeedingEmbedding returns up to `limit` rows whose embedding column is
// NULL. Used by the backfill command and (optionally) a periodic reconciler.
func (r *QuestionRepo) ListNeedingEmbedding(ctx context.Context, limit int) ([]QuestionEmbeddingInput, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, title, body, answer
		FROM   questions
		WHERE  embedding IS NULL
		ORDER  BY created_at
		LIMIT  $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QuestionEmbeddingInput{}
	for rows.Next() {
		var id, title, body, answer string
		if err := rows.Scan(&id, &title, &body, &answer); err != nil {
			return nil, err
		}
		out = append(out, QuestionEmbeddingInput{
			ID:   id,
			Text: buildEmbedText(title, body, answer),
		})
	}
	return out, rows.Err()
}

// buildEmbedText is shared by backfill and the on-create goroutine so the
// stored vectors are derived from the same input shape.
func BuildEmbedText(title, body, answer string) string {
	return buildEmbedText(title, body, answer)
}

func buildEmbedText(title, body, answer string) string {
	parts := []string{title}
	if body != "" {
		parts = append(parts, body)
	}
	if answer != "" {
		parts = append(parts, answer)
	}
	return strings.Join(parts, "\n\n")
}
