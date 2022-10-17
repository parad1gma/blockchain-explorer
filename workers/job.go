package workers

import (
	"context"
	"log"
)

type ExecutionFn func(ctx context.Context, args interface{}) (interface{}, error)

type Result struct {
	Value interface{}
	Err   error
}

type Job struct {
	ExecFn ExecutionFn
	Args   interface{}
}

func (j Job) execute(ctx context.Context) Result {
	value, err := j.ExecFn(ctx, j.Args)
	if err != nil {
		log.Println("Execute error ", err.Error())
		return Result{
			Err: err,
		}
	}

	return Result{
		Value: value,
	}
}
