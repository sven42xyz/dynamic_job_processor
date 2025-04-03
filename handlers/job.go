package handlers

import (
	"net/http"
	"sync"
	"time"

	"djp.chapter42.de/a/data"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewJobHandler(logger *zap.Logger, jobs_mutex *sync.Mutex, pending_jobs *[]data.PendingJob) gin.HandlerFunc {
	return func(c *gin.Context) {
		var job data.Job
		if err := c.BindJSON(&job); err != nil {
			logger.Warn("Fehler beim Parsen des JSON-Jobs:", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges JSON-Format"})
			return
		}
	
		jobs_mutex.Lock()
		*pending_jobs = append(*pending_jobs, data.PendingJob{Job: job, CreatedAt: time.Now()})
		jobs_mutex.Unlock()
	
		logger.Info("Neuer Job empfangen:", zap.String("uid", job.UID))
		c.JSON(http.StatusAccepted, gin.H{"message": "Job akzeptiert", "uid": job.UID})
	}
}