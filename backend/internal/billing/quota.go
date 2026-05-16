// Package billing enforces per-user quotas on AI calls and exposes the
// plan/usage state used by the paywall UI.
//
// The unit of accounting is the Kind — one string per billable action
// (recording_review, mock_basic, question_add, ...). Each handler that
// gates one of these actions calls Service.Check before the AI call and
// Service.Record after a successful AI call.
//
// Window is rolling: a count is the number of events in the user's last
// 7 days, not since a calendar-week reset. See repository/usage.go for the
// SQL.
package billing

import "time"

// Kind names a billable action. Add a new constant when introducing a
// new gated feature, then wire it into FreeQuotas (and the handler).
type Kind string

const (
	KindRecordingReview Kind = "recording_review" // capped: AI grading of a recorded answer
	KindMockBasic       Kind = "mock_basic"       // capped: topic/adaptive interview start
	KindMockLive        Kind = "mock_live"        // paid-only: live interview start
	KindQuestionAdd     Kind = "question_add"     // capped: user creates a question (auto-answer rolls in)
	KindQuestionGen     Kind = "question_gen"     // paid-only: batch generate questions for a category
	KindAnswerGen       Kind = "answer_gen"       // capped: standalone reference-answer drafting
	KindExplanation     Kind = "explanation"     // capped: AI explanation of a question
	KindTTS             Kind = "tts"             // capped: TTS audio for reference answer
)

// AllKinds lists every kind so the /usage endpoint can return a complete
// row set even for kinds the user hasn't touched this week.
var AllKinds = []Kind{
	KindRecordingReview,
	KindMockBasic,
	KindMockLive,
	KindQuestionAdd,
	KindQuestionGen,
	KindAnswerGen,
	KindExplanation,
	KindTTS,
}

// Week is the rolling window used by every quota in this codebase. Kept
// as a single constant so changing "weekly" to "monthly" is one edit.
const Week = 7 * 24 * time.Hour

// Quota is one row in a plan's quota table.
type Quota struct {
	Window time.Duration // rolling window length
	Limit  int           // -1 unlimited; 0 blocked entirely; >0 = max events allowed in Window
}

const (
	LimitUnlimited = -1
	LimitBlocked   = 0
)

// FreeQuotas captures the limits described in the product spec:
//   - 3 AI reviews of recorded answers per week
//   - 2 basic mock interviews per week
//   - Live interviews are paid-only
//   - 5 user-added questions per week (the auto-answer rolls into this budget)
//   - Batch question generation is paid-only
//   - Soft caps on minor AI features (10/week) so abuse doesn't burn budget
var FreeQuotas = map[Kind]Quota{
	KindRecordingReview: {Window: Week, Limit: 3},
	KindMockBasic:       {Window: Week, Limit: 2},
	KindMockLive:        {Window: Week, Limit: LimitBlocked},
	KindQuestionAdd:     {Window: Week, Limit: 5},
	KindQuestionGen:     {Window: Week, Limit: LimitBlocked},
	KindAnswerGen:       {Window: Week, Limit: 5},
	KindExplanation:     {Window: Week, Limit: 10},
	KindTTS:             {Window: Week, Limit: 10},
}

// ProQuotas: everything unlimited. Kept as a map (rather than a "pro=skip
// check" shortcut) so future tiers (team, enterprise) slot in cleanly.
var ProQuotas = map[Kind]Quota{
	KindRecordingReview: {Window: Week, Limit: LimitUnlimited},
	KindMockBasic:       {Window: Week, Limit: LimitUnlimited},
	KindMockLive:        {Window: Week, Limit: LimitUnlimited},
	KindQuestionAdd:     {Window: Week, Limit: LimitUnlimited},
	KindQuestionGen:     {Window: Week, Limit: LimitUnlimited},
	KindAnswerGen:       {Window: Week, Limit: LimitUnlimited},
	KindExplanation:     {Window: Week, Limit: LimitUnlimited},
	KindTTS:             {Window: Week, Limit: LimitUnlimited},
}

// QuotaFor returns the row for (plan, kind). Falls back to a permissive
// default if a new kind hasn't been registered yet, so an unconfigured
// feature ships open rather than silently blocked.
func QuotaFor(plan string, k Kind) Quota {
	src := FreeQuotas
	if plan == "pro" {
		src = ProQuotas
	}
	if q, ok := src[k]; ok {
		return q
	}
	return Quota{Window: Week, Limit: LimitUnlimited}
}
