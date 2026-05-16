package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/models"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/submissions"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
	"github.com/Ankitsinghchadda/InterviewPrep/pkg/response"
	"github.com/go-chi/chi/v5"
)

type InterviewHandler struct {
	Interviews      *repository.InterviewRepo
	Questions       *repository.QuestionRepo
	Categories      *repository.CategoryRepo
	Submissions     *repository.SubmissionRepo
	Profiles        *repository.ProfileRepo
	Submitter       *submissions.Service
	Aggregator      agent.Aggregator
	Designer        agent.InterviewDesigner
	LiveInterviewer agent.LiveInterviewer
	Synth           tts.Synthesizer
	Billing         *billing.Service
	MaxAudioBytes   int64
}

type startInterviewBody struct {
	Mode            string   `json:"mode"`            // "topic" (default) | "adaptive" | "live"
	CategorySlugs   []string `json:"categories"`      // ignored in adaptive/live mode
	Count           int      `json:"count"`           // topic/adaptive
	DurationMinutes int      `json:"durationMinutes"` // live only: 15|30|45
	JobDescription  string   `json:"jobDescription"`  // live only, optional; pasted by the candidate
}

// maxJobDescriptionChars caps the JD we accept from the client; the prompt
// builder additionally truncates to 4000 chars before sending to the model.
const maxJobDescriptionChars = 8000

// Start creates an interview. In "topic" mode it picks random questions
// matching the chosen categories. In "adaptive" mode it asks the designer
// agent to generate a tailored question plan from the caller's profile.
func (h *InterviewHandler) Start(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body startInterviewBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Mode == "" {
		body.Mode = "topic"
	}
	if body.Mode != "topic" && body.Mode != "adaptive" && body.Mode != "live" {
		response.Err(w, http.StatusBadRequest, "mode must be 'topic', 'adaptive', or 'live'")
		return
	}

	if body.Mode == "live" {
		switch body.DurationMinutes {
		case 15, 30, 45:
		default:
			response.Err(w, http.StatusBadRequest, "durationMinutes must be 15, 30, or 45")
			return
		}
		jd := strings.TrimSpace(body.JobDescription)
		if len(jd) > maxJobDescriptionChars {
			jd = jd[:maxJobDescriptionChars]
		}
		h.startLive(w, r, userID, body.DurationMinutes, jd)
		return
	}

	if body.Count <= 0 {
		body.Count = 5
	}
	if body.Count > 12 {
		body.Count = 12
	}

	if body.Mode == "adaptive" {
		h.startAdaptive(w, r, userID, body.Count)
		return
	}
	h.startTopic(w, r, userID, body.CategorySlugs, body.Count)
}

func (h *InterviewHandler) startTopic(w http.ResponseWriter, r *http.Request, userID string, slugs []string, count int) {
	if !checkQuota(w, r, h.Billing, billing.KindMockBasic) {
		return
	}
	questions, err := h.Questions.PickRandom(r.Context(), slugs, userID, count)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to pick questions")
		return
	}
	if len(questions) == 0 {
		response.Err(w, http.StatusBadRequest, "no questions match the selected categories")
		return
	}
	categoryIDs, err := h.Categories.IDsBySlugs(r.Context(), slugs)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to resolve categories")
		return
	}
	questionIDs := make([]string, len(questions))
	for i, q := range questions {
		questionIDs[i] = q.ID
	}
	iv, err := h.Interviews.Create(r.Context(), repository.CreateInterviewInput{
		UserID:      userID,
		Mode:        "topic",
		CategoryIDs: categoryIDs,
		QuestionIDs: questionIDs,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create interview")
		return
	}
	iv.Questions = questions
	safeRecord(r.Context(), h.Billing, billing.KindMockBasic, map[string]any{"mode": "topic", "interview_id": iv.ID})
	response.OK(w, http.StatusCreated, iv)
}

