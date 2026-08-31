package normalize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// NormalizerService encapsula la lógica de normalización de nombres.
type NormalizerService struct{}

func NewNormalizerService() *NormalizerService {
	return &NormalizerService{}
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeAll renombra recursivamente archivos y carpetas a slugs.
func (s *NormalizerService) NormalizeAll(targetDir string, dryRun bool) error {
	var paths []string

	err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != filepath.Clean(targetDir) {
			paths = append(paths, path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(paths[i], string(os.PathSeparator)) > strings.Count(paths[j], string(os.PathSeparator))
	})

	count := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		isDir := info.IsDir()
		dir := filepath.Dir(path)
		oldName := filepath.Base(path)
		newName := Slugify(oldName, isDir)

		if oldName == newName {
			continue
		}

		finalPath := filepath.Join(dir, newName)
		counter := 1
		for {
			if _, err := os.Stat(finalPath); os.IsNotExist(err) {
				break
			}
			if isDir {
				finalPath = filepath.Join(dir, fmt.Sprintf("%s-%d", newName, counter))
			} else {
				ext := filepath.Ext(newName)
				base := strings.TrimSuffix(newName, ext)
				finalPath = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, counter, ext))
			}
			counter++
		}

		if dryRun {
			fmt.Printf("Renombraría %s -> %s\n", path, finalPath)
			continue
		}

		if err := os.Rename(path, finalPath); err == nil {
			count++
		}
	}
	fmt.Printf("Normalización completada. %d elementos renombrados.\n", count)
	return nil
}

// Slugify convierte un nombre en slug normalizado.
func Slugify(name string, isDir bool) string {
	var ext, base string
	if isDir {
		base = name
	} else {
		ext = strings.ToLower(filepath.Ext(name))
		base = strings.TrimSuffix(name, filepath.Ext(name))
	}
	slug := strings.ToLower(base)
	slug = slugRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "item"
	}
	return slug + ext
}
