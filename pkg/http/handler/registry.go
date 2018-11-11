package handler

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/yurykabanov/nomad-deployer/pkg"
	"github.com/yurykabanov/nomad-deployer/pkg/domain"
	"github.com/yurykabanov/nomad-deployer/pkg/domain/redeploy"
	"github.com/yurykabanov/nomad-deployer/pkg/registry"
)

type registryCallbackHandler struct {
	jobsRepository  domain.JobsRepository
	redeployService domain.RedeployService
}

func NewRegistryCallbackHandler(jobsRepository domain.JobsRepository, redeployService domain.RedeployService) *registryCallbackHandler {
	return &registryCallbackHandler{
		jobsRepository:  jobsRepository,
		redeployService: redeployService,
	}
}

func (h *registryCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := r.Context().Value(pkg.ContextLoggerKey).(log.FieldLogger)

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Error while reading body")

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		return
	}

	var notifications registry.Notifications
	json.Unmarshal(b, &notifications)

	for _, evt := range notifications.Events {
		// skip all untagged or non-push (if any) events, these may relate to some layers or something else
		if !evt.IsPush() || !evt.HasTag() {
			continue
		}

		jobs := h.jobsRepository.FindJobsByImage(evt.Target.Repository)

		// skip all tagged push events if we don't know about any jobs that are affected by corresponding image
		if jobs == nil {
			continue
		}

		logger.WithField("jobs", jobs).Infof(
			"Found %d jobs to redeploy after image '%s:%s' was pushed",
			len(jobs), evt.Target.Repository, evt.Target.Tag,
		)

		// for each affected job
		for i := range jobs {
			logger := logger.WithFields(log.Fields{
				"repository": evt.Target.Repository,
				"tag":        evt.Target.Tag,
				"job_name":   jobs[i].Name,
			})

			ctx := context.WithValue(r.Context(), pkg.ContextLoggerKey, logger)

			// trigger redeployment
			err := h.redeployService.RedeployJob(ctx, &jobs[i], &domain.RedeployConfig{NewVersion: evt.Target.Tag})
			if err != nil {
				if err == redeploy.VersionNotSupportedError {
					logger.WithError(err).Error("Unable to trigger job redeployment due to definition lacks version support")
				} else {
					logger.WithError(err).Error("Unable to read/write job definition in nomad")
				}

				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal server error"))
				return
			}
		}
	}

	w.Write([]byte("ok"))
}
