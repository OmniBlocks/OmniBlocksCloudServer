package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// this appends to a "current.log" file inside a directory and compresses old logs.
type rotatingFile struct {
	mu         sync.Mutex
	dir        string
	maxSize    int64
	maxBackups int

	f    *os.File
	size int64
}

func newRotatingFile(dir string, maxSizeMB, maxBackups int) (*rotatingFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory %q: %w", dir, err)
	}
	rf := &rotatingFile{
		dir:        dir,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
	}
	if err := rf.open(); err != nil {
		return nil, err
	}
	return rf, nil
}

func (rf *rotatingFile) currentPath() string {
	return filepath.Join(rf.dir, "current.log")
}

func (rf *rotatingFile) open() error {
	f, err := os.OpenFile(rf.currentPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %q: %w", rf.currentPath(), err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	rf.f = f
	rf.size = info.Size()
	return nil
}

func (rf *rotatingFile) Write(p []byte) (int, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.maxSize > 0 && rf.size > 0 && rf.size+int64(len(p)) > rf.maxSize {
		if err := rf.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := rf.f.Write(p)
	rf.size += int64(n)
	return n, err
}

// rotate closes the current file, compresses it into a timestamped backup,
// prunes old backups beyond maxBackups, and opens a fresh current.log.
func (rf *rotatingFile) rotate() error {
	if err := rf.f.Close(); err != nil {
		return err
	}

	backupName := fmt.Sprintf("log-%s.gz", time.Now().Format("20060102-150405.000000"))
	if err := compressFile(rf.currentPath(), filepath.Join(rf.dir, backupName)); err != nil {
		return err
	}
	if err := os.Remove(rf.currentPath()); err != nil {
		return err
	}
	if err := rf.pruneBackups(); err != nil {
		return err
	}

	return rf.open()
}

func compressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	return gz.Close()
}

// pruneBackups deletes the oldest compressed backups until at most
// maxBackups remain. maxBackups <= 0 means unlimited.
func (rf *rotatingFile) pruneBackups() error {
	if rf.maxBackups <= 0 {
		return nil
	}

	entries, err := os.ReadDir(rf.dir)
	if err != nil {
		return err
	}

	var backups []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, "log-") && strings.HasSuffix(name, ".gz") {
			backups = append(backups, name)
		}
	}
	sort.Strings(backups) // timestamped names sort chronologically

	for len(backups) > rf.maxBackups {
		if err := os.Remove(filepath.Join(rf.dir, backups[0])); err != nil {
			return err
		}
		backups = backups[1:]
	}
	return nil
}

func (rf *rotatingFile) Close() error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.f.Close()
}
