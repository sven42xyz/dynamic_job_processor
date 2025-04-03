package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"djp.chapter42.de/a/config"
	"djp.chapter42.de/a/data"
	"djp.chapter42.de/a/handlers"
	"djp.chapter42.de/a/logger"
	"djp.chapter42.de/a/persistence"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	pending_jobs    []data.PendingJob
	jobs_mutex      sync.Mutex
)

func main() {
	// Konfiguration laden
	config.InitConfig(logger.Log)

	debug_mode := config.Config.GetBool("debug")

	// Logger initialisieren
	logger.InitLogger(debug_mode)
	defer logger.Log.Sync()

	// Geladene Jobs wiederherstellen
	persistence.RestorePendingJobs(&jobs_mutex, &pending_jobs)

	// Gin-Router initialisieren
	router := gin.Default()
	router.POST("/jobs", handlers.NewJobHandler(&jobs_mutex, &pending_jobs))

	// Server starten
	port := config.Config.GetString("port")
	if port == "" {
		port = config.DefaultPort
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// Goroutine für das Abfangen von Shutdown-Signalen
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		logger.Log.Info("Server wird heruntergefahren...")

		// Offene Jobs sichern
		persistence.SavePendingJobs(&jobs_mutex, &pending_jobs)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Log.Fatal("Server-Shutdown fehlgeschlagen:", zap.Error(err))
		}
		
		logger.Log.Info("Server heruntergefahren.")
	}()

	// Server starten (blockierend)
	logger.Log.Info("Server startet...", zap.String("port", port))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Log.Fatal("Fehler beim Starten des Servers:", zap.Error(err))
	}
}