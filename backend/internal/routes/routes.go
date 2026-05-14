package routes

import (
	"net/http"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/config"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/handlers"
	appmw "github.com/Ankitsinghchadda/InterviewPrep/internal/middleware"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/submissions"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	Config       *config.Config
	Repo         *repository.Repository
	AuthService  *auth.Service
	TokenManager *auth.TokenManager
	Cookies      auth.CookieConfig

	Submitter       *submissions.Service
	Broker          *submissions.Broker
	Aggregator      agent.Aggregator
	Designer        agent.InterviewDesigner
	LiveInterviewer agent.LiveInterviewer
	Extractor       agent.ProfileExtractor
	Synth           tts.Synthesizer
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(appmw.Logger)
	r.Use(appmw.CORS(d.Config.AllowOrigin))

	r.Get("/health", handlers.Health)

	authH := &handlers.AuthHandler{
		Service:     d.AuthService,
		Users:       d.Repo.Users,
		Cookies:     d.Cookies,
		FrontendURL: d.Config.FrontendURL,
		DefaultPath: d.Config.PostLoginRedirect,
	}
	categoryH := &handlers.CategoryHandler{Repo: d.Repo.Categories}
	questionH := &handlers.QuestionHandler{
		Repo:          d.Repo.Questions,
		Profiles:      d.Repo.Profiles,
		Submitter:     d.Submitter,
		Synth:         d.Synth,
		MaxAudioBytes: d.Config.MaxAudioBytes,
	}
	audioH := &handlers.AudioHandler{
		Questions: d.Repo.Questions,
		Synth:     d.Synth,
	}
	interviewH := &handlers.InterviewHandler{
		Interviews:      d.Repo.Interviews,
		Questions:       d.Repo.Questions,
		Categories:      d.Repo.Categories,
		Submissions:     d.Repo.Submissions,
		Profiles:        d.Repo.Profiles,
		Submitter:       d.Submitter,
		Aggregator:      d.Aggregator,
		Designer:        d.Designer,
		LiveInterviewer: d.LiveInterviewer,
		Synth:           d.Synth,
		MaxAudioBytes:   d.Config.MaxAudioBytes,
	}
	submissionH := &handlers.SubmissionHandler{
		Repo:   d.Repo.Submissions,
		Broker: d.Broker,
	}
	profileH := &handlers.ProfileHandler{
		Profiles:       d.Repo.Profiles,
		Extractor:      d.Extractor,
		MaxResumeBytes: d.Config.MaxResumeBytes,
	}

	requireAuth := auth.Authenticator(d.TokenManager)

	r.Route("/auth", func(r chi.Router) {
		r.Get("/google/login", authH.Login)
		r.Get("/google/callback", authH.Callback)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)
		r.With(requireAuth).Get("/me", authH.Me)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(requireAuth)

		r.Route("/profile", func(r chi.Router) {
			r.Get("/", profileH.Get)
			r.Put("/", profileH.Upsert)
			r.Post("/resume", profileH.UploadResume)
		})

		r.Route("/categories", func(r chi.Router) {
			r.Get("/", categoryH.List)
			r.Get("/{slug}", categoryH.GetBySlug)
		})

		r.Route("/questions", func(r chi.Router) {
			r.Get("/", questionH.List)
			r.Post("/", questionH.Create)
			r.Get("/recommended", questionH.Recommended)
			r.Get("/{id}", questionH.Get)
			r.Delete("/{id}", questionH.Delete)
			r.Post("/{id}/answer", questionH.SubmitAnswer)
			r.Post("/{id}/audio", audioH.Generate)
		})

		r.Route("/submissions", func(r chi.Router) {
			r.Get("/", submissionH.ListMine)
			r.Get("/{id}", submissionH.Get)
			r.Get("/{id}/stream", submissionH.Stream)
		})

		r.Route("/interviews", func(r chi.Router) {
			r.Post("/", interviewH.Start)
			r.Get("/", interviewH.ListMine)
			r.Get("/{id}", interviewH.Get)
			r.Post("/{id}/questions/{qid}/answer", interviewH.SubmitAnswer)
			r.Post("/{id}/next-question", interviewH.NextQuestion)
			r.Post("/{id}/complete", interviewH.Complete)
		})
	})

	return r
}
