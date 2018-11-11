package memory

import (
	"github.com/yurykabanov/nomad-deployer/pkg/domain"
)

type jobsRepository struct {
	mapping map[string][]domain.Job
}

func NewJobsRepository(mapping map[string][]domain.Job) *jobsRepository {
	return &jobsRepository{
		mapping: mapping,
	}
}

func (r *jobsRepository) FindJobsByImage(image string) []domain.Job {
	return r.mapping[image]
}
