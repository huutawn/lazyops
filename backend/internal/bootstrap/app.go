package bootstrap

import (
	"lazyops-server/internal/ai"
	"lazyops-server/internal/config"
	"lazyops-server/internal/hub"
	"lazyops-server/internal/repository"
	"lazyops-server/internal/service"

	"gorm.io/gorm"
)

type Application struct {
	Config       config.Config
	DB           *gorm.DB
	Hub          *hub.Hub
	AI           *ai.GeminiClient
	UserRepo     *repository.UserRepository
	AgentRepo    *repository.AgentRepository
	AuthService  *service.AuthService
	UserService  *service.UserService
	AgentService *service.AgentService
}

func NewApplication(cfg config.Config) (*Application, error) {
	db, err := NewDatabase(cfg)
	if err != nil {
		return nil, err
	}

	if err := Migrate(db); err != nil {
		return nil, err
	}
	if err := SeedAdmin(db, cfg); err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	authService := service.NewAuthService(userRepo, cfg.JWT)
	userService := service.NewUserService(userRepo)
	agentService := service.NewAgentService(agentRepo)
	wsHub := hub.New()
	wsHub.Start()

	return &Application{
		Config:       cfg,
		DB:           db,
		Hub:          wsHub,
		AI:           ai.NewGeminiClient(""),
		UserRepo:     userRepo,
		AgentRepo:    agentRepo,
		AuthService:  authService,
		UserService:  userService,
		AgentService: agentService,
	}, nil
}
