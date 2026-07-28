package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Executor struct {
	privateKeyPath string
	user           string
	port           int
	key            ssh.Signer
}

func NewExecutor(privateKeyPath, user string, port int) (*Executor, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading SSH key %q: %w", privateKeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parsing SSH private key: %w", err)
	}

	if port == 0 {
		port = 22
	}
	if user == "" {
		user = "ubuntu"
	}

	return &Executor{
		privateKeyPath: privateKeyPath,
		user:           user,
		port:           port,
		key:            signer,
	}, nil
}

func (e *Executor) dialWithRetry(ctx context.Context, host string) (*ssh.Client, error) {
	addr := fmt.Sprintf("%s:%d", host, e.port)
	cfg := &ssh.ClientConfig{
		User:            e.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(e.key)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("SSH connect to %s: %w (last error: %v)", host, ctx.Err(), lastErr)
			}
			return nil, ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}

		sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
		if err != nil {
			conn.Close()
			lastErr = err
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}

		return ssh.NewClient(sshConn, chans, reqs), nil
	}
}

func (e *Executor) Run(ctx context.Context, host, cmd string) (string, string, error) {
	client, err := e.dialWithRetry(ctx, host)
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("creating SSH session on %s: %w", host, err)
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	err = sess.Run(cmd)
	return stdout.String(), stderr.String(), err
}

func (e *Executor) RunStream(ctx context.Context, host, cmd string, out io.Writer) error {
	client, err := e.dialWithRetry(ctx, host)
	if err != nil {
		return err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session on %s: %w", host, err)
	}
	defer sess.Close()

	sess.Stdout = out
	sess.Stderr = out

	return sess.Run(cmd)
}

func (e *Executor) Upload(ctx context.Context, host, remotePath string, content []byte, mode uint32) error {
	client, err := e.dialWithRetry(ctx, host)
	if err != nil {
		return err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session on %s: %w", host, err)
	}
	defer sess.Close()

	go func() {
		w, _ := sess.StdinPipe()
		defer w.Close()
		fmt.Fprintf(w, "C%04o %d %s\n", mode, len(content), remotePath[strings.LastIndex(remotePath, "/")+1:])
		w.Write(content)
		fmt.Fprintf(w, "\x00")
	}()

	dir := remotePath[:strings.LastIndex(remotePath, "/")]
	return sess.Run(fmt.Sprintf("scp -qt %s", dir))
}

func (e *Executor) WaitSSH(ctx context.Context, host string) error {
	addr := fmt.Sprintf("%s:%d", host, e.port)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}
