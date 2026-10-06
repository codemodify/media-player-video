package player

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func videoExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v", ".mkv", ".webm", ".avi", ".mov", ".wmv", ".mpg", ".mpeg", ".m2v", ".m2ts", ".mts", ".ts", ".vob", ".ogv", ".flv", ".3gp", ".divx":
		return true
	}
	return false
}
func subtitleExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".srt", ".ass", ".ssa", ".vtt", ".sub":
		return true
	}
	return false
}

// Expand validates a whole request before committing. Folder scans are sorted,
// cancellable and do not follow nested directory symlinks.
func Expand(ctx context.Context, paths []string) ([]Video, error) {
	var videos []Video
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, fmt.Errorf("open video: %w", err)
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("%s is not a regular file", path)
			}
			videos = append(videos, Video{Path: absolute})
			continue
		}
		root, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !videoExtension(path) {
				return nil
			}
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				videos = append(videos, Video{Path: path})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan video folder: %w", err)
		}
	}
	if len(videos) == 0 {
		return nil, fmt.Errorf("no video files found")
	}
	return videos, nil
}
