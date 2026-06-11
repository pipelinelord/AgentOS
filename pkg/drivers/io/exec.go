package io

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerDriver struct {
	cli   *client.Client
	image string
}

func NewDockerDriver() (*DockerDriver, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerDriver{
		cli:   cli,
		image: "alpine:latest", // Default fast image for POC
	}, nil
}

func (d *DockerDriver) Exec(ctx context.Context, cmd string) (string, string, int, error) {
	resp, err := d.cli.ContainerCreate(ctx, &container.Config{
		Image: d.image,
		Cmd:   []string{"sh", "-c", cmd},
		Tty:   false,
	}, nil, nil, nil, "")
	if err != nil {
		return "", "", -1, err
	}
	
	defer func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.cli.ContainerRemove(bgCtx, resp.ID, container.RemoveOptions{Force: true})
	}()

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", "", -1, err
	}

	statusCh, errCh := d.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	var exitCode int
	select {
	case err := <-errCh:
		if err != nil {
			return "", "", -1, err
		}
	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	}

	out, err := d.cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", "", -1, err
	}
	defer out.Close()

	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, out)
	if err != nil && err != io.EOF {
		return "", "", -1, err
	}

	return stdout.String(), stderr.String(), exitCode, nil
}
