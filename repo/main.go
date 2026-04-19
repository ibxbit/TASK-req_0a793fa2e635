package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"helios-backend/internal/approval"
	"helios-backend/internal/auth"
	"helios-backend/internal/crawler"
	"helios-backend/internal/crypto"
	"helios-backend/internal/db"
	"helios-backend/internal/handlers"
	"helios-backend/internal/idempotency"
	"helios-backend/internal/monitoring"
	"helios-backend/internal/search"
	"helios-backend/internal/settings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	db.Init()
	defer db.Close()

	auth.BootstrapAdmin()

	if err := crypto.Init(); err != nil {
		log.Fatalf("failed to init crypto: %v", err)
	}
	if err := monitoring.InitCrashDir(); err != nil {
		log.Printf("crash dir init failed: %v", err)
	}

	if err := settings.Load(); err != nil {
		log.Fatalf("failed to load settings: %v", err)
	}

	approval.StartScheduler()
	search.StartScheduler()
	idempotency.StartSweeper()

	if err := crawler.InitNode(); err != nil {
		log.Printf("crawler: node init failed: %v", err)
	}
	crawler.StartScheduler()

	monitoring.StartSampler()

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(monitoring.OnPanic))
	r.Use(monitoring.RequestMetrics())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Cookie"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	api.Use(idempotency.Middleware())
	{
		api.GET("/health", func(c *gin.Context) {
			if err := db.DB.Ping(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "up"})
		})

		auth.RegisterRoutes(api)

		handlers.RegisterDynasties(api)
		handlers.RegisterAuthors(api)
		handlers.RegisterPoems(api)
		handlers.RegisterExcerpts(api)
		handlers.RegisterTags(api)

		handlers.RegisterApprovals(api)
		handlers.RegisterSettings(api)
		handlers.RegisterSearch(api)
		handlers.RegisterContentPacks(api)
		handlers.RegisterPricing(api)
		handlers.RegisterPricingManagement(api)
		handlers.RegisterRevisions(api)

		handlers.RegisterReviews(api)
		handlers.RegisterComplaints(api)
		handlers.RegisterArbitration(api)
		handlers.RegisterCrawler(api)
		handlers.RegisterMonitoring(api)
		handlers.RegisterAudit(api)
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("helios-backend listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
