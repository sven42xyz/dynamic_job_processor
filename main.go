package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Job definiert die Struktur eines zu verarbeitenden Jobs.
type Job struct {
	UID  string                 `json:"uid"`
	Data map[string]interface{} `json:"data"`
}

// PendingJob enthält den Job und zusätzliche Informationen für die Verarbeitung.
type PendingJob struct {
	Job       Job
	CreatedAt time.Time
	Attempts  int
}

const (
	defaultPort             = "8080"
	defaultCheckInterval    = 5 * time.Second
	maxCheckInterval        = 60 * time.Second
	minCheckInterval        = 1 * time.Second
	persistenceFileName     = "pending_jobs.json"
	initialPollingInterval  = 2 * time.Second
	successfulWriteInterval = 10 * time.Second
	failedCheckMultiplier   = 1.5
)

var (
	pendingJobs   []PendingJob
	jobsMutex     sync.Mutex
	logger        *zap.Logger
	config        *viper.Viper
	targetSystemURL string
)

func main() {
	// Konfiguration laden
	initConfig()

	// Logger initialisieren
	initLogger()
	defer logger.Sync()

	targetSystemURL = config.GetString("target_system_url")
	if targetSystemURL == "" {
		logger.Fatal("target_system_url ist nicht in der Konfiguration definiert")
		return
	}

	// Geladene Jobs wiederherstellen
	restorePendingJobs()

	// Goroutine für die Jobverarbeitung starten
	go processJobs()

	// Gin-Router initialisieren
	router := gin.Default()
	router.POST("/jobs", handleNewJob)

	// Server starten
	port := config.GetString("port")
	if port == "" {
		port = defaultPort
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
		logger.Info("Server wird heruntergefahren...")
		// Offene Jobs sichern
		savePendingJobs()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Fatal("Server-Shutdown fehlgeschlagen:", zap.Error(err))
		}
		logger.Info("Server heruntergefahren.")
	}()

	// Server starten (blockierend)
	logger.Info("Server startet...", zap.String("port", port))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatal("Fehler beim Starten des Servers:", zap.Error(err))
	}
}

func initConfig() {
	config = viper.New()
	config.SetDefault("port", defaultPort)
	config.SetDefault("check_interval", defaultCheckInterval)
	config.SetDefault("target_system_url", "")
	config.SetConfigName("config")
	config.SetConfigType("yaml")
	config.AddConfigPath(".")
	err := config.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("Konfigurationsdatei nicht gefunden, verwende Standardwerte")
		} else {
			logger.Error("Fehler beim Lesen der Konfigurationsdatei:", zap.Error(err))
		}
	}
}

func initLogger() {
	level := zap.NewAtomicLevel()
	if config.GetBool("debug") {
		level.SetLevel(zap.DebugLevel)
	} else {
		level.SetLevel(zap.InfoLevel)
	}

	cfg := zap.Config{
		Level:            level,
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:  "msg",
			LevelKey:    "level",
			TimeKey:     "time",
			CallerKey:   "caller",
			EncodeLevel: zapcore.LowercaseLevelEncoder,
			EncodeTime:  zapcore.ISO8601TimeEncoder,
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}

	var err error
	logger, err = cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("Fehler beim Initialisieren des Loggers: %v", err))
	}
}

