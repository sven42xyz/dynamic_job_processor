package data

import "time"

// PendingJob enthält den Job und zusätzliche Informationen für die Verarbeitung.
type PendingJob struct {
	Job       Job
	CreatedAt time.Time
	Attempts  int
}