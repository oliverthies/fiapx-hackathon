package domain

type JobStatus string

const (
	StatusUploaded   JobStatus = "UPLOADED"
	StatusQueued     JobStatus = "QUEUED"
	StatusProcessing JobStatus = "PROCESSING"
	StatusReady      JobStatus = "READY"
	StatusFailed     JobStatus = "FAILED"
)

func (s JobStatus) CanTransitionTo(next JobStatus) bool {
	switch s {
	case StatusUploaded:
		return next == StatusQueued
	case StatusQueued:
		return next == StatusProcessing
	case StatusProcessing:
		return next == StatusReady || next == StatusFailed
	default:
		return false
	}
}
