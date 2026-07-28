package client

import (
	"context"
	"fmt"
	"time"
)

type TaskStatus struct {
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
	Type       string `json:"type"`
	ID         string `json:"id"`
}

func (c *Client) WaitTask(ctx context.Context, pveNode, upid string) error {
	pollInterval := 500 * time.Millisecond
	maxInterval := 3 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}

		status, err := c.TaskStatus(ctx, pveNode, upid)
		if err != nil {
			return fmt.Errorf("polling task %s: %w", upid, err)
		}

		if status.Status == "stopped" {
			if status.ExitStatus != "OK" {
				return fmt.Errorf("task %s failed: %s", upid, status.ExitStatus)
			}
			return nil
		}

		if pollInterval < maxInterval {
			pollInterval = pollInterval * 2
			if pollInterval > maxInterval {
				pollInterval = maxInterval
			}
		}
	}
}

func (c *Client) TaskStatus(ctx context.Context, pveNode, upid string) (*TaskStatus, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/tasks/%s/status", pveNode, upid)
	return get[*TaskStatus](ctx, c, path)
}

func (c *Client) Version(ctx context.Context) (map[string]interface{}, error) {
	return get[map[string]interface{}](ctx, c, "/api2/json/version")
}

func (c *Client) NextVMID(ctx context.Context, min, max int) (int, error) {
	path := "/api2/json/cluster/nextid"
	if min > 0 {
		path += fmt.Sprintf("?vmid=%d", min)
	}

	type vmidResp struct {
		Data int `json:"data"`
	}

	id, err := get[int](ctx, c, path)
	if err != nil {
		return 0, err
	}
	if max > 0 && id > max {
		return 0, fmt.Errorf("next available VMID %d is outside configured vmid_range [%d, %d]", id, min, max)
	}
	return id, nil
}
