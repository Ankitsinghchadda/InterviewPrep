package repository

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/lib/pq"
)

type StatsRepo struct {
	DB *sql.DB
}

type StatsOverview struct {
	Volume      VolumeStats        `json:"volume"`
	Scoring     ScoringStats       `json:"scoring"`
	Streak      StreakStats        `json:"streak"`
	Trend       []TrendPoint       `json:"trend"`
	Categories  CategoryStrengths  `json:"categories"`
	Themes      ThemeStats         `json:"themes"`
	Difficulty  []DifficultyBucket `json:"difficultyDistribution"`
	Goal        GoalAlignment      `json:"goalAlignment"`
	GeneratedAt time.Time          `json:"generatedAt"`
}

type VolumeStats struct {
	TotalSubmissions      int `json:"totalSubmissions"`
	UniqueQuestions       int `json:"uniqueQuestions"`
	InterviewsStarted     int `json:"interviewsStarted"`
	InterviewsCompleted   int `json:"interviewsCompleted"`
	SubmissionsLast30Days int `json:"submissionsLast30Days"`
}

type ScoringStats struct {
	AverageScore   *float64 `json:"averageScore"`
	AverageLast30  *float64 `json:"averageLast30"`
	AveragePrior30 *float64 `json:"averagePrior30"`
	BestScore      *float64 `json:"bestScore"`
	ScoredCount    int      `json:"scoredCount"`
}

type StreakStats struct {
	Current        int  `json:"current"`
	Longest        int  `json:"longest"`
	PracticedToday bool `json:"practicedToday"`
}

type TrendPoint struct {
	Day         string   `json:"day"`
	Submissions int      `json:"submissions"`
	AvgScore    *float64 `json:"avgScore"`
}

type CategoryScore struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Submissions int     `json:"submissions"`
	AvgScore    float64 `json:"avgScore"`
}

type CategoryStrengths struct {
	Strong []CategoryScore `json:"strong"`
	Weak   []CategoryScore `json:"weak"`
}

type ThemeStats struct {
	Strengths    []ThemeCount `json:"strengths"`
	Improvements []ThemeCount `json:"improvements"`
}

type ThemeCount struct {
	Theme string `json:"theme"`
	Count int    `json:"count"`
}

type DifficultyBucket struct {
	Difficulty  string   `json:"difficulty"`
	Submissions int      `json:"submissions"`
	AvgScore    *float64 `json:"avgScore"`
}

type GoalAlignment struct {
	TargetRole           string  `json:"targetRole"`
	OnTargetSubmissions  int     `json:"onTargetSubmissions"`
	OffTargetSubmissions int     `json:"offTargetSubmissions"`
	AlignmentPercent     float64 `json:"alignmentPercent"`
}