func (h *InterviewHandler) startAdaptive(w http.ResponseWriter, r *http.Request, userID string, count int) {
	if h.Designer == nil {
		response.Err(w, http.StatusServiceUnavailable, "adaptive interviews are not configured")
		return
	}
	if !checkQuota(w, r, h.Billing, billing.KindMockBasic) {
		return
	}

	profile, err := h.Profiles.Get(r.Context(), userID)
	if errors.Is(err, repository.ErrNotFound) || profile == nil {
		response.Err(w, http.StatusBadRequest, "please complete onboarding before starting an adaptive interview")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	// Design (and persist) with a fresh time-bounded context so a slow LLM
	// doesn't hold the request open forever.
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	plan, err := h.Designer.Design(ctx, agent.DesignInput{
		TargetRole:      profile.TargetRole,
		YearsExperience: profile.YearsExperience,
		Seniority:       profile.Seniority,
		CurrentRole:     profile.CurrentRole,
		TechStack:       profile.TechStack,
		Goals:           profile.Goals,
		ResumeText:      profile.ResumeText,
		Count:           count,
	})
	if err != nil {
		response.Err(w, http.StatusBadGateway, "interview generation failed: "+err.Error())
		return
	}
	if len(plan.Questions) == 0 {
		response.Err(w, http.StatusBadGateway, "designer returned no questions")
		return
	}

	// Persist generated questions as adaptive (private, user-owned).
	created := make([]models.Question, 0, len(plan.Questions))
	questionIDs := make([]string, 0, len(plan.Questions))
	for _, pq := range plan.Questions {
		diff := pq.Difficulty
		switch diff {
		case "easy", "medium", "hard":
		default:
			diff = "medium"
		}
		q, err := h.Questions.Create(r.Context(), repository.CreateQuestionInput{
			Title:         pq.Title,
			Body:          pq.Body,
			Answer:        pq.Answer,
			Difficulty:    diff,
			OwnerID:       userID,
			Source:        "adaptive",
			Intent:        pq.Intent,
			CategorySlugs: pq.Categories,
			IsPublic:      false,
		})
		if err != nil {
			response.Err(w, http.StatusInternalServerError, "failed to persist generated question")
			return
		}
		synthesizeAndStore(h.Synth, h.Questions, q)
		created = append(created, *q)
		questionIDs = append(questionIDs, q.ID)
	}

	iv, err := h.Interviews.Create(r.Context(), repository.CreateInterviewInput{
		UserID:      userID,
		Mode:        "adaptive",
		CategoryIDs: nil,
		QuestionIDs: questionIDs,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create interview")
		return
	}
	iv.Questions = created
	safeRecord(r.Context(), h.Billing, billing.KindMockBasic, map[string]any{"mode": "adaptive", "interview_id": iv.ID})
	response.OK(w, http.StatusCreated, iv)
}

// startLive begins an agentic, time-bounded interview. The first question is
// generated immediately by the live interviewer agent and persisted as a
// regular question row (source='live'). Subsequent questions are generated
// turn-by-turn via NextQuestion.
func (h *InterviewHandler) startLive(w http.ResponseWriter, r *http.Request, userID string, durationMin int, jobDescription string) {
	if h.LiveInterviewer == nil {
		response.Err(w, http.StatusServiceUnavailable, "live interviews are not configured")
		return
	}
	if !checkQuota(w, r, h.Billing, billing.KindMockLive) {
		return
	}

	// Profile is optional for live mode — fall back to an empty input if absent.
	var design agent.DesignInput
	profile, err := h.Profiles.Get(r.Context(), userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	if profile != nil {
		design = agent.DesignInput{
			TargetRole:      profile.TargetRole,
			YearsExperience: profile.YearsExperience,
			Seniority:       profile.Seniority,
			CurrentRole:     profile.CurrentRole,
			TechStack:       profile.TechStack,
			Goals:           profile.Goals,
			ResumeText:      profile.ResumeText,
		}
	}
	design.JobDescription = jobDescription

	durationSeconds := durationMin * 60

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	next, err := h.LiveInterviewer.NextQuestion(ctx, agent.NextQuestionInput{
		Profile:          design,
		History:          nil,
		TimeRemainingSec: durationSeconds,
		TotalDurationSec: durationSeconds,
		IsFirst:          true,
	})
	if err != nil {
		response.Err(w, http.StatusBadGateway, "live interview generation failed: "+err.Error())
		return
	}

	q, err := h.persistLiveQuestion(r.Context(), userID, next)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to persist generated question")
		return
	}

	iv, err := h.Interviews.Create(r.Context(), repository.CreateInterviewInput{
		UserID:          userID,
		Mode:            "live",
		QuestionIDs:     []string{q.ID},
		DurationSeconds: durationSeconds,
		JobDescription:  jobDescription,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create interview")
		return
	}
	iv.Questions = []models.Question{*q}
	safeRecord(r.Context(), h.Billing, billing.KindMockLive, map[string]any{"interview_id": iv.ID, "duration_min": durationMin})
	response.OK(w, http.StatusCreated, iv)
}

// persistLiveQuestion writes a generated NextQuestion to the questions table,
// marked source='live' and private to the user.
func (h *InterviewHandler) persistLiveQuestion(ctx context.Context, userID string, nq *agent.NextQuestion) (*models.Question, error) {
	diff := nq.Difficulty
	switch diff {
	case "easy", "medium", "hard":
	default:
		diff = "medium"
	}
	q, err := h.Questions.Create(ctx, repository.CreateQuestionInput{
		Title:         nq.Title,
		Body:          nq.Body,
		Answer:        nq.Answer,
		Difficulty:    diff,
		OwnerID:       userID,
		Source:        "live",
		Intent:        nq.Intent,
		CategorySlugs: nq.Categories,
		IsPublic:      false,
	})
	if err == nil {
		synthesizeAndStore(h.Synth, h.Questions, q)
		// Live interviews need the interviewer voice asking the question,
		// kicked off in parallel with answer-audio synthesis. The candidate
		// hears it as soon as it's ready, before they start recording.
		synthesizePromptAndStore(h.Synth, h.Questions, q)
	}
	return q, err
}

type nextQuestionResponse struct {
	Question         *models.Question `json:"question,omitempty"`
	Wrap             bool             `json:"wrap"`
	TimeRemainingSec int              `json:"timeRemainingSec"`
}

// NextQuestion advances a live interview by one turn. It requires the latest
// question to already have a fully-reviewed submission, then asks the live
// interviewer agent for the next question (or to wrap). Server-side timer
// rejects calls after the duration has elapsed.
func (h *InterviewHandler) NextQuestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.LiveInterviewer == nil {
		response.Err(w, http.StatusServiceUnavailable, "live interviews are not configured")
		return
	}

	interviewID := chi.URLParam(r, "id")
	iv, err := h.Interviews.Get(r.Context(), interviewID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "interview not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load interview")
		return
	}
	if iv.Mode != "live" {
		response.Err(w, http.StatusBadRequest, "interview is not in live mode")
		return
	}
	if iv.Status != "in_progress" {
		response.Err(w, http.StatusConflict, "interview is no longer in progress")
		return
	}

	elapsed := int(time.Since(iv.StartedAt).Seconds())
	timeRemaining := iv.DurationSeconds - elapsed
	if timeRemaining <= 0 {
		response.Err(w, http.StatusConflict, "time_expired")
		return
	}

	questions, err := h.Interviews.QuestionsFor(r.Context(), interviewID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load interview questions")
		return
	}
	subs, err := h.Submissions.ListForInterview(r.Context(), interviewID, userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load submissions")
		return
	}

	// Idempotency: if the latest question already has no submission, just
	// return it (protects against double-clicks generating duplicate questions).
	if len(questions) > 0 && len(questions) > len(subs) {
		last := questions[len(questions)-1]
		response.OK(w, http.StatusOK, nextQuestionResponse{
			Question:         &last,
			Wrap:             false,
			TimeRemainingSec: timeRemaining,
		})
		return
	}

	// Require the latest submission to be terminal before generating the next.
	if len(subs) > 0 {
		last := subs[len(subs)-1]
		if last.Status != "complete" && last.Status != "failed" {
			response.Err(w, http.StatusConflict, "previous answer is still being reviewed")
			return
		}
	}

	// Build history by zipping ordered questions with submissions, and at the
	// same time derive the coverage / score / pacing signals the live agent
	// uses to decide between follow-up, pivot, and close.
	subByQ := map[string]int{}
	for i, s := range subs {
		subByQ[s.QuestionID] = i
	}
	history := make([]agent.LiveTurn, 0, len(questions))
	intentsCovered := map[string]int{}
	var (
		scoreSum float64
		scoreN   int
		turnSum  int
		turnN    int
	)
	for _, q := range questions {
		intentsCovered[q.Intent]++
		idx, ok := subByQ[q.ID]
		if !ok {
			continue
		}
		s := subs[idx]
		turn := agent.LiveTurn{
			QuestionTitle:   q.Title,
			QuestionIntent:  q.Intent,
			CandidateAnswer: s.Transcript,
			AnswerStrengths: s.Strengths,
			AnswerGaps:      s.Improvements,
		}
		if s.Score != nil {
			score := *s.Score
			turn.AnswerScore = &score
			scoreSum += score
			scoreN++
		}
		if !s.UpdatedAt.IsZero() && !q.CreatedAt.IsZero() {
			d := int(s.UpdatedAt.Sub(q.CreatedAt).Seconds())
			if d > 0 && d < 1200 { // sanity bound
				turn.TurnDurationSec = d
				turnSum += d
				turnN++
			}
		}
		history = append(history, turn)
	}
	var avgScore *float64
	if scoreN > 0 {
		v := scoreSum / float64(scoreN)
		avgScore = &v
	}
	avgTurn := 240
	if turnN > 0 {
		avgTurn = turnSum / turnN
	}
	denom := avgTurn
	if denom < 60 {
		denom = 60
	}
	expectedRemaining := timeRemaining / denom

	// Profile is optional.
	var design agent.DesignInput
	if profile, err := h.Profiles.Get(r.Context(), userID); err == nil && profile != nil {
		design = agent.DesignInput{
			TargetRole:      profile.TargetRole,
			YearsExperience: profile.YearsExperience,
			Seniority:       profile.Seniority,
			CurrentRole:     profile.CurrentRole,
			TechStack:       profile.TechStack,
			Goals:           profile.Goals,
			ResumeText:      profile.ResumeText,
		}
	}
	design.JobDescription = iv.JobDescription

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	next, err := h.LiveInterviewer.NextQuestion(ctx, agent.NextQuestionInput{
		Profile:                    design,
		History:                    history,
		TimeRemainingSec:           timeRemaining,
		TotalDurationSec:           iv.DurationSeconds,
		IsFirst:                    false,
		IntentsCovered:             intentsCovered,
		AvgScoreSoFar:              avgScore,
		AvgTurnSec:                 avgTurn,
		ExpectedRemainingQuestions: expectedRemaining,
	})
	if err != nil {
		response.Err(w, http.StatusBadGateway, "next question generation failed: "+err.Error())
		return
	}

	// Defensive: keep the persisted intent label consistent with is_follow_up
	// so the next turn's coverage map and reviewer agent see the same shape.
	if next.IsFollowUp {
		next.Intent = "follow_up"
	}

	// Log the agent's choice for audit. Useful when the agent ignores the policy.
	log.Printf("live next_question interview_id=%s intent=%s is_follow_up=%t wrap=%t time_remaining=%d expected_remaining=%d avg_score=%s rationale=%q",
		interviewID, next.Intent, next.IsFollowUp, next.ShouldWrap, timeRemaining, expectedRemaining,
		fmtAvgScore(avgScore), trimRationale(next.FollowUpRationale))

	// Hard wrap floor — the agent owns most wrap logic now; keep a small
	// safety floor so we never start a brand-new question with seconds left.
	if next.ShouldWrap || timeRemaining < 30 {
		response.OK(w, http.StatusOK, nextQuestionResponse{
			Wrap:             true,
			TimeRemainingSec: timeRemaining,
		})
		return
	}

	q, err := h.persistLiveQuestion(r.Context(), userID, next)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to persist generated question")
		return
	}
	if err := h.Interviews.AppendQuestion(r.Context(), interviewID, q.ID, len(questions)); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to attach question to interview")
		return
	}

	response.OK(w, http.StatusOK, nextQuestionResponse{
		Question:         q,
		Wrap:             false,
		TimeRemainingSec: timeRemaining,
	})
}

