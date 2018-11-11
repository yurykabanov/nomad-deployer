package domain

import (
	"context"
)

type RedeployConfig struct {
	NewVersion string
}

type RedeployService interface {
	RedeployJob(context.Context, *Job, *RedeployConfig) error
}
