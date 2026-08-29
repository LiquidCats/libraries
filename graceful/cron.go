package graceful

import (
	"context"

	"github.com/robfig/cron/v3"
)

type Task interface {
	Spec() string
	Run()
}

func ScheduleRunner(tasks ...Task) Runner {
	cr := cron.New()

	for _, task := range tasks {
		_, err := cr.AddFunc(task.Spec(), task.Run)
		if err != nil {
			panic(err)
		}
	}

	return func(ctx context.Context) error {
		cr.Start()
		defer cr.Stop()

		<-ctx.Done()

		return ctx.Err()
	}
}
