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

	"djp.chapter42.de/a/data"
	"djp.chapter42.de/a/handlers"
	"djp.chapter42.de/a/config"
	"djp.chapter42.de/a/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Job definiert die Struktur eines zu verarbeitenden Jobs.
type Job struct {
	UID  string                 `json:"uid"`
	Data map[string]interface{} `json:"data"`
}

const (
	maxCheckInterval        = 60 * time.Second
	minCheckInterval        = 1 * time.Second
	persistenceFileName     = "pending_jobs.json"
	initialPollingInterval  = 2 * time.Second
	successfulWriteInterval = 10 * time.Second
	failedCheckMultiplier   = 1.5
)

var (
	pending_jobs    []data.PendingJob
	jobs_mutex      sync.Mutex
	targetSystemURL string
)

func main() {
	// Konfiguration laden
	config.InitConfig(logger.Log)

	debug_mode := config.Config.GetBool("debug")

	// Logger initialisieren
	logger.InitLogger(debug_mode)
	defer logger.Log.Sync()

	targetSystemURL = config.Config.GetString("target_system_url")
	if targetSystemURL == "" {
		logger.Log.Fatal("target_system_url ist nicht in der Konfiguration definiert")
		return
	}

	// Geladene Jobs wiederherstellen
	restorePendingJobs()

	// Goroutine für die Jobverarbeitung starten
	go processJobs()

	// Gin-Router initialisieren
	router := gin.Default()
	router.POST("/jobs", handlers.NewJobHandler(logger.Log, &jobs_mutex, &pending_jobs))

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
		savePendingJobs()
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

func processJobs() {
	pollingInterval := initialPollingInterval

	for {
		time.Sleep(pollingInterval)

		jobs_mutex.Lock()
		if len(pending_jobs) == 0 {
			jobs_mutex.Unlock()
			pollingInterval = successfulWriteInterval // Langsameres Polling, wenn keine Jobs anstehen
			continue
		}

		var nextPendingJobs []data.PendingJob
		var calculatedPollingInterval time.Duration

		for _, pJob := range pending_jobs {
			writable, err := checkWritable(pJob.Job.UID)
			if err != nil {
				logger.Log.Error("Fehler beim Überprüfen des Schreibzugriffs:", zap.String("uid", pJob.Job.UID), zap.Error(err))
				pJob.Attempts++
				nextPendingJobs = append(nextPendingJobs, pJob)
				calculatedPollingInterval = time.Duration(float64(pollingInterval) * failedCheckMultiplier)
				pollingInterval = min(calculatedPollingInterval, maxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
				continue
			}

			if writable {
				err := writeData(pJob.Job.UID, pJob.Job.Data)
				if err != nil {
					logger.Log.Error("Fehler beim Schreiben der Daten:", zap.String("uid", pJob.Job.UID), zap.Error(err))
					pJob.Attempts++
					nextPendingJobs = append(nextPendingJobs, pJob)
					calculatedPollingInterval = time.Duration(float64(pollingInterval) * failedCheckMultiplier)
					pollingInterval = min(calculatedPollingInterval, maxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
				} else {
					logger.Log.Info("Daten erfolgreich geschrieben:", zap.String("uid", pJob.Job.UID))
					pollingInterval = successfulWriteInterval // Langsameres Polling nach erfolgreichem Schreiben
				}
			} else {
				pJob.Attempts++
				nextPendingJobs = append(nextPendingJobs, pJob)
				calculatedPollingInterval = time.Duration(float64(pollingInterval) * failedCheckMultiplier)
				pollingInterval = min(calculatedPollingInterval, maxCheckInterval) // Dynamische Anpassung der Abfragerate, wenn Objekt blockiert ist
			}
		}
		pending_jobs = nextPendingJobs
		jobs_mutex.Unlock()
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
		logger.Log.Warn("Zielobjekt nicht gefunden:", zap.String("uid", uid))
		return false, nil // Objekt existiert nicht oder ist nicht auffindbar, nicht als Blockade interpretieren
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Log.Debug("Schreibstatus-API Antwort:", zap.String("status", resp.Status), zap.String("body", string(bodyBytes)))
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
	jobs_mutex.Lock()
	defer jobs_mutex.Unlock()

	data, err := json.MarshalIndent(pending_jobs, "", "  ")
	if err != nil {
		logger.Log.Error("Fehler beim Serialisieren der ausstehenden Jobs:", zap.Error(err))
		return
	}

	err = os.WriteFile(persistenceFileName, data, 0644)
	if err != nil {
		logger.Log.Error("Fehler beim Speichern der ausstehenden Jobs in die Datei:", zap.String("filename", persistenceFileName), zap.Error(err))
	} else {
		logger.Log.Info("Ausstehende Jobs in Datei gespeichert:", zap.String("filename", persistenceFileName), zap.Int("count", len(pending_jobs)))
	}
}

func restorePendingJobs() {
	data, err := os.ReadFile(persistenceFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Log.Error("Fehler beim Lesen der ausstehenden Jobs aus der Datei:", zap.String("filename", persistenceFileName), zap.Error(err))
		}
		return
	}

	err = json.Unmarshal(data, &pending_jobs)
	if err != nil {
		logger.Log.Error("Fehler beim Deserialisieren der ausstehenden Jobs:", zap.String("filename", persistenceFileName), zap.Error(err))
		return
	}

	logger.Log.Info("Ausstehende Jobs aus Datei wiederhergestellt:", zap.String("filename", persistenceFileName), zap.Int("count", len(pending_jobs)))
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
