package redeploy

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/yurykabanov/nomad-deployer/pkg"
	"github.com/yurykabanov/nomad-deployer/pkg/domain"
	"github.com/yurykabanov/nomad-deployer/pkg/nomad"
)

type redeployService struct {
	nomadClient *nomad.Client
}

var VersionNotSupportedError = errors.New("job definition lacks meta version")

func NewRedeployService(nomadClient *nomad.Client) *redeployService {
	return &redeployService{
		nomadClient: nomadClient,
	}
}

func (svc *redeployService) RedeployJob(ctx context.Context, job *domain.Job, config *domain.RedeployConfig) error {
	logger := ctx.Value(pkg.ContextLoggerKey).(log.FieldLogger)

	logger.Debugf("Triggering redeploy of job '%s'", job.Name)

	// read old jobDefinition definition
	jobDefinition, err := svc.nomadClient.ReadJob(ctx, job.Name)
	if err != nil {
		return err
	}

	if jobDefinition.Meta.Version == nil {
		return VersionNotSupportedError
	}

	// patch jobDefinition version
	*jobDefinition.Meta.Version = config.NewVersion

	// commit new jobDefinition definition
	err = svc.nomadClient.CreateJob(ctx, job.Name, jobDefinition)
	if err != nil {
		return err
	}

	logger.Debugf("Redeployment of job '%s' was successfully triggered", job.Name)

	return nil
}
