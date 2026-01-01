package transfer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type DiskHealth struct {
	Path       string
	Accessible bool
	Writable   bool
	SpaceFree  int64
	SpaceTotal int64
	IOErrors   bool
	MountOK    bool
	Error      error
}

func (h *DiskHealth) IsHealthy() bool {
	return h.Accessible && h.Writable && h.MountOK && !h.IOErrors && h.Error == nil
}

func CheckDiskHealth(path string, timeout time.Duration) (*DiskHealth, error) {
	health := &DiskHealth{Path: path}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type statResult struct {
		info os.FileInfo
		err  error
	}
	statCh := make(chan statResult, 1)

	go func() {
		info, err := os.Stat(path)
		statCh <- statResult{info: info, err: err}
	}()

	select {
	case res := <-statCh:
		if res.err != nil {
			health.Error = fmt.Errorf("stat failed: %w", res.err)
			return health, health.Error
		}
		health.Accessible = true
		health.MountOK = res.info.IsDir()
	case <-ctx.Done():
		health.Error = fmt.Errorf("stat timed out after %s (possible I/O hang)", timeout)
		return health, health.Error
	}

	if !health.Accessible {
		return health, nil
	}

	var statfs syscall.Statfs_t
	statfsCh := make(chan error, 1)

	go func() {
		statfsCh <- syscall.Statfs(path, &statfs)
	}()

	select {
	case err := <-statfsCh:
		if err != nil {
			health.Error = fmt.Errorf("statfs failed: %w", err)
		} else {
			health.SpaceFree = int64(statfs.Bavail) * int64(statfs.Bsize)
			health.SpaceTotal = int64(statfs.Blocks) * int64(statfs.Bsize)
		}
	case <-ctx.Done():
		health.Error = fmt.Errorf("statfs timed out after %s", timeout)
		return health, health.Error
	}

	writeCh := make(chan error, 1)
	testFile := filepath.Join(path, fmt.Sprintf(".jellywatch_health_check_%d", time.Now().UnixNano()))

	go func() {
		f, err := os.Create(testFile)
		if err != nil {
			writeCh <- err
			return
		}
		_, err = f.WriteString("health check")
		f.Close()
		os.Remove(testFile)
		writeCh <- err
	}()

	select {
	case err := <-writeCh:
		health.Writable = (err == nil)
		if err != nil && health.Error == nil {
			health.Error = fmt.Errorf("write test failed: %w", err)
		}
	case <-ctx.Done():
		health.Error = fmt.Errorf("write test timed out after %s (possible I/O hang)", timeout)
		return health, health.Error
	}

	return health, nil
}

func CheckDiskHealthForTransfer(src, dst string, timeout time.Duration, requiredSpace int64) error {
	srcHealth, err := CheckDiskHealth(filepath.Dir(src), timeout)
	if err != nil {
		return fmt.Errorf("source disk unhealthy: %w", err)
	}
	if !srcHealth.Accessible {
		return fmt.Errorf("source path not accessible: %s", src)
	}

	dstDir := filepath.Dir(dst)
	dstHealth, err := CheckDiskHealth(dstDir, timeout)
	if err != nil {
		return fmt.Errorf("destination disk unhealthy: %w", err)
	}
	if !dstHealth.IsHealthy() {
		return fmt.Errorf("destination disk not healthy: %s (error: %v)", dstDir, dstHealth.Error)
	}

	if requiredSpace > 0 && dstHealth.SpaceFree < requiredSpace {
		return fmt.Errorf("insufficient space: need %d bytes, have %d bytes",
			requiredSpace, dstHealth.SpaceFree)
	}

	return nil
}

func StatWithTimeout(path string, timeout time.Duration) (os.FileInfo, error) {
	type result struct {
		info os.FileInfo
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		info, err := os.Stat(path)
		ch <- result{info: info, err: err}
	}()

	select {
	case res := <-ch:
		return res.info, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("stat timed out after %s for path: %s", timeout, path)
	}
}

func OpenWithTimeout(path string, flag int, perm os.FileMode, timeout time.Duration) (*os.File, error) {
	type result struct {
		file *os.File
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		f, err := os.OpenFile(path, flag, perm)
		ch <- result{file: f, err: err}
	}()

	select {
	case res := <-ch:
		return res.file, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("open timed out after %s for path: %s", timeout, path)
	}
}

func RemoveWithTimeout(path string, timeout time.Duration) error {
	ch := make(chan error, 1)

	go func() {
		ch <- os.Remove(path)
	}()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("remove timed out after %s for path: %s", timeout, path)
	}
}
