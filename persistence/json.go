package persistence

import (
	"encoding/json"
	"os"
	"sync"

	"djp.chapter42.de/a/data"
	"djp.chapter42.de/a/logger"
	"djp.chapter42.de/a/processor"
	"go.uber.org/zap"
)

const PersistenceFileName string = "pending_jobs.json"

func SavePendingJobs(jobs_mutex *sync.Mutex, pending_jobs *[]data.PendingJob) {
	jobs_mutex.Lock()
	defer jobs_mutex.Unlock()

	data, err := json.MarshalIndent(*pending_jobs, "", "  ")
	if err != nil {
		logger.Log.Error("Fehler beim Serialisieren der ausstehenden Jobs:", zap.Error(err))
		return
	}

	err = os.WriteFile(PersistenceFileName, data, 0644)
	if err != nil {
		logger.Log.Error("Fehler beim Speichern der ausstehenden Jobs in die Datei:", zap.String("filename", PersistenceFileName), zap.Error(err))
	} else {
		logger.Log.Info("Ausstehende Jobs in Datei gespeichert:", zap.String("filename", PersistenceFileName), zap.Int("count", len(*pending_jobs)))
	}
}

func RestorePendingJobs(jobs_mutex *sync.Mutex, pending_jobs *[]data.PendingJob, currentCfg *data.CurrentConfig) {
	data, err := os.ReadFile(PersistenceFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Log.Error("Fehler beim Lesen der ausstehenden Jobs aus der Datei:", zap.String("filename", PersistenceFileName), zap.Error(err))
		}
		return
	}

	err = json.Unmarshal(data, &pending_jobs)
	if err != nil {
		logger.Log.Error("Fehler beim Deserialisieren der ausstehenden Jobs:", zap.String("filename", PersistenceFileName), zap.Error(err))
		return
	}

	logger.Log.Info("Ausstehende Jobs aus Datei wiederhergestellt:", zap.String("filename", PersistenceFileName), zap.Int("count", len(*pending_jobs)))

	for _, job := range *pending_jobs {
		go processor.ProcessJob(job, pending_jobs, jobs_mutex, currentCfg)
	}
}
