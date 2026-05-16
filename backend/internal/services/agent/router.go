package agent

import (
	"context"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
)

// Each *Router type below holds a Free and Paid implementation of one agent
// interface and dispatches at call time based on the plan stamped onto the
// request context. The router itself implements the agent interface so
// handlers continue calling the same methods they always did — they don't
// need to know which backend served the request.
//
// Paid may be nil; in that case Pro users transparently fall back to the
// Free agent so the system stays operable before GEMINI_API_KEY is set.

func isPro(ctx context.Context) bool {
	return auth.PlanFromContext(ctx) == auth.PlanPro
}

// --- Reviewer ----------------------------------------------------------------

type ReviewerRouter struct {
	Free Reviewer
	Paid Reviewer
}

func (r *ReviewerRouter) pick(ctx context.Context) Reviewer {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *ReviewerRouter) Review(ctx context.Context, in ReviewInput) (*ReviewResult, error) {
	return r.pick(ctx).Review(ctx, in)
}

func (r *ReviewerRouter) ReviewStream(ctx context.Context, in ReviewInput, onToken func(string)) (*ReviewResult, error) {
	return r.pick(ctx).ReviewStream(ctx, in, onToken)
}

// --- Aggregator --------------------------------------------------------------

type AggregatorRouter struct {
	Free Aggregator
	Paid Aggregator
}

func (r *AggregatorRouter) pick(ctx context.Context) Aggregator {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *AggregatorRouter) Aggregate(ctx context.Context, items []AggregateInput) (*AggregateResult, error) {
	return r.pick(ctx).Aggregate(ctx, items)
}

// --- InterviewDesigner -------------------------------------------------------

type InterviewDesignerRouter struct {
	Free InterviewDesigner
	Paid InterviewDesigner
}

func (r *InterviewDesignerRouter) pick(ctx context.Context) InterviewDesigner {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *InterviewDesignerRouter) Design(ctx context.Context, in DesignInput) (*DesignedInterview, error) {
	return r.pick(ctx).Design(ctx, in)
}

// --- LiveInterviewer ---------------------------------------------------------

type LiveInterviewerRouter struct {
	Free LiveInterviewer
	Paid LiveInterviewer
}

func (r *LiveInterviewerRouter) pick(ctx context.Context) LiveInterviewer {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *LiveInterviewerRouter) NextQuestion(ctx context.Context, in NextQuestionInput) (*NextQuestion, error) {
	return r.pick(ctx).NextQuestion(ctx, in)
}

// --- ProfileExtractor --------------------------------------------------------

type ProfileExtractorRouter struct {
	Free ProfileExtractor
	Paid ProfileExtractor
}

func (r *ProfileExtractorRouter) pick(ctx context.Context) ProfileExtractor {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *ProfileExtractorRouter) Extract(ctx context.Context, fileBytes []byte, mimeType, filename string) (*ExtractedProfile, error) {
	return r.pick(ctx).Extract(ctx, fileBytes, mimeType, filename)
}

// --- Explainer ---------------------------------------------------------------

type ExplainerRouter struct {
	Free Explainer
	Paid Explainer
}

func (r *ExplainerRouter) pick(ctx context.Context) Explainer {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *ExplainerRouter) Explain(ctx context.Context, in ExplainInput) (*ExplainResult, error) {
	return r.pick(ctx).Explain(ctx, in)
}

// --- AnswerGenerator ---------------------------------------------------------

type AnswerGeneratorRouter struct {
	Free AnswerGenerator
	Paid AnswerGenerator
}

func (r *AnswerGeneratorRouter) pick(ctx context.Context) AnswerGenerator {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *AnswerGeneratorRouter) Generate(ctx context.Context, in GenerateAnswerInput) (string, error) {
	return r.pick(ctx).Generate(ctx, in)
}

// --- QuestionGenerator -------------------------------------------------------

type QuestionGeneratorRouter struct {
	Free QuestionGenerator
	Paid QuestionGenerator
}

func (r *QuestionGeneratorRouter) pick(ctx context.Context) QuestionGenerator {
	if isPro(ctx) && r.Paid != nil {
		return r.Paid
	}
	return r.Free
}

func (r *QuestionGeneratorRouter) Generate(ctx context.Context, in GenerateInput) (*DesignedInterview, error) {
	return r.pick(ctx).Generate(ctx, in)
}
