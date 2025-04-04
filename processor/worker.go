package processor

import (
	"sync"

	"djp.chapter42.de/a/data"
)

// Todo:
// - Export the worker delimiters to config
// - Make the worker number dynamic 
// -- let it adapt to outside factors such as http-429



const (
	MinWorkers int = 5
	MaxWorkers int = 10
)

var JobQueue = make(chan data.PendingJob, 100)

func worker(pending_jobs *[]data.PendingJob, job_mutex *sync.Mutex) {
	for job := range JobQueue {
		ProcessJob(job, pending_jobs, job_mutex)
	}
}

func StartWorkerPool(pending_jobs *[]data.PendingJob, job_mutex *sync.Mutex) {
	for i := 0; i < MaxWorkers; i++ {
		go worker(pending_jobs, job_mutex)
	}
}