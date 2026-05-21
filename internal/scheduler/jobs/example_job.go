package jobs

import (
	"go.uber.org/zap"
)

type ExampleJob struct {
	logger *zap.Logger
}

func NewExampleJob(logger *zap.Logger) *ExampleJob {
	return &ExampleJob{logger: logger}
}

func (j *ExampleJob) Run() {
	j.logger.Info("example job executed")
}
