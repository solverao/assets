package asset

import (
	"database/sql"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"asset/internal/checksum"
	"asset/internal/normalize"
)

// ScannerService orquesta el escaneo de un árbol de directorios y su
// indexado en asset_blobs + assets.
type ScannerService struct {
	repo Repository
}

// NewScannerService inyecta el repositorio.
func NewScannerService(repo Repository) *ScannerService {
	return &ScannerService{repo: repo}
}

// ScanDirectory recorre root creando el árbol de assets (carpetas y
// archivos), deduplicando blobs por checksum.
func (s *ScannerService) ScanDirectory(root string, userID string, workers int) error {
	fmt.Printf("Escaneando directorio %s...\n", root)

	if err := s.repo.EnsureUser(userID); err != nil {
		return fmt.Errorf("asegurando usuario %s: %w", userID, err)
	}

	dirAssets := make(map[string]int64)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.Base(path)

		parentID := sql.NullInt64{}
		if parent := filepath.Dir(path); parent != root {
			if id, ok := dirAssets[parent]; ok {
				parentID = sql.NullInt64{Int64: id, Valid: true}
			}
		}

		if d.IsDir() {
			slug := normalize.Slugify(name, true)
			if exists, _ := s.repo.AssetExists(userID, slug); exists {
				return filepath.SkipDir
			}
			id, err := s.repo.CreateAsset(Asset{
				UserID:   userID,
				ParentID: parentID,
				Type:     "folder",
				Slug:     slug,
				Name:     name,
			})
			if err != nil {
				return fmt.Errorf("creando carpeta %s: %w", rel, err)
			}
			dirAssets[path] = id
			return nil
		}

		slug := normalize.Slugify(name, false)
		if exists, _ := s.repo.AssetExists(userID, slug); exists {
			fmt.Printf("Saltando %s (ya existe)\n", rel)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		sum, err := checksum.CalculateChecksum(path)
		if err != nil {
			return fmt.Errorf("checksum de %s: %w", rel, err)
		}
		ext := filepath.Ext(name)
		blobID, err := s.repo.UpsertBlob(Blob{
			Path:         path,
			Size:         info.Size(),
			Checksum:     sum,
			MimeType:     mime.TypeByExtension(ext),
			Extension:    ext,
			OriginalName: name,
		})
		if err != nil {
			return fmt.Errorf("guardando blob de %s: %w", rel, err)
		}

		if _, err := s.repo.CreateAsset(Asset{
			UserID:      userID,
			ParentID:    parentID,
			AssetBlobID: sql.NullInt64{Int64: blobID, Valid: true},
			Type:        "file",
			Slug:        slug,
			Name:        name,
		}); err != nil {
			return fmt.Errorf("creando asset de %s: %w", rel, err)
		}
		return nil
	})

	if err != nil {
		return err
	}
	fmt.Println("Escaneo completado.")
	return nil
}