// Overview returns the full dashboard payload for the given user. Each metric
// is computed by a separate small query against existing tables — no new
// schema. All slice fields are initialized to [] so the JSON shape is stable
// for new users with no data.
func (r *StatsRepo) Overview(ctx context.Context, userID, targetRole string, techStack []string) (*StatsOverview, error) {
	out := &StatsOverview{
		Trend:      []TrendPoint{},
		Categories: CategoryStrengths{Strong: []CategoryScore{}, Weak: []CategoryScore{}},
		Themes:     ThemeStats{Strengths: []ThemeCount{}, Improvements: []ThemeCount{}},
		Difficulty: []DifficultyBucket{},
		Goal:       GoalAlignment{TargetRole: targetRole},
	}

	if err := r.loadVolumeAndScoring(ctx, userID, &out.Volume, &out.Scoring); err != nil {
		return nil, err
	}
	if err := r.loadInterviewCounts(ctx, userID, &out.Volume); err != nil {
		return nil, err
	}
	trend, err := r.loadTrend(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.Trend = trend

	strong, weak, err := r.loadCategoryStrengths(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.Categories.Strong = strong
	out.Categories.Weak = weak

	strengths, err := r.loadThemes(ctx, userID, "strengths")
	if err != nil {
		return nil, err
	}
	out.Themes.Strengths = strengths
	improvements, err := r.loadThemes(ctx, userID, "improvements")
	if err != nil {
		return nil, err
	}
	out.Themes.Improvements = improvements

	diff, err := r.loadDifficulty(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.Difficulty = diff

	streak, err := r.loadStreak(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.Streak = streak

	if goalSlugs := buildGoalSlugs(targetRole, techStack); len(goalSlugs) > 0 {
		ga, err := r.loadGoalAlignment(ctx, userID, goalSlugs)
		if err != nil {
			return nil, err
		}
		ga.TargetRole = targetRole
		out.Goal = ga
	}

	out.GeneratedAt = time.Now().UTC()
	return out, nil
}

func (r *StatsRepo) loadVolumeAndScoring(ctx context.Context, userID string, v *VolumeStats, s *ScoringStats) error {
	const q = `
		WITH base AS (
			SELECT score, created_at, question_id
			FROM answer_submissions
			WHERE user_id = $1 AND status = 'complete'
		)
		SELECT
			COUNT(*),
			COUNT(DISTINCT question_id),
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '30 days'),
			AVG(score) FILTER (WHERE score IS NOT NULL),
			AVG(score) FILTER (WHERE score IS NOT NULL
			                   AND created_at >= NOW() - INTERVAL '30 days'),
			AVG(score) FILTER (WHERE score IS NOT NULL
			                   AND created_at >= NOW() - INTERVAL '60 days'
			                   AND created_at <  NOW() - INTERVAL '30 days'),
			MAX(score),
			COUNT(*) FILTER (WHERE score IS NOT NULL)
		FROM base
	`
	var (
		total, unique, last30, scored int
		avg, avg30, prior30, best     sql.NullFloat64
	)
	if err := r.DB.QueryRowContext(ctx, q, userID).Scan(
		&total, &unique, &last30, &avg, &avg30, &prior30, &best, &scored,
	); err != nil {
		return err
	}
	v.TotalSubmissions = total
	v.UniqueQuestions = unique
	v.SubmissionsLast30Days = last30
	s.ScoredCount = scored
	if avg.Valid {
		x := avg.Float64
		s.AverageScore = &x
	}
	if avg30.Valid {
		x := avg30.Float64
		s.AverageLast30 = &x
	}
	if prior30.Valid {
		x := prior30.Float64
		s.AveragePrior30 = &x
	}
	if best.Valid {
		x := best.Float64
		s.BestScore = &x
	}
	return nil
}

func (r *StatsRepo) loadInterviewCounts(ctx context.Context, userID string, v *VolumeStats) error {
	const q = `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'completed')
		FROM interviews WHERE user_id = $1
	`
	return r.DB.QueryRowContext(ctx, q, userID).Scan(&v.InterviewsStarted, &v.InterviewsCompleted)
}

func (r *StatsRepo) loadTrend(ctx context.Context, userID string) ([]TrendPoint, error) {
	const q = `
		WITH days AS (
			SELECT generate_series(
				(CURRENT_DATE - INTERVAL '29 days')::date,
				CURRENT_DATE,
				INTERVAL '1 day'
			)::date AS day
		),
		agg AS (
			SELECT date_trunc('day', created_at)::date AS day,
			       COUNT(*) AS subs,
			       AVG(score) FILTER (WHERE score IS NOT NULL) AS avg_score
			FROM answer_submissions
			WHERE user_id = $1
			  AND status = 'complete'
			  AND created_at >= CURRENT_DATE - INTERVAL '29 days'
			GROUP BY 1
		)
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       COALESCE(a.subs, 0),
		       a.avg_score
		FROM days d LEFT JOIN agg a USING (day)
		ORDER BY d.day
	`
	rows, err := r.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TrendPoint{}
	for rows.Next() {
		var tp TrendPoint
		var avg sql.NullFloat64
		if err := rows.Scan(&tp.Day, &tp.Submissions, &avg); err != nil {
			return nil, err
		}
		if avg.Valid {
			x := avg.Float64
			tp.AvgScore = &x
		}
		out = append(out, tp)
	}
	return out, rows.Err()
}

func (r *StatsRepo) loadCategoryStrengths(ctx context.Context, userID string) ([]CategoryScore, []CategoryScore, error) {
	const q = `
		SELECT c.slug, c.name, COUNT(*)::int, AVG(s.score)::float
		FROM answer_submissions s
		JOIN question_categories qc ON qc.question_id = s.question_id
		JOIN categories c           ON c.id = qc.category_id
		WHERE s.user_id = $1 AND s.status = 'complete' AND s.score IS NOT NULL
		GROUP BY c.slug, c.name
		HAVING COUNT(*) >= 2
		ORDER BY avg(s.score) DESC
	`
	rows, err := r.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	all := []CategoryScore{}
	for rows.Next() {
		var cs CategoryScore
		if err := rows.Scan(&cs.Slug, &cs.Name, &cs.Submissions, &cs.AvgScore); err != nil {
			return nil, nil, err
		}
		all = append(all, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	strong := []CategoryScore{}
	weak := []CategoryScore{}
	n := len(all)
	if n == 0 {
		return strong, weak, nil
	}
	topN := 5
	if n < topN {
		topN = n
	}
	strong = append(strong, all[:topN]...)
	// Weak = bottom up to 5 categories that don't overlap with strong, reversed
	// so the worst-performing comes first.
	if n > topN {
		bottomStart := n - 5
		if bottomStart < topN {
			bottomStart = topN
		}
		weak = append(weak, all[bottomStart:]...)
		sort.Slice(weak, func(i, j int) bool { return weak[i].AvgScore < weak[j].AvgScore })
	}
	return strong, weak, nil
}

func (r *StatsRepo) loadThemes(ctx context.Context, userID, column string) ([]ThemeCount, error) {
	// column is hardcoded by caller to "strengths" or "improvements"; never user input.
	q := `
		SELECT lower(trim(theme)) AS theme, COUNT(*)::int AS n
		FROM answer_submissions s, unnest(s.` + column + `) AS theme
		WHERE s.user_id = $1 AND s.status = 'complete'
		  AND theme IS NOT NULL AND length(trim(theme)) > 0
		GROUP BY 1
		ORDER BY n DESC
		LIMIT 8
	`
	rows, err := r.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ThemeCount{}
	for rows.Next() {
		var tc ThemeCount
		if err := rows.Scan(&tc.Theme, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

func (r *StatsRepo) loadDifficulty(ctx context.Context, userID string) ([]DifficultyBucket, error) {
	const q = `
		SELECT q.difficulty,
		       COUNT(*)::int,
		       AVG(s.score) FILTER (WHERE s.score IS NOT NULL)
		FROM answer_submissions s
		JOIN questions q ON q.id = s.question_id
		WHERE s.user_id = $1 AND s.status = 'complete'
		GROUP BY q.difficulty
		ORDER BY CASE q.difficulty
			WHEN 'easy' THEN 1 WHEN 'medium' THEN 2 WHEN 'hard' THEN 3 ELSE 4 END
	`
	rows, err := r.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DifficultyBucket{}
	for rows.Next() {
		var db DifficultyBucket
		var avg sql.NullFloat64
		if err := rows.Scan(&db.Difficulty, &db.Submissions, &avg); err != nil {
			return nil, err
		}
		if avg.Valid {
			x := avg.Float64
			db.AvgScore = &x
		}
		out = append(out, db)
	}
	return out, rows.Err()
}

// loadStreak fetches distinct submission dates (last 90 days) and computes the
// current and longest consecutive-day streaks in Go.
func (r *StatsRepo) loadStreak(ctx context.Context, userID string) (StreakStats, error) {
	const q = `
		SELECT DISTINCT date_trunc('day', created_at AT TIME ZONE 'UTC')::date
		FROM answer_submissions
		WHERE user_id = $1
		  AND status = 'complete'
		  AND created_at >= NOW() - INTERVAL '90 days'
		ORDER BY 1 DESC
	`
	rows, err := r.DB.QueryContext(ctx, q, userID)
	if err != nil {
		return StreakStats{}, err
	}
	defer rows.Close()

	dates := []time.Time{}
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return StreakStats{}, err
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return StreakStats{}, err
	}

	out := StreakStats{}
	if len(dates) == 0 {
		return out, nil
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	out.PracticedToday = dates[0].Equal(today)

	// Current streak: only counts if most-recent day is today or yesterday.
	if dates[0].Equal(today) || dates[0].Equal(yesterday) {
		out.Current = 1
		for i := 1; i < len(dates); i++ {
			if dates[i-1].Sub(dates[i]) == 24*time.Hour {
				out.Current++
			} else {
				break
			}
		}
	}

	// Longest streak across the 90-day window.
	longest, run := 1, 1
	for i := 1; i < len(dates); i++ {
		if dates[i-1].Sub(dates[i]) == 24*time.Hour {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 1
		}
	}
	out.Longest = longest
	return out, nil
}

func (r *StatsRepo) loadGoalAlignment(ctx context.Context, userID string, goalSlugs []string) (GoalAlignment, error) {
	const q = `
		WITH user_subs AS (
			SELECT s.id, s.question_id
			FROM answer_submissions s
			WHERE s.user_id = $1 AND s.status = 'complete'
		),
		on_target AS (
			SELECT DISTINCT us.id
			FROM user_subs us
			JOIN question_categories qc ON qc.question_id = us.question_id
			JOIN categories c           ON c.id = qc.category_id
			WHERE c.slug = ANY($2::text[])
		)
		SELECT
			(SELECT COUNT(*) FROM on_target),
			(SELECT COUNT(*) FROM user_subs) - (SELECT COUNT(*) FROM on_target)
	`
	var onTarget, offTarget int
	if err := r.DB.QueryRowContext(ctx, q, userID, pq.Array(goalSlugs)).
		Scan(&onTarget, &offTarget); err != nil {
		return GoalAlignment{}, err
	}
	out := GoalAlignment{OnTargetSubmissions: onTarget, OffTargetSubmissions: offTarget}
	total := onTarget + offTarget
	if total > 0 {
		out.AlignmentPercent = float64(onTarget) / float64(total) * 100.0
	}
	return out, nil
}

// buildGoalSlugs merges the user's target role and tech stack into a single
// slug list (deduplicated). Empty when no profile.
func buildGoalSlugs(targetRole string, techStack []string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		s = SlugifyTech(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(targetRole)
	for _, t := range techStack {
		add(t)
	}
	return out
}

// SlugifyTech normalizes a free-text tech name into a category slug. Exported
// so handlers can share the canonicalization logic. Matches the historical
// slugify helper in handlers/question.go.
func SlugifyTech(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ', r == '_', r == '/', r == '.', r == '-':
			out = append(out, '-')
		}
	}
	// Trim surrounding dashes
	start, end := 0, len(out)
	for start < end && out[start] == '-' {
		start++
	}
	for end > start && out[end-1] == '-' {
		end--
	}
	return string(out[start:end])
}