// Get returns one interview owned by the caller with its questions and submissions.
func (h *InterviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	iv, err := h.Interviews.Get(r.Context(), id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "interview not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load interview")
		return
	}
	if qs, err := h.Interviews.QuestionsFor(r.Context(), id); err == nil {
		iv.Questions = qs
	}
	if subs, err := h.Submissions.ListForInterview(r.Context(), id, userID); err == nil {
		iv.Submissions = subs
	}
	response.OK(w, http.StatusOK, iv)
}

// ListMine returns the caller's interviews.
func (h *InterviewHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	out, err := h.Interviews.ListForUser(r.Context(), userID, 25)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to list interviews")
		return
	}
	response.OK(w, http.StatusOK, out)
}

// SubmitAnswer accepts an audio recording for a specific question inside an
// interview. The submission is tagged with both the question and interview ids.
func (h *InterviewHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	interviewID := chi.URLParam(r, "id")
	questionID := chi.URLParam(r, "qid")

	iv, err := h.Interviews.Get(r.Context(), interviewID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "interview not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load interview")
		return
	}
	if iv.Status != "in_progress" {
		response.Err(w, http.StatusConflict, "interview is no longer in progress")
		return
	}

	question, err := h.Questions.Get(r.Context(), questionID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load question")
		return
	}

	file, mimeType, err := readAudioPart(w, r, h.MaxAudioBytes)
	if err != nil {
		response.Err(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()
	clientTranscript := readClientTranscript(r)

	sub, err := h.Submitter.Submit(r.Context(), submissions.SubmitInput{
		UserID:      userID,
		Question:    question,
		InterviewID: &interviewID,
		Audio:       file,
		MimeType:    mimeType,
		Transcript:  clientTranscript,
	})
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to create submission")
		return
	}
	response.OK(w, http.StatusAccepted, sub)
}

