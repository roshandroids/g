package command

import (
	"context"

	"github.com/roshandroids/g/internal/output"
)

// runSync implements `g sync`.
func runSync(ctx context.Context, env *Env, args []string) error {
	if len(args) > 0 {
		return usageErrorf("sync takes no arguments")
	}

	result, err := env.Service.Sync(ctx, env.Dir)
	if err != nil {
		return err
	}
	return output.RenderSync(env.Out, result)
}
