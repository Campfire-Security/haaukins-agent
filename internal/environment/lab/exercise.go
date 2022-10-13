package lab

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aau-network-security/haaukins-agent/internal/environment/lab/exercise"
	"github.com/rs/zerolog/log"
)

// AddExercises uses exercise configs from the exercise service to configure containers and flags to be started at a later time
func (l *Lab) AddExercises(ctx context.Context, confs ...exercise.ExerciseConfig) error {
	var e *exercise.Exercise
	var aRecord string

	for _, conf := range confs {
		if conf.Tag == "" {
			return errors.New("No tags, need atleast one tag")
		}

		if _, ok := l.ExTags[conf.Tag]; ok {
			return errors.New("Tag already exists")
		}

		if conf.Static {
			// TODO remove static exercises on agent side, but need the overview first
			e = exercise.NewExercise(conf, nil, nil, "")
		} else {
			e = exercise.NewExercise(conf, l.Vlib, l.Network, l.DnsAddress)
			if err := e.Create(ctx); err != nil {
				return err
			}
			ip := strings.Split(e.DnsAddr, ".")

			for i, c := range e.ContainerOpts {
				for _, r := range c.Records {
					if strings.Contains(c.DockerConf.Image, "client") {
						continue
					}
					if r.Type == "A" {
						aRecord = r.Name
						l.DnsRecords = append(l.DnsRecords, &DNSRecord{Record: map[string]string{
							fmt.Sprintf("%s.%s.%s.%d", ip[0], ip[1], ip[2], e.Ips[i]): aRecord,
						}})
					}
				}
			}
		}
		l.ExTags[conf.Tag] = e
		l.Exercises = append(l.Exercises, e)
	}

	return nil
}

// Used to add exercises to an already running lab.
// It configures the containers, refreshes the DNS to add the new records and then starts the new exercise containers
func (l *Lab) AddAndStartExercises(ctx context.Context, exerConfs ...exercise.ExerciseConfig) error {
	l.M.Lock()
	defer l.M.Unlock()

	if err := l.AddExercises(ctx, exerConfs...); err != nil {
		log.Error().Err(err).Msg("error adding exercise to lab")
		return err
	}

	// Refresh the DNS
	if err := l.RefreshDNS(ctx); err != nil {
		log.Error().Err(err).Msg("error refreshing DNS")
		return err
	}

	// Start the exercises
	var res error
	var wg sync.WaitGroup
	for _, ex := range l.Exercises {
		wg.Add(1)
		go func(e *exercise.Exercise) {
			if err := e.Start(ctx); err != nil {
				// TODO: https://pkg.go.dev/github.com/hashicorp/go-multierror
				res = err
			}
			wg.Done()
		}(ex)
	}
	wg.Wait()
	if res != nil {
		return res
	}
	return nil
}

func (l *Lab) GetChallenges() []exercise.Challenge {
	var challenges []exercise.Challenge
	for _, e := range l.Exercises {
		challenges = append(challenges, e.GetChallenges()...)
	}
	return challenges
}
