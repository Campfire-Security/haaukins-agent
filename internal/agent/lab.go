package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aau-network-security/haaukins-agent/internal/environment"
	"github.com/aau-network-security/haaukins-agent/internal/environment/lab"
	"github.com/aau-network-security/haaukins-agent/internal/environment/lab/exercise"
	"github.com/aau-network-security/haaukins-agent/pkg/proto"
	eproto "github.com/aau-network-security/haaukins-exercises/proto"
	"github.com/rs/zerolog/log"
)

// For the daemon to listen to. New labs created with the workers are pushed to the daemon through the stream when they are created and running.
func (a *Agent) LabStream(req *proto.Empty, stream proto.Agent_LabStreamServer) error {
	for {
		select {
		case lab := <-a.newLabs:
			log.Debug().Msg("Lab in new lab channel, sending to client...")
			stream.Send(&lab)
		}
	}
}

// TODO: Rethink func name as this should be the function that configures a lab for a user
// TODO: Handle assignment (Guac connection and VPN configs here)
func (a *Agent) CreateLabForEnv(ctx context.Context, req *proto.CreateLabRequest) (*proto.StatusResponse, error) {
	a.State.EnvPool.M.RLock()
	env, ok := a.State.EnvPool.Envs[req.EventTag]
	a.State.EnvPool.M.RUnlock()
	if !ok {
		return nil, errors.New("environment for event does not exist")
	}

	if env.EnvConfig.Type == environment.BeginnerType && req.IsVPN {
		return nil, errors.New("cannot create vpn lab for beginner environment")
	}

	ec := env.EnvConfig

	m := &sync.RWMutex{}
	ec.WorkerPool.AddTask(func() {
		ctx := context.Background()

		// Creating containers etc.
		lab, err := ec.LabConf.NewLab(ctx, req.IsVPN, ec.Type, ec.Tag)
		if err != nil {
			log.Error().Err(err).Str("eventTag", env.EnvConfig.Tag).Msg("error creating new lab")
			return
		}
		// Starting the created containers and frontends
		if err := lab.Start(ctx); err != nil {
			log.Error().Err(err).Str("eventTag", env.EnvConfig.Tag).Msg("error starting new lab")
			return
		}
		// Sending lab info to daemon
		newLab := proto.Lab{
			Tag:      lab.Tag,
			EventTag: ec.Tag,
			IsVPN:    req.IsVPN,
		}

		a.newLabs <- newLab
		m.Lock()
		env.Labs[lab.Tag] = &lab
		m.Unlock()
	})
	return &proto.StatusResponse{Message: "OK"}, nil
}

// Shuts down and removes all frontends and containers related to specific lab. Then removes it from the environment's lab map.
func (a *Agent) CloseLab(ctx context.Context, req *proto.CloseLabRequest) (*proto.StatusResponse, error) {
	l, err := a.State.EnvPool.GetLabByTag(req.LabTag)
	if err != nil {
		log.Error().Str("labTag", req.LabTag).Err(err).Msg("error getting lab by tag")
		return nil, err
	}

	a.workerPool.AddTask(func() {
		l.M.Lock()
		defer l.M.Unlock()
		if err := l.Close(); err != nil {
			log.Error().Err(err).Msg("error closing lab")
		}
	})

	envKey := strings.Split(req.LabTag, "-")
	log.Debug().Str("envKey", envKey[0]).Msg("env for lab")

	delete(a.State.EnvPool.Envs[envKey[0]].Labs, req.LabTag)

	return &proto.StatusResponse{Message: "OK"}, nil
}

// GRPc endpoint that adds exercises to an already running lab. It requires the lab tag, and an array of exercise tags.
// It starts by creating the containers needed for the exercise, then it refreshes the DNS and starts the containers afterwards.
// It utilizes a mutex lock to make sure that if anyone tries to run the same GRPc call twice without the first being finished, the second one will wait
func (a *Agent) AddExercisesToLab(ctx context.Context, req *proto.AddExercisesRequest) (*proto.StatusResponse, error) {
	l, err := a.State.EnvPool.GetLabByTag(req.LabTag)
	if err != nil {
		log.Error().Str("labTag", req.LabTag).Err(err).Msg("error getting lab by tag")
		return nil, err
	}

	if l.Type == lab.BeginnerType {
		return nil, errors.New("cannot add arbitrary exercise to lab of type beginner")
	}

	var exerConfs []exercise.ExerciseConfig
	exerDbConfs, err := a.State.ExClient.GetExerciseByTags(ctx, &eproto.GetExerciseByTagsRequest{Tag: req.Exercises})
	if err != nil {
		log.Error().Err(err).Msg("error getting exercise by tags")
		return nil, errors.New(fmt.Sprintf("error getting exercises: %s", err))
	}

	// Unpack into exercise slice
	for _, e := range exerDbConfs.Exercises {
		ex, err := protobufToJson(e)
		if err != nil {
			return nil, err
		}
		estruct := exercise.ExerciseConfig{}
		json.Unmarshal([]byte(ex), &estruct)
		exerConfs = append(exerConfs, estruct)
	}

	// Add exercises to lab
	ctx = context.Background()
	if err := l.AddAndStartExercises(ctx, exerConfs...); err != nil {
		log.Error().Err(err).Msg("error adding and starting exercises")
		return nil, errors.New(fmt.Sprintf("error adding and starting exercises: %v", err))
	}

	// TODO: Need to return host information back to daemon to display to user in case of VPN lab
	return &proto.StatusResponse{Message: "OK"}, nil
}
