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

func ProcessJobs(jobs_mutex *sync.Mutex, pending_jobs *[]data.PendingJob, ) {
	pollingInterval := InitialPollingInterval

	for {
		time.Sleep(pollingInterval)

		jobs_mutex.Lock()
		if len(*pending_jobs) == 0 {
			jobs_mutex.Unlock()
			pollingInterval = SuccessfulWriteInterval // Langsameres Polling, wenn keine Jobs anstehen
			continue
		}

		var nextPendingJobs []data.PendingJob
		var calculatedPollingInterval time.Duration

		for _, pJob := range *pending_jobs {
			writable, err := external.WriteCheck(pJob.Job.UID)
			if err != nil {
				logger.Log.Error("Fehler beim Überprüfen des Schreibzugriffs:", zap.String("uid", pJob.Job.UID), zap.Error(err))
				pJob.Attempts++
				nextPendingJobs = append(nextPendingJobs, pJob)
				calculatedPollingInterval = time.Duration(float64(pollingInterval) * FailedCheckMultiplier)
				pollingInterval = timebackoff.Min(calculatedPollingInterval, MaxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
				continue
			}

			if writable {
				err := external.WriteData(pJob.Job.UID, pJob.Job.Data)
				if err != nil {
					logger.Log.Error("Fehler beim Schreiben der Daten:", zap.String("uid", pJob.Job.UID), zap.Error(err))
					pJob.Attempts++
					nextPendingJobs = append(nextPendingJobs, pJob)
					calculatedPollingInterval = time.Duration(float64(pollingInterval) * FailedCheckMultiplier)
					pollingInterval = timebackoff.Min(calculatedPollingInterval, MaxCheckInterval) // Dynamische Anpassung der Abfragerate bei Fehler
				} else {
					logger.Log.Info("Daten erfolgreich geschrieben:", zap.String("uid", pJob.Job.UID))
					pollingInterval = SuccessfulWriteInterval // Langsameres Polling nach erfolgreichem Schreiben
				}
			} else {
				pJob.Attempts++
				nextPendingJobs = append(nextPendingJobs, pJob)
				calculatedPollingInterval = time.Duration(float64(pollingInterval) * FailedCheckMultiplier)
				pollingInterval = timebackoff.Min(calculatedPollingInterval, MaxCheckInterval) // Dynamische Anpassung der Abfragerate, wenn Objekt blockiert ist
			}
		}
		*pending_jobs = nextPendingJobs
		jobs_mutex.Unlock()
	}
}
