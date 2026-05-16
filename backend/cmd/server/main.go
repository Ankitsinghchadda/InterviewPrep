package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/auth"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/config"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/database"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/routes"
	rzp "github.com/Ankitsinghchadda/InterviewPrep/internal/services/billing"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/audio"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/embeddings"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/stt"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/submissions"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/tts"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	repo := repository.New(db)

	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, "interviewprep")
	authSvc := auth.NewService(tm, repo.Users, repo.Tokens, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	cookies := auth.CookieConfig{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure}

	storage, err := audio.NewLocalFS(cfg.AudioLocalDir)
	if err != nil {
		log.Fatalf("audio storage: %v", err)
	}

	ai := buildAIServices(cfg)

	billingSvc := &billing.Service{Repo: repo.Usage}
	razorpaySvc := rzp.New(cfg.RazorpayKeyID, cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret)
	if razorpaySvc == nil {
		log.Println("BILLING: Razorpay disabled (RAZORPAY_KEY_ID unset); /billing/checkout returns 503")
	} else {
		log.Printf("BILLING: Razorpay enabled (key=%s)", cfg.RazorpayKeyID)
	}

	broker := submissions.NewBroker()
	submitter := &submissions.Service{
		Submissions: repo.Submissions,
		Storage:     storage,
		Transcriber: ai.transcriber,
		Reviewer:    ai.reviewer,
		Broker:      broker,
	}

	router := routes.New(routes.Deps{
		Config:          cfg,
		Repo:            repo,
		AuthService:     authSvc,
		TokenManager:    tm,
		Cookies:         cookies,
		Submitter:       submitter,
		Broker:          broker,
		Aggregator:      ai.aggregator,
		Designer:        ai.designer,
		LiveInterviewer: ai.liveInterviewer,
		Extractor:       ai.extractor,
		Explainer:       ai.explainer,
		AnswerGen:       ai.answerGen,
		QuestionGen:     ai.questionGen,
		Synth:           ai.synth,
		Embedder:        ai.embedder,
		Billing:         billingSvc,
		Razorpay:        razorpaySvc,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s (env=%s, agent=%v)", cfg.Port, cfg.Env, cfg.AgentEnabled)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}

type aiBundle struct {
	transcriber     stt.Transcriber
	reviewer        agent.Reviewer
	aggregator      agent.Aggregator
	designer        agent.InterviewDesigner
	liveInterviewer agent.LiveInterviewer
	extractor       agent.ProfileExtractor
	explainer       agent.Explainer
	answerGen       agent.AnswerGenerator
	questionGen     agent.QuestionGenerator
	synth           tts.Synthesizer
	embedder        embeddings.Embedder
}

// buildAIServices wires every AI-backed service. Two parallel sets of agents
// are constructed: a "free" set on Vertex AI (flash model) and a "paid" set
// on the Gemini API (premium model). Each pair is wrapped in a router that
// dispatches per-request based on the caller's plan.
//
// Stubs are used when AGENT_ENABLED=false so the full UX still works in local
// dev without GCP. When GEMINI_API_KEY is unset, the paid set is left nil and
// Pro users transparently fall back to the free Vertex agent.
func buildAIServices(cfg *config.Config) aiBundle {
	stubs := aiBundle{
		transcriber:     stt.Stub{},
		reviewer:        &agent.ReviewerRouter{Free: agent.Stub{}},
		aggregator:      &agent.AggregatorRouter{Free: agent.StubAggregator{}},
		designer:        &agent.InterviewDesignerRouter{Free: agent.StubInterviewDesigner{}},
		liveInterviewer: &agent.LiveInterviewerRouter{Free: agent.StubLiveInterviewer{}},
		extractor:       &agent.ProfileExtractorRouter{Free: agent.StubProfileExtractor{}},
		explainer:       &agent.ExplainerRouter{Free: agent.StubExplainer{}},
		answerGen:       &agent.AnswerGeneratorRouter{Free: agent.StubAnswerGenerator{}},
		questionGen:     &agent.QuestionGeneratorRouter{Free: agent.StubQuestionGenerator{}},
		synth:           tts.Stub{},
		embedder:        embeddings.Stub{}, // deterministic fallback so dedup UX works without GCP
	}
	if !cfg.AgentEnabled {
		log.Println("AI: using stubs for reviewer/aggregator/designer/live/extractor + STT (set AGENT_ENABLED=true to enable Vertex AI)")
		return stubs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hasPaid := cfg.GeminiAPIKey != ""
	if !hasPaid {
		log.Println("AI: GEMINI_API_KEY unset; Pro users will fall back to Vertex/flash")
	}

	// --- non-LLM services (Google STT/TTS/embeddings) ------------------------
	out := stubs
	if g, err := stt.NewGoogle(ctx, cfg.STTLanguage); err == nil {
		out.transcriber = g
	} else {
		log.Printf("AI: Google STT init failed: %v (using stub)", err)
	}
	if cfg.AudioBucket != "" {
		if g, err := tts.NewGoogle(ctx, cfg.AudioBucket, cfg.TTSVoice, cfg.TTSPromptVoice); err == nil {
			out.synth = g
		} else {
			log.Printf("AI: Google TTS init failed: %v (audio synthesis disabled)", err)
		}
	} else {
		log.Println("AI: AUDIO_BUCKET unset; reference-answer audio synthesis disabled")
	}
	if em, err := embeddings.New(ctx, cfg.GCPProject, cfg.GCPLocation, embeddings.DefaultModel); err == nil {
		out.embedder = em
	} else {
		log.Printf("AI: Vertex embeddings init failed: %v (falling back to stub embedder)", err)
	}

	// --- agent routers (one per kind, wrapping free+paid impls) --------------
	rev := &agent.ReviewerRouter{Free: agent.Stub{}}
	if v, err := agent.NewReviewer(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		rev.Free = v
	} else {
		log.Printf("AI: Vertex reviewer init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if v, err := agent.NewReviewer(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			rev.Paid = v
		} else {
			log.Printf("AI: Gemini API reviewer init failed: %v (Pro will fall back to Vertex)", err)
		}
	}
	out.reviewer = rev

	agg := &agent.AggregatorRouter{Free: agent.StubAggregator{}}
	if a, err := agent.NewAggregator(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		agg.Free = a
	} else {
		log.Printf("AI: Vertex aggregator init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if a, err := agent.NewAggregator(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			agg.Paid = a
		} else {
			log.Printf("AI: Gemini API aggregator init failed: %v", err)
		}
	}
	out.aggregator = agg

	des := &agent.InterviewDesignerRouter{Free: agent.StubInterviewDesigner{}}
	if d, err := agent.NewInterviewDesigner(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		des.Free = d
	} else {
		log.Printf("AI: Vertex designer init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if d, err := agent.NewInterviewDesigner(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			des.Paid = d
		} else {
			log.Printf("AI: Gemini API designer init failed: %v", err)
		}
	}
	out.designer = des

	live := &agent.LiveInterviewerRouter{Free: agent.StubLiveInterviewer{}}
	if l, err := agent.NewLiveInterviewer(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		live.Free = l
	} else {
		log.Printf("AI: Vertex live interviewer init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if l, err := agent.NewLiveInterviewer(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			live.Paid = l
		} else {
			log.Printf("AI: Gemini API live interviewer init failed: %v", err)
		}
	}
	out.liveInterviewer = live

	ext := &agent.ProfileExtractorRouter{Free: agent.StubProfileExtractor{}}
	if e, err := agent.NewProfileExtractor(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		ext.Free = e
	} else {
		log.Printf("AI: Vertex extractor init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if e, err := agent.NewProfileExtractor(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			ext.Paid = e
		} else {
			log.Printf("AI: Gemini API extractor init failed: %v", err)
		}
	}
	out.extractor = ext

	exp := &agent.ExplainerRouter{Free: agent.StubExplainer{}}
	if x, err := agent.NewExplainer(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		exp.Free = x
	} else {
		log.Printf("AI: Vertex explainer init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if x, err := agent.NewExplainer(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			exp.Paid = x
		} else {
			log.Printf("AI: Gemini API explainer init failed: %v", err)
		}
	}
	out.explainer = exp

	ag := &agent.AnswerGeneratorRouter{Free: agent.StubAnswerGenerator{}}
	if g, err := agent.NewAnswerGenerator(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		ag.Free = g
	} else {
		log.Printf("AI: Vertex answer generator init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if g, err := agent.NewAnswerGenerator(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			ag.Paid = g
		} else {
			log.Printf("AI: Gemini API answer generator init failed: %v", err)
		}
	}
	out.answerGen = ag

	qg := &agent.QuestionGeneratorRouter{Free: agent.StubQuestionGenerator{}}
	if g, err := agent.NewQuestionGenerator(ctx, agent.BackendVertex, cfg.AgentModel, cfg.GCPProject, cfg.GCPLocation, ""); err == nil {
		qg.Free = g
	} else {
		log.Printf("AI: Vertex question generator init failed: %v (using stub for free)", err)
	}
	if hasPaid {
		if g, err := agent.NewQuestionGenerator(ctx, agent.BackendGeminiAPI, cfg.GeminiPremiumModel, "", "", cfg.GeminiAPIKey); err == nil {
			qg.Paid = g
		} else {
			log.Printf("AI: Gemini API question generator init failed: %v", err)
		}
	}
	out.questionGen = qg

	if hasPaid {
		log.Printf("AI: free=Vertex %s/%s model=%s, paid=GeminiAPI model=%s, STT lang=%s",
			cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel, cfg.GeminiPremiumModel, cfg.STTLanguage)
	} else {
		log.Printf("AI: Vertex-only %s/%s, model=%s, STT lang=%s",
			cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel, cfg.STTLanguage)
	}
	return out
}
