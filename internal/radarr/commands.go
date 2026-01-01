package radarr

import (
	"fmt"
	"time"
)

func (c *Client) ExecuteCommand(cmd Command) (*CommandResponse, error) {
	var response CommandResponse
	if err := c.post("/api/v3/command", cmd, &response); err != nil {
		return nil, fmt.Errorf("executing command %s: %w", cmd.Name, err)
	}
	return &response, nil
}

func (c *Client) TriggerDownloadedMoviesScan(path string) (*CommandResponse, error) {
	cmd := Command{
		Name:       "DownloadedMoviesScan",
		Path:       path,
		ImportMode: "Move",
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) TriggerDownloadedMoviesScanCopy(path string) (*CommandResponse, error) {
	cmd := Command{
		Name:       "DownloadedMoviesScan",
		Path:       path,
		ImportMode: "Copy",
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) RefreshMovie(movieID int) (*CommandResponse, error) {
	cmd := Command{
		Name:    "RefreshMovie",
		MovieID: movieID,
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) RefreshAllMovies() (*CommandResponse, error) {
	cmd := Command{
		Name: "RefreshMovie",
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) RescanMovie(movieID int) (*CommandResponse, error) {
	cmd := Command{
		Name:    "RescanMovie",
		MovieID: movieID,
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) RenameMovie(movieID int) (*CommandResponse, error) {
	cmd := Command{
		Name:     "RenameMovie",
		MovieIDs: []int{movieID},
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) RenameFiles(fileIDs []int) (*CommandResponse, error) {
	cmd := Command{
		Name:  "RenameFiles",
		Files: fileIDs,
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) MovieSearch(movieID int) (*CommandResponse, error) {
	cmd := Command{
		Name:     "MoviesSearch",
		MovieIDs: []int{movieID},
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) RssSync() (*CommandResponse, error) {
	cmd := Command{
		Name: "RssSync",
	}
	return c.ExecuteCommand(cmd)
}

func (c *Client) GetCommandStatus(commandID int) (*CommandResponse, error) {
	endpoint := fmt.Sprintf("/api/v3/command/%d", commandID)
	var response CommandResponse
	if err := c.get(endpoint, &response); err != nil {
		return nil, fmt.Errorf("getting command status %d: %w", commandID, err)
	}
	return &response, nil
}

func (c *Client) WaitForCommand(commandID int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("command %d timed out after %s", commandID, timeout)
			}

			status, err := c.GetCommandStatus(commandID)
			if err != nil {
				return fmt.Errorf("checking command status: %w", err)
			}

			switch status.Status {
			case "completed":
				return nil
			case "failed":
				return fmt.Errorf("command failed: %s", status.Message)
			case "aborted":
				return fmt.Errorf("command was aborted")
			}
		}
	}
}