func handleNewJob(c *gin.Context) {
	var job Job
	if err := c.BindJSON(&job); err != nil {
		logger.Warn("Fehler beim Parsen des JSON-Jobs:", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges JSON-Format"})
		return
	}

	jobsMutex.Lock()
	pendingJobs = append(pendingJobs, PendingJob{Job: job, CreatedAt: time.Now()})
	jobsMutex.Unlock()

	logger.Info("Neuer Job empfangen:", zap.String("uid", job.UID))
	c.JSON(http.StatusAccepted, gin.H{"message": "Job akzeptiert", "uid": job.UID})
}

func processJobs() {
	pollingInterval := initialPollingInterval

	for {
		time.Sleep(pollingInterval)

		jobsMutex.Lock()
		if len(pendingJobs) == 0 {
			jobsMutex.Unlock()
			pollingInterval = successfulWriteInterval // Langsameres Polling, wenn keine Jobs anstehen
			continue
		}

		var nextPendingJobs []PendingJob
		var calculatedPollingInterval time.Duration

		for _, pJob := range pendingJobs {
			writable, err := checkWritable(pJob.Job.UID)
			if err != nil {
				logger.Error("Fehler beim Überprüfen des Schreibzugriffs:", zap.String("uid", pJob.Job.UID), zap.Error(err))
				pJob.Attempts++
				nextPendingJobs = append(nextPendingJobs, pJob)
				calculatedPollingInterval = time.Duration(float64(pollingInterval) * failedCheckMultiplier)
				pollingInterval = min(calculatedPollingInterval, maxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
				continue
			}

			if writable {
				err := writeData(pJob.Job.UID, pJob.Job.Data)
				if err != nil {
					logger.Error("Fehler beim Schreiben der Daten:", zap.String("uid", pJob.Job.UID), zap.Error(err))
					pJob.Attempts++
					nextPendingJobs = append(nextPendingJobs, pJob)
					calculatedPollingInterval = time.Duration(float64(pollingInterval) * failedCheckMultiplier)
					pollingInterval = min(calculatedPollingInterval, maxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
				} else {
					logger.Info("Daten erfolgreich geschrieben:", zap.String("uid", pJob.Job.UID))
					pollingInterval = successfulWriteInterval // Langsameres Polling nach erfolgreichem Schreiben
				}
			} else {
				pJob.Attempts++
				nextPendingJobs = append(nextPendingJobs, pJob)
				calculatedPollingInterval = time.Duration(float64(pollingInterval) * failedCheckMultiplier)
				pollingInterval = min(calculatedPollingInterval, maxCheckInterval) // Dynamische Anpassung der Abfragerate, wenn Objekt blockiert ist
			}
		}
		pendingJobs = nextPendingJobs
		jobsMutex.Unlock()
	}
}

func checkWritable(uid string) (bool, error) {
	checkURL := fmt.Sprintf("%s/objects/%s/writable", targetSystemURL, uid)
	resp, err := http.Get(checkURL)
	if err != nil {
		return false, fmt.Errorf("fehler beim Aufruf der Schreibstatus-API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		logger.Warn("Zielobjekt nicht gefunden:", zap.String("uid", uid))
		return false, nil // Objekt existiert nicht oder ist nicht auffindbar, nicht als Blockade interpretieren
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Debug("Schreibstatus-API Antwort:", zap.String("status", resp.Status), zap.String("body", string(bodyBytes)))
		return false, nil // Andere Statuscodes deuten auf Blockade oder Fehler hin
	}
}

func writeData(uid string, data map[string]interface{}) error {
	writeURL := fmt.Sprintf("%s/objects/%s", targetSystemURL, uid)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("fehler beim Serialisieren der Daten zu JSON: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, writeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("fehler beim Erstellen der PUT-Anfrage: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fehler beim Senden der PUT-Anfrage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fehler beim Schreiben der Daten, Status: %s, Body: %s", resp.Status, string(bodyBytes))
	}
}

func savePendingJobs() {
	jobsMutex.Lock()
	defer jobsMutex.Unlock()

	data, err := json.MarshalIndent(pendingJobs, "", "  ")
	if err != nil {
		logger.Error("Fehler beim Serialisieren der ausstehenden Jobs:", zap.Error(err))
		return
	}

	err = os.WriteFile(persistenceFileName, data, 0644)
	if err != nil {
		logger.Error("Fehler beim Speichern der ausstehenden Jobs in die Datei:", zap.String("filename", persistenceFileName), zap.Error(err))
	} else {
		logger.Info("Ausstehende Jobs in Datei gespeichert:", zap.String("filename", persistenceFileName), zap.Int("count", len(pendingJobs)))
	}
}

func restorePendingJobs() {
	data, err := os.ReadFile(persistenceFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("Fehler beim Lesen der ausstehenden Jobs aus der Datei:", zap.String("filename", persistenceFileName), zap.Error(err))
		}
		return
	}

	err = json.Unmarshal(data, &pendingJobs)
	if err != nil {
		logger.Error("Fehler beim Deserialisieren der ausstehenden Jobs:", zap.String("filename", persistenceFileName), zap.Error(err))
		return
	}

	logger.Info("Ausstehende Jobs aus Datei wiederhergestellt:", zap.String("filename", persistenceFileName), zap.Int("count", len(pendingJobs)))
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

/* func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
} */