// Complete aggregates all submissions for the interview into a final score +
// summary via the aggregator agent, then marks the interview completed.
// If submissions are still being reviewed it returns 409 so the client retries.
func (h *InterviewHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.Err(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")

	iv, err := h.Interviews.Get(r.Context(), id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.Err(w, http.StatusNotFound, "interview not found")
		return
	}
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load interview")
		return
	}

	subs, err := h.Submissions.ListForInterview(r.Context(), id, userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load submissions")
		return
	}

	for _, s := range subs {
		if s.Status != "complete" && s.Status != "failed" {
			response.Err(w, http.StatusConflict, "submissions are still being reviewed; try again in a moment")
			return
		}
	}
	if len(subs) == 0 {
		response.Err(w, http.StatusBadRequest, "no answers were submitted for this interview")
		return
	}

	if iv.Status == "completed" {
		if qs, err := h.Interviews.QuestionsFor(r.Context(), id); err == nil {
			iv.Questions = qs
		}
		iv.Submissions = subs
		response.OK(w, http.StatusOK, iv)
		return
	}

	questions, err := h.Interviews.QuestionsFor(r.Context(), id)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to load interview questions")
		return
	}

	subByQ := map[string]int{}
	for i, s := range subs {
		subByQ[s.QuestionID] = i
	}
	items := make([]agent.AggregateInput, 0, len(questions))
	for _, q := range questions {
		idx, ok := subByQ[q.ID]
		if !ok {
			continue
		}
		s := subs[idx]
		score := 0.0
		if s.Score != nil {
			score = *s.Score
		}
		items = append(items, agent.AggregateInput{
			QuestionTitle: q.Title,
			Difficulty:    q.Difficulty,
			Score:         score,
			Strengths:     s.Strengths,
			Improvements:  s.Improvements,
			Feedback:      s.Feedback,
		})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	result, err := h.Aggregator.Aggregate(ctx, items)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "aggregation failed: "+err.Error())
		return
	}

	if err := h.Interviews.Complete(r.Context(), id, result.Score, result.Summary); err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to mark interview completed")
		return
	}

	iv, err = h.Interviews.Get(r.Context(), id, userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "failed to reload interview")
		return
	}
	iv.Questions = questions
	iv.Submissions = subs
	response.OK(w, http.StatusOK, iv)
}

func fmtAvgScore(s *float64) string {
	if s == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.0f", *s)
}

func trimRationale(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}
