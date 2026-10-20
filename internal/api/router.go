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
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func NewRouter(cfg config.Config, db *mongo.Database, judgeHandler *judge.Handler, authHandler *auth.Handler, problemRepo *problem.Repository, submissionHandler *handler.SubmissionHandler, contestRepo *contest.Repository, trainingRepo *training.Repository, discussionRepo *discussion.Repository, userRepo *user.Repository, adminHandler *handler.AdminHandler, sessions *auth.SessionManager) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS(cfg.CORSOrigins))

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", handler.NewHealthHandler(db).Get)
		v1.GET("/info", handler.NewInfoHandler().Get)
	}
	if authHandler != nil {
		authHandler.RegisterRoutes(router.Group("/"))
	}
	if problemRepo != nil {
		handler.NewProblemHandler(problemRepo).RegisterRoutes(router.Group("/"))
	}
	if submissionHandler != nil {
		submissionHandler.RegisterRoutes(router.Group("/"))
	}
	if contestRepo != nil {
		contestGroup := router.Group("/")
		handler.NewContestHandler(contestRepo).RegisterRoutes(contestGroup)
		handler.NewHomeworkHandler(contestRepo).RegisterRoutes(contestGroup)
	}
	if trainingRepo != nil {
		handler.NewTrainingHandler(trainingRepo).RegisterRoutes(router.Group("/"))
	}
	if discussionRepo != nil {
		handler.NewDiscussionHandler(discussionRepo, sessions).RegisterRoutes(router.Group("/"))
	}
	if userRepo != nil {
		handler.NewUserHandler(userRepo).RegisterRoutes(router.Group("/"))
	}
	if adminHandler != nil {
		adminHandler.RegisterRoutes(router.Group("/"))
	}
	if judgeHandler != nil {
		judgeHandler.RegisterRoutes(router.Group("/"))
	}

	return router
}
