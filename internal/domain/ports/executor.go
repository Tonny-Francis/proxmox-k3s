package ports

import (
	"context"
	"io"
)

type CommandExecutor interface {
	Run(ctx context.Context, host string, cmd string) (stdout string, stderr string, err error)
	RunStream(ctx context.Context, host string, cmd string, out io.Writer) error
	Upload(ctx context.Context, host string, remotePath string, content []byte, mode uint32) error
}
