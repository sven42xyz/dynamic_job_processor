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
	"djp.chapter42.de/a/processor"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	pendingJobs []data.PendingJob
	jobsMutex   sync.Mutex
)

func main() {
	// Konfiguration laden
	config.InitConfig(logger.Log)

	// Setzt den Debug Mode
	debugMode := config.Config.Debug

	// Logger initialisieren
	logger.InitLogger(debugMode)
	defer logger.Log.Sync()

	// Geladene Jobs wiederherstellen
	persistence.RestorePendingJobs(&jobsMutex, &pendingJobs, &config.Config.Current)

	// Start des Workerpools zum parallelen Verarbeiten der Jobs
	processor.StartWorkerPool(&pendingJobs, &jobsMutex, config.Config)

	// Gin-Router initialisieren
	router := gin.Default()
	router.POST("/jobs", handlers.NewJobHandler(&jobsMutex, &pendingJobs))

	// Server starten
	port := config.Config.Port
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
		persistence.SavePendingJobs(&jobsMutex, &pendingJobs)

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
