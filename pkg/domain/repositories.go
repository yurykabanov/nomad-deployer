package domain

type Job struct {
	Name string
}

type JobsRepository interface {
	FindJobsByImage(image string) []Job
}
