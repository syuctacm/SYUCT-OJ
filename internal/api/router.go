package api

import (
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/api/middleware"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/mail"
	"github.com/syuctacm/syuct-oj/internal/message"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func NewRouter(cfg config.Config, db *mongo.Database, judgeHandler *judge.Handler, authHandler *auth.Handler, problemRepo *problem.Repository, submissionHandler *handler.SubmissionHandler, contestRepo *contest.Repository, trainingRepo *training.Repository, discussionRepo *discussion.Repository, userRepo *user.Repository, adminHandler *handler.AdminHandler, sessions *auth.SessionManager) *gin.Engine {
	router := gin.New()
	router.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.CORS(cfg.CORSOrigins),
		middleware.SecurityHeaders(),
		middleware.BodyLimit(middleware.DefaultBodyLimit),
		middleware.RateLimit(cfg.LoginRateLimit, cfg.LoginRateWindow,
			"/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/webauthn/login/begin",
			"/api/v1/auth/webauthn/login/finish", "/api/v1/auth/lostpass",
			"/api/v1/user/security/webauthn/register/begin",
			"/api/v1/user/security/webauthn/register/finish", "/api/v1/user/security/password",
			"/api/v1/user/security/email"),
		middleware.IPAutoLogin(cfg.OnsiteIPLogin, userRepo, sessions),
	)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", handler.NewHealthHandler(db).Get)
		v1.GET("/info", handler.NewInfoHandler().Get)
		handler.NewStatusHandler(db).RegisterRoutes(router.Group("/"))
		handler.NewSettingsHandler(db).RegisterRoutes(router.Group("/"))
		handler.NewCountdownHandler(cfg.CountdownTitle, cfg.CountdownMax, cfg.CountdownDates).RegisterRoutes(router.Group("/"))
	}
	if authHandler != nil {
		authHandler.RegisterRoutes(router.Group("/"))
	}
	if problemRepo != nil {
		var problemStore *judge.FileStore
		if judgeHandler != nil {
			problemStore = judgeHandler.Store()
		}
		handler.NewProblemHandler(problemRepo, contestRepo, sessions, userRepo, problemStore, submissionHandler.Repository()).RegisterRoutes(router.Group("/"))
		handler.NewLanguageHandler(problemRepo, contestRepo).RegisterRoutes(router.Group("/"))
		handler.NewSolutionHandler(problemRepo, sessions, userRepo).RegisterRoutes(router.Group("/"))
	}
	if submissionHandler != nil {
		hub := submission.NewRecordHub()
		submissionHandler.SetHub(hub)
		submissionHandler.Repository().SetUpdateHook(hub.Publish)
		submissionHandler.RegisterRoutes(router.Group("/"))
	}
	var activityStore *judge.FileStore
	if judgeHandler != nil {
		activityStore = judgeHandler.Store()
	}
	if contestRepo != nil {
		contestGroup := router.Group("/")
		handler.NewContestHandler(contestRepo, sessions, userRepo).RegisterRoutes(contestGroup)
		handler.NewHomeworkHandler(contestRepo, sessions, userRepo).RegisterRoutes(contestGroup)
		handler.NewActivityFileHandler(contestRepo, activityStore, sessions, userRepo, "contest", "contests").RegisterRoutes(contestGroup)
		handler.NewActivityFileHandler(contestRepo, activityStore, sessions, userRepo, "contest", "homeworks").RegisterRoutes(contestGroup)
	}
	if trainingRepo != nil {
		trainingGroup := router.Group("/")
		handler.NewTrainingHandler(trainingRepo, sessions, userRepo).RegisterRoutes(trainingGroup)
		handler.NewActivityFileHandler(trainingRepo, activityStore, sessions, userRepo, "training", "trainings").RegisterRoutes(trainingGroup)
	}
	var messageRepo *message.Repository
	var messageHub *message.Hub
	if discussionRepo != nil || userRepo != nil {
		messageRepo = message.NewRepository(db)
		messageHub = message.NewHub()
		messageRepo.SetPublisher(messageHub.Publish)
	}
	if discussionRepo != nil {
		handler.NewDiscussionHandler(discussionRepo, sessions, userRepo, messageRepo).RegisterRoutes(router.Group("/"))
	}
	if userRepo != nil {
		mailer := mail.New(mail.Config{
			Host: cfg.MailHost, Port: cfg.MailPort, Username: cfg.MailUsername,
			Password: cfg.MailPassword, From: cfg.MailFrom,
		})
		handler.NewUserHandler(userRepo, sessions, problemRepo).RegisterRoutes(router.Group("/"))
		handler.NewLostPassHandler(userRepo, sessions, mailer).RegisterRoutes(router.Group("/"))
		handler.NewMessageHandler(messageRepo, sessions, userRepo, messageHub).RegisterRoutes(router.Group("/"))
		handler.NewUserFileHandler(userRepo, activityStore, sessions).RegisterRoutes(router.Group("/"))
		handler.NewUserAvatarHandler(userRepo, activityStore, sessions).RegisterRoutes(router.Group("/"))
		handler.NewUserEmailHandler(userRepo, sessions, mailer).RegisterRoutes(router.Group("/"))
	}
	if adminHandler != nil {
		adminHandler.RegisterRoutes(router.Group("/"))
	}
	if judgeHandler != nil {
		judgeHandler.RegisterRoutes(router.Group("/"))
	}

	return router
}
