package processor

import (
	"sync"
	"time"

	"djp.chapter42.de/a/internal/data"
	"djp.chapter42.de/a/internal/external"
	"djp.chapter42.de/a/internal/logger"
	timebackoff "djp.chapter42.de/a/internal/time_backoff"
	"go.uber.org/zap"
)

func ProcessJob(job data.PendingJob, pendingJobs *[]data.PendingJob, jobMutex *sync.Mutex, currentCfg *data.CurrentConfig) {
	backoff := timebackoff.NewSinusBackoff()

	for {
		time.Sleep(backoff.CalculateBackoff(job.Attempts))

		latestRevision, err := external.LatestRevision(&job.Job, currentCfg)
		if err != nil {
			logger.Log.Error("Konnte die neueste Revision nicht abrufen:", zap.String("uid", job.Job.UID), zap.Error(err))
			job.Attempts++
			continue
		}
		job.Job.UID = latestRevision

		writable, err := external.WriteCheck(&job.Job, currentCfg)
		if err != nil {
			logger.Log.Error("Fehler beim Überprüfen des Schreibzugriffs:", zap.String("uid", job.Job.UID), zap.Error(err))
			job.Attempts++
			continue
		}

		if writable {
			err := external.WriteData(&job.Job, job.Job.Data, currentCfg)
			if err != nil {
				logger.Log.Error("Fehler beim Schreiben der Daten:", zap.String("uid", job.Job.UID), zap.Error(err))
				job.Attempts++
			} else {
				logger.Log.Info("Daten erfolgreich geschrieben:", zap.String("uid", job.Job.UID))

				jobMutex.Lock()
				for i, j := range *pendingJobs {
					if j.Job.UID == job.Job.UID {
						*pendingJobs = append((*pendingJobs)[:i], (*pendingJobs)[i+1:]...) // Job entfernen
						break
					}
				}
				jobMutex.Unlock()

				return
			}
		} else {
			job.Attempts++
		}
	}
}
