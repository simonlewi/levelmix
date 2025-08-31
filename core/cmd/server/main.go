package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/core/internal/handlers"
	ee_auth "github.com/simonlewi/levelmix/ee/auth"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
	"github.com/simonlewi/levelmix/pkg/email"
)

func main() {
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../../..")

	// Load environment variables
	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Error loading .env file to server: %v", err)
		} else {
			log.Println(".env file loaded successfully to server")
		}
	} else {
		log.Println("No .env file found, using environment variables from system/Docker")
	}

	// Initialize storage
	factory := ee_storage.NewFactory()

	audioStorage, err := factory.CreateAudioStorage()
	if err != nil {
		log.Fatal("Failed to create audio storage:", err)
	}

	metadataStorage, err := factory.CreateMetadataStorage()
	if err != nil {
		log.Fatal("Failed to create metadata storage:", err)
	}

	var emailService email.EmailService

	// Check if we're in development or production
	if os.Getenv("EMAIL_SERVICE") == "mock" || os.Getenv("RESEND_API_KEY") == "" {
		log.Println("Using mock email service (emails will be logged)")
		emailService = email.NewMockEmailService()
	} else {
		emailService, err = email.NewResendService()
		if err != nil {
			log.Printf("Failed to initialize email service, falling back to mock: %v", err)
			emailService = email.NewMockEmailService()
		} else {
			log.Println("Email service initialized successfully")
		}
	}

	// Initialize queue
	qm := audio.NewQueueManager(os.Getenv("REDIS_URL"))
	defer qm.Shutdown()

	// Initialize handlers
	uploadHandler := handlers.NewUploadHandler(audioStorage, metadataStorage, qm)
	downloadHandler := handlers.NewDownloadHandler(audioStorage, metadataStorage)
	aboutHandler := handlers.NewAboutHandler()
	pricingHandler := handlers.NewPricingHandler()
	dashboardHandler := handlers.NewDashboardHandler(metadataStorage)
	accountHandler := handlers.NewAccountHandler(metadataStorage, audioStorage)
	passwordRecoveryHandler := ee_auth.NewPasswordRecoveryHandler(metadataStorage, emailService)
	healthHandler := handlers.NewHealthHandler(metadataStorage, os.Getenv("REDIS_URL"))

	// Initialize auth
	authMiddleware := ee_auth.NewMiddleware(metadataStorage)
	authHandler := ee_auth.NewHandler(metadataStorage, authMiddleware)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	r := gin.Default()

	// Configure trusted proxies based on environment
	configureTrustedProxies(r)

	// Load templates
	baseTemplate := filepath.Join(projectRoot, "core", "templates", "base.html")
	homeTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "home.html")
	uploadTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "upload.html")
	resultsTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "results.html")
	aboutTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "about.html")
	pricingTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "pricing.html")
	loginTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "login.html")
	registerTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "register.html")
	dashboardTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "dashboard.html")
	accessTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "access.html")
	deleteAccountTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "delete-account.html")
	forgotPasswordTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "forgot-password.html")
	resetPasswordTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "reset-password.html")
	changeEmailTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "change-email.html")
	changePasswordTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "change-password.html")

	r.LoadHTMLFiles(
		baseTemplate,
		homeTemplate,
		uploadTemplate,
		resultsTemplate,
		aboutTemplate,
		pricingTemplate,
		loginTemplate,
		registerTemplate,
		dashboardTemplate,
		accessTemplate,
		deleteAccountTemplate,
		forgotPasswordTemplate,
		resetPasswordTemplate,
		changeEmailTemplate,
		changePasswordTemplate,
	)

	// Static files
	r.Static("/static", filepath.Join(projectRoot, "core", "static"))

	// Health check
	r.GET("/health", healthHandler.HealthCheck)

	// Authentication routes
	r.GET("/login", authHandler.ShowLogin)
	r.POST("/login", authHandler.HandleLogin)
	r.GET("/register", authHandler.ShowRegister)
	r.POST("/register", authHandler.HandleRegister)
	r.GET("/logout", authHandler.HandleLogout)

	// Access control routes
	r.GET("/access", handlers.ShowAccessForm)
	r.POST("/access", handlers.AccessControlMiddleware())

	// Password recovery routes (no access control needed)
	r.GET("/forgot-password", passwordRecoveryHandler.ShowForgotPassword)
	r.POST("/forgot-password", passwordRecoveryHandler.HandleForgotPassword)
	r.GET("/reset-password", passwordRecoveryHandler.ShowResetPassword)
	r.POST("/reset-password", passwordRecoveryHandler.HandleResetPassword)

	// API routes that don't need access control
	r.GET("/status/:id", uploadHandler.GetStatus)
	r.GET("/download/:id", downloadHandler.HandleDownload)
	r.GET("/results/:id", downloadHandler.ShowResults)

	// Public routes that need access control
	publicProtected := r.Group("/")
	publicProtected.Use(handlers.TemplateContext())
	publicProtected.Use(handlers.AccessControlMiddleware())
	publicProtected.Use(authMiddleware.TemplateContext())
	{
		publicProtected.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "home.html", handlers.GetTemplateData(c, gin.H{
				"CurrentPage": "home",
			}))
		})

		publicProtected.GET("/upload", authMiddleware.OptionalAuth(), func(c *gin.Context) {
			templateData := handlers.GetTemplateData(c, gin.H{
				"CurrentPage": "upload",
				"PageTitle":   "Upload",
			})

			if user, exists := c.Get("user"); exists {
				templateData["user"] = user
			}

			c.HTML(http.StatusOK, "upload.html", templateData)
		})

		publicProtected.POST("/upload", authMiddleware.OptionalAuth(), uploadHandler.HandleUpload)
		publicProtected.GET("/about", aboutHandler.ShowAbout)
		publicProtected.GET("/pricing", pricingHandler.ShowPricing)
	}

	// Protected routes (need authentication)
	protected := r.Group("/")
	protected.Use(handlers.TemplateContext())
	protected.Use(authMiddleware.RequireAuth())
	{
		protected.GET("/dashboard", dashboardHandler.ShowDashboard)
		protected.GET("/account/delete", accountHandler.ShowDeleteConfirmation)
		protected.POST("/account/delete", accountHandler.HandleDeleteAccount)
		protected.GET("/account/change-email", accountHandler.ShowChangeEmail)
		protected.POST("/account/change-email", accountHandler.HandleChangeEmail)
		protected.GET("/account/change-password", accountHandler.ShowChangePassword)
		protected.POST("/account/change-password", accountHandler.HandleChangePassword)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)

	go func() {
		if err := r.Run(":" + port); err != nil {
			log.Printf("Server error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Shutting down server...")
}

// configureTrustedProxies sets up proxy trust configuration based on environment
func configureTrustedProxies(r *gin.Engine) {
	trustedProxies := os.Getenv("TRUSTED_PROXIES")

	if trustedProxies == "" {
		// No proxies configured - direct deployment
		log.Println("No trusted proxies configured - disabling proxy trust")
		r.SetTrustedProxies(nil)
		return
	}

	// Parse comma-separated proxy IPs/CIDRs
	proxies := strings.Split(trustedProxies, ",")
	for i, proxy := range proxies {
		proxies[i] = strings.TrimSpace(proxy)
	}

	log.Printf("Configuring trusted proxies for Traefik: %v", proxies)
	if err := r.SetTrustedProxies(proxies); err != nil {
		log.Printf("Warning: Failed to set trusted proxies: %v", err)
		// Fall back to disabling proxy trust
		r.SetTrustedProxies(nil)
		return
	}

	// Configure additional middleware for Traefik headers
	r.Use(func(c *gin.Context) {
		// Traefik forwards the original protocol
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			if proto == "https" {
				c.Request.TLS = &tls.ConnectionState{}
			}
		}

		c.Next()
	})
}
