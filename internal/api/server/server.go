package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"sixtyseven/internal/api/handlers"
	"sixtyseven/internal/api/middleware"
	"sixtyseven/internal/api/websocket"
	"sixtyseven/internal/config"
	"sixtyseven/internal/repository/postgres"
	"sixtyseven/internal/service"
)

// Server represents the HTTP server
type Server struct {
	cfg    *config.Config
	router *chi.Mux
	server *http.Server

	// Dependencies
	db   *postgres.DB
	repos *postgres.Repositories

	// Services
	authService    *service.AuthService
	teamService    *service.TeamService
	appService     *service.AppService
	runService     *service.RunService
	metricsService *service.MetricsService

	// WebSocket
	wsHub *websocket.Hub

	// Handlers
	authHandler    *handlers.AuthHandler
	teamHandler    *handlers.TeamHandler
	appHandler     *handlers.AppHandler
	runHandler     *handlers.RunHandler
	metricsHandler *handlers.MetricsHandler
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	// Connect to database
	db, err := postgres.New(context.Background(), postgres.DefaultConfig(cfg.DatabaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create repositories
	repos := postgres.NewRepositories(db)

	// Create WebSocket hub
	wsHub := websocket.NewHub()

	// Create services
	authService := service.NewAuthService(cfg, repos.Users, repos.APIKeys, repos.Sessions, repos.Teams)
	teamService := service.NewTeamService(repos.Teams, repos.Users)
	appService := service.NewAppService(repos.Apps, repos.Teams)
	runService := service.NewRunService(repos.Runs, repos.Apps, repos.Metrics, repos.Continuations)
	metricsService := service.NewMetricsService(cfg, repos.Metrics, wsHub)

	// Create handlers
	authHandler := handlers.NewAuthHandler(authService)
	teamHandler := handlers.NewTeamHandler(teamService)
	appHandler := handlers.NewAppHandler(appService, teamService)
	runHandler := handlers.NewRunHandler(runService, appService, teamService)
	metricsHandler := handlers.NewMetricsHandler(metricsService, runService)

	s := &Server{
		cfg:            cfg,
		db:             db,
		repos:          repos,
		authService:    authService,
		teamService:    teamService,
		appService:     appService,
		runService:     runService,
		metricsService: metricsService,
		wsHub:          wsHub,
		authHandler:    authHandler,
		teamHandler:    teamHandler,
		appHandler:     appHandler,
		runHandler:     runHandler,
		metricsHandler: metricsHandler,
	}

	// Seed admin user if no users exist (only in self-hosted mode)
	if cfg.DeploymentMode == config.DeploymentModeSelfHosted {
		if err := authService.SeedAdmin(context.Background()); err != nil {
			log.Printf("Warning: Failed to seed admin user: %v", err)
		} else {
			log.Println("Admin user check complete")
		}
	}

	s.setupRouter()
	return s, nil
}

// setupRouter configures all routes
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", s.healthCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Group(func(r chi.Router) {
			r.Post("/auth/register", s.authHandler.Register)
			r.Post("/auth/login", s.authHandler.Login)
		})

		// Protected routes (JWT or API Key)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(s.authService))

			// Auth
			r.Get("/auth/me", s.authHandler.Me)
			r.Post("/auth/logout", s.authHandler.Logout)
			r.Post("/auth/api-keys", s.authHandler.CreateAPIKey)
			r.Get("/auth/api-keys", s.authHandler.ListAPIKeys)
			r.Delete("/auth/api-keys/{keyID}", s.authHandler.DeleteAPIKey)

			// Teams
			r.Get("/teams", s.teamHandler.List)
			r.Post("/teams", s.teamHandler.Create)
			r.Get("/teams/{teamSlug}", s.teamHandler.Get)
			r.Put("/teams/{teamSlug}", s.teamHandler.Update)
			r.Delete("/teams/{teamSlug}", s.teamHandler.Delete)

			// Team members
			r.Get("/teams/{teamSlug}/members", s.teamHandler.ListMembers)
			r.Post("/teams/{teamSlug}/members", s.teamHandler.AddMember)
			r.Put("/teams/{teamSlug}/members/{userID}", s.teamHandler.UpdateMember)
			r.Delete("/teams/{teamSlug}/members/{userID}", s.teamHandler.RemoveMember)

			// Apps
			r.Get("/teams/{teamSlug}/apps", s.appHandler.List)
			r.Post("/teams/{teamSlug}/apps", s.appHandler.Create)
			r.Get("/teams/{teamSlug}/apps/{appSlug}", s.appHandler.Get)
			r.Put("/teams/{teamSlug}/apps/{appSlug}", s.appHandler.Update)
			r.Delete("/teams/{teamSlug}/apps/{appSlug}", s.appHandler.Delete)

			// Runs
			r.Get("/teams/{teamSlug}/apps/{appSlug}/runs", s.runHandler.List)
			r.Post("/teams/{teamSlug}/apps/{appSlug}/runs", s.runHandler.Create)
			r.Get("/teams/{teamSlug}/apps/{appSlug}/runs/{runID}", s.runHandler.Get)
			r.Put("/teams/{teamSlug}/apps/{appSlug}/runs/{runID}", s.runHandler.Update)
			r.Delete("/teams/{teamSlug}/apps/{appSlug}/runs/{runID}", s.runHandler.Delete)

			// Run operations (by run ID directly for SDK convenience)
			r.Get("/runs/{runID}", s.runHandler.GetByID)
			r.Put("/runs/{runID}/status", s.runHandler.UpdateStatus)
			r.Put("/runs/{runID}/config", s.runHandler.UpdateConfig)
			r.Post("/runs/{runID}/resume", s.runHandler.ResumeRun)
			r.Get("/runs/{runID}/continuations", s.runHandler.GetContinuations)

			// Metrics
			r.Post("/runs/{runID}/metrics", s.metricsHandler.BatchLog)
			r.Get("/runs/{runID}/metrics", s.metricsHandler.List)
			r.Get("/runs/{runID}/metrics/latest", s.metricsHandler.GetLatest)
			r.Get("/runs/{runID}/metrics/summary", s.metricsHandler.GetSummary)
			r.Get("/runs/{runID}/metrics/{metricName}", s.metricsHandler.GetSeries)
		})

		// WebSocket routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(s.authService))
			r.Get("/ws/runs/{runID}/metrics", s.wsHub.HandleConnection)
		})
	})

	s.router = r
}

// healthCheck handles health check requests
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Health(r.Context()); err != nil {
		http.Error(w, "database unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start WebSocket hub
	go s.wsHub.Run()

	s.server = &http.Server{
		Addr:         s.cfg.Address(),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting server on %s", s.cfg.Address())
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	// Stop metrics service (flushes pending metrics)
	s.metricsService.Stop()

	// Close WebSocket hub
	s.wsHub.Close()

	// Shutdown HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	// Close database
	s.db.Close()

	return nil
}
