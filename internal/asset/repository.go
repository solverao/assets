package asset

import (
	"database/sql"
)

// Blob representa un archivo físico (asset_blobs).
type Blob struct {
	Path         string
	Size         int64
	Checksum     string
	MimeType     string
	Extension    string
	OriginalName string
}

// Asset representa una entrada lógica del árbol (assets).
type Asset struct {
	UserID      string
	ParentID    sql.NullInt64
	AssetBlobID sql.NullInt64
	Type        string
	Slug        string
	Name        string
}

// Repository define las operaciones de base de datos del escáner.
type Repository interface {
	EnsureUser(userID string) error
	UpsertBlob(b Blob) (int64, error)
	CreateAsset(a Asset) (int64, error)
	AssetExists(userID, slug string) (bool, error)
}

// sqliteRepo es la implementación concreta.
type sqliteRepo struct {
	db *sql.DB
}

// NewSQLiteRepo retorna el struct, que implementa Repository implícitamente.
func NewSQLiteRepo(db *sql.DB) *sqliteRepo {
	return &sqliteRepo{db: db}
}

func (r *sqliteRepo) EnsureUser(userID string) error {
	_, err := r.db.Exec(`INSERT INTO user (id, name, email) VALUES (?, ?, ?)
		ON CONFLICT(id) DO NOTHING`, userID, userID, userID+"@local")
	return err
}

func (r *sqliteRepo) UpsertBlob(b Blob) (int64, error) {
	_, err := r.db.Exec(`INSERT INTO asset_blobs (path, size, checksum, mime_type, extension, original_name)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(checksum) DO NOTHING`,
		b.Path, b.Size, b.Checksum, b.MimeType, b.Extension, b.OriginalName)
	if err != nil {
		return 0, err
	}

	var id int64
	err = r.db.QueryRow(`SELECT id FROM asset_blobs WHERE checksum = ?`, b.Checksum).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *sqliteRepo) CreateAsset(a Asset) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO assets (user_id, parent_id, asset_blob_id, type, slug, name)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.UserID, a.ParentID, a.AssetBlobID, a.Type, a.Slug, a.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *sqliteRepo) AssetExists(userID, slug string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM assets WHERE user_id = ? AND slug = ?)`,
		userID, slug).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
