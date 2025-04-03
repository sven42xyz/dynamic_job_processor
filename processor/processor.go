package processor

import (
	"sync"
	"time"

	"djp.chapter42.de/a/data"
	"djp.chapter42.de/a/external"
	"djp.chapter42.de/a/logger"
	timebackoff "djp.chapter42.de/a/time_backoff"
	"go.uber.org/zap"
)

const (
	MaxCheckInterval        time.Duration = 60 * time.Second
	MinCheckInterval        time.Duration = 1 * time.Second
	InitialPollingInterval  time.Duration = 2 * time.Second
	SuccessfulWriteInterval time.Duration = 10 * time.Second
	FailedCheckMultiplier   float64       = 1.5
)

func ProcessJob(job data.PendingJob, pending_jobs *[]data.PendingJob, job_mutex *sync.Mutex) {
	pollingInterval := InitialPollingInterval

	for {
		time.Sleep(pollingInterval)

		writable, err := external.WriteCheck(job.Job.UID)
		if err != nil {
			logger.Log.Error("Fehler beim Überprüfen des Schreibzugriffs:", zap.String("uid", job.Job.UID), zap.Error(err))
			job.Attempts++
			pollingInterval = timebackoff.Min(time.Duration(float64(pollingInterval) * FailedCheckMultiplier), MaxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
			continue
		}

		if writable {
			err := external.WriteData(job.Job.UID, job.Job.Data)
			if err != nil {
				logger.Log.Error("Fehler beim Schreiben der Daten:", zap.String("uid", job.Job.UID), zap.Error(err))
				job.Attempts++
				pollingInterval = timebackoff.Min(time.Duration(float64(pollingInterval) * FailedCheckMultiplier), MaxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
			} else {
				logger.Log.Info("Daten erfolgreich geschrieben:", zap.String("uid", job.Job.UID))

				job_mutex.Lock()
                for i, j := range *pending_jobs {
                    if j.Job.UID == job.Job.UID {
                        *pending_jobs = append((*pending_jobs)[:i], (*pending_jobs)[i+1:]...) // Job entfernen
                        break
                    }
                }
                job_mutex.Unlock()

				return
			}
		} else {
			job.Attempts++
			pollingInterval = timebackoff.Min(time.Duration(float64(pollingInterval) * FailedCheckMultiplier), MaxCheckInterval) // Dynamische Anpassung der Abfragerate, wenn Objekt blockiert ist
		}
	}
}
