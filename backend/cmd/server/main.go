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
	"github.com/Ankitsinghchadda/InterviewPrep/internal/config"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/database"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/routes"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/agent"
	"github.com/Ankitsinghchadda/InterviewPrep/internal/services/audio"
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
		Synth:           ai.synth,
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
	synth           tts.Synthesizer
}

// buildAIServices wires every AI-backed service. Stubs are used when GCP isn't
// configured so the full UX still works in local dev.
func buildAIServices(cfg *config.Config) aiBundle {
	stubs := aiBundle{
		transcriber:     stt.Stub{},
		reviewer:        agent.Stub{},
		aggregator:      agent.StubAggregator{},
		designer:        agent.StubInterviewDesigner{},
		liveInterviewer: agent.StubLiveInterviewer{},
		extractor:       agent.StubProfileExtractor{},
		synth:           tts.Stub{},
	}
	if !cfg.AgentEnabled {
		log.Println("AI: using stubs for reviewer/aggregator/designer/live/extractor + STT (set AGENT_ENABLED=true to enable Vertex AI)")
		return stubs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out := stubs
	if g, err := stt.NewGoogle(ctx, cfg.STTLanguage); err == nil {
		out.transcriber = g
	} else {
		log.Printf("AI: Google STT init failed: %v (using stub)", err)
	}
	if v, err := agent.NewVertex(ctx, cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel); err == nil {
		out.reviewer = v
	} else {
		log.Printf("AI: Vertex reviewer init failed: %v (using stub)", err)
	}
	if a, err := agent.NewVertexAggregator(ctx, cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel); err == nil {
		out.aggregator = a
	} else {
		log.Printf("AI: Vertex aggregator init failed: %v (using stub)", err)
	}
	if d, err := agent.NewVertexInterviewDesigner(ctx, cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel); err == nil {
		out.designer = d
	} else {
		log.Printf("AI: Vertex designer init failed: %v (using stub)", err)
	}
	if l, err := agent.NewVertexLiveInterviewer(ctx, cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel); err == nil {
		out.liveInterviewer = l
	} else {
		log.Printf("AI: Vertex live interviewer init failed: %v (using stub)", err)
	}
	if cfg.AudioBucket != "" {
		if g, err := tts.NewGoogle(ctx, cfg.AudioBucket, cfg.TTSVoice); err == nil {
			out.synth = g
		} else {
			log.Printf("AI: Google TTS init failed: %v (audio synthesis disabled)", err)
		}
	} else {
		log.Println("AI: AUDIO_BUCKET unset; reference-answer audio synthesis disabled")
	}
	if e, err := agent.NewVertexProfileExtractor(ctx, cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel); err == nil {
		out.extractor = e
	} else {
		log.Printf("AI: Vertex extractor init failed: %v (using stub)", err)
	}

	log.Printf("AI: Vertex %s/%s, model=%s, STT lang=%s", cfg.GCPProject, cfg.GCPLocation, cfg.AgentModel, cfg.STTLanguage)
	return out
}
