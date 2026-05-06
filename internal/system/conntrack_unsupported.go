//go:build !linux

package system

import (
	"context"
	"fmt"
)

func (c Conntrack) dropIP(ctx context.Context, value string) error {
	if _, err := parseConntrackIP(value); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: platform does not support conntrack deletion", ErrConntrackUnavailable)
}
