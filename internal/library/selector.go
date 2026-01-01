package library

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
)

type Selector struct {
	libraries []string
}

func NewSelector(libraries []string) *Selector {
	return &Selector{
		libraries: libraries,
	}
}

type SelectionResult struct {
	Library   string
	Reason    string
	Available int64
}

func (s *Selector) SelectMovieLibrary(movieTitle string, year string, fileSize int64) (*SelectionResult, error) {
	if len(s.libraries) == 0 {
		return nil, fmt.Errorf("no libraries configured")
	}

	if len(s.libraries) == 1 {
		lib := s.libraries[0]
		available, err := getAvailableSpace(lib)
		if err != nil {
			return nil, err
		}
		if available < fileSize {
			return nil, fmt.Errorf("insufficient space in %s: need %d, have %d", lib, fileSize, available)
		}
		return &SelectionResult{
			Library:   lib,
			Reason:    "Only library available",
			Available: available,
		}, nil
	}

	var candidates []SelectionResult
	for _, lib := range s.libraries {
		available, err := getAvailableSpace(lib)
		if err != nil || available < fileSize {
			continue
		}

		hasExisting, franchiseCount := s.findRelatedContent(lib, movieTitle, year, true)
		var reasons []string
		if hasExisting {
			reasons = append(reasons, fmt.Sprintf("Contains %d related items", franchiseCount))
		}

		spaceInGB := available / (1024 * 1024 * 1024)
		reasons = append(reasons, fmt.Sprintf("%d GB available", spaceInGB))

		candidates = append(candidates, SelectionResult{
			Library:   lib,
			Available: available,
			Reason:    strings.Join(reasons, ", "),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable libraries found with sufficient space")
	}

	var best *SelectionResult
	for i := range candidates {
		if best == nil || s.scoreLibrary(candidates[i].Library, movieTitle, year, fileSize, true) >
			s.scoreLibrary(best.Library, movieTitle, year, fileSize, true) {
			best = &candidates[i]
		}
	}

	return best, nil
}

func (s *Selector) SelectTVShowLibrary(showName string, year string, fileSize int64) (*SelectionResult, error) {
	if len(s.libraries) == 0 {
		return nil, fmt.Errorf("no libraries configured")
	}

	if len(s.libraries) == 1 {
		lib := s.libraries[0]
		available, err := getAvailableSpace(lib)
		if err != nil {
			return nil, err
		}
		if available < fileSize {
			return nil, fmt.Errorf("insufficient space in %s: need %d, have %d", lib, fileSize, available)
		}
		return &SelectionResult{
			Library:   lib,
			Reason:    "Only library available",
			Available: available,
		}, nil
	}

	var candidates []SelectionResult
	for _, lib := range s.libraries {
		available, err := getAvailableSpace(lib)
		if err != nil || available < fileSize {
			continue
		}

		hasExisting, episodeCount := s.findRelatedContent(lib, showName, year, false)
		var reasons []string
		if hasExisting {
			reasons = append(reasons, fmt.Sprintf("Contains %d episodes", episodeCount))
		}

		spaceInGB := available / (1024 * 1024 * 1024)
		reasons = append(reasons, fmt.Sprintf("%d GB available", spaceInGB))

		candidates = append(candidates, SelectionResult{
			Library:   lib,
			Available: available,
			Reason:    strings.Join(reasons, ", "),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable libraries found with sufficient space")
	}

	var best *SelectionResult
	for i := range candidates {
		if best == nil || s.scoreLibrary(candidates[i].Library, showName, year, fileSize, false) >
			s.scoreLibrary(best.Library, showName, year, fileSize, false) {
			best = &candidates[i]
		}
	}

	return best, nil
}

func (s *Selector) scoreLibrary(library, title, year string, fileSize int64, isMovie bool) int {
	score := 0

	hasExisting, itemCount := s.findRelatedContent(library, title, year, isMovie)
	if hasExisting {
		score += 1000 + itemCount*100
	}

	available, _ := getAvailableSpace(library)
	spaceScore := int(available / (1024 * 1024 * 1024))
	if spaceScore > 100 {
		spaceScore = 100
	}
	score += spaceScore

	itemCount = s.countMediaItems(library, isMovie)
	score += min(itemCount/10, 50)

	return score
}

func (s *Selector) findRelatedContent(library, title, year string, isMovie bool) (hasContent bool, itemCount int) {
	normalizedTitle := strings.ToLower(strings.ReplaceAll(title, " ", ""))

	_ = filepath.Walk(library, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			dirName := strings.ToLower(info.Name())
			dirName = strings.ReplaceAll(dirName, " ", "")
			dirName = strings.ReplaceAll(dirName, "(", "")
			dirName = strings.ReplaceAll(dirName, ")", "")

			if strings.Contains(dirName, normalizedTitle) {
				itemCount++
				hasContent = true
			}
		}

		return nil
	})

	return hasContent, itemCount
}

func (s *Selector) countMediaItems(library string, isMovie bool) int {
	count := 0

	if isMovie {
		_ = filepath.Walk(library, func(path string, info fs.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".mkv" || ext == ".mp4" || ext == ".avi" {
					if strings.Contains(info.Name(), "(") && strings.Contains(info.Name(), ")") {
						count++
					}
				}
			}
			return nil
		})
	} else {
		_ = filepath.Walk(library, func(path string, info fs.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".mkv" || ext == ".mp4" || ext == ".avi" {
					if strings.Contains(info.Name(), "S") && strings.Contains(info.Name(), "E") {
						count++
					}
				}
			}
			return nil
		})
	}

	return count
}

func getAvailableSpace(path string) (int64, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(path, &stat)
	if err != nil {
		current := path
		for {
			dir := filepath.Dir(current)
			if dir == current {
				break
			}
			current = dir
			err = syscall.Statfs(current, &stat)
			if err == nil {
				break
			}
		}
		if err != nil {
			return 0, fmt.Errorf("unable to get disk space for %s: %w", path, err)
		}
	}

	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
