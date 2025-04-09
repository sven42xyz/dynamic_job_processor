package handlers

import (
	"net/http"
	"sync"
	"time"

	"djp.chapter42.de/a/internal/data"
	"djp.chapter42.de/a/internal/logger"
	"djp.chapter42.de/a/internal/processor"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewJobHandler(jobs_mutex *sync.Mutex, pending_jobs *[]data.PendingJob) gin.HandlerFunc {
	return func(c *gin.Context) {
		var job data.Job
		if err := c.BindJSON(&job); err != nil {
			logger.Log.Warn("Fehler beim Parsen des JSON-Jobs:", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges JSON-Format"})
			return
		}

		pending_job := data.PendingJob{Job: job, CreatedAt: time.Now()}

		jobs_mutex.Lock()
		*pending_jobs = append(*pending_jobs, pending_job)
		jobs_mutex.Unlock()

		select {
		case processor.JobQueue <- pending_job:
			logger.Log.Info("Neuer Job empfangen:", zap.String("uid", job.UID))
			c.JSON(http.StatusAccepted, gin.H{"message": "Job akzeptiert", "uid": job.UID})
		default:
			logger.Log.Error("Keine freien worker vorhanden für:", zap.String("uid", job.UID))
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Versuche es später nochmal", "uid": job.UID})
		}
	}
}
