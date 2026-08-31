package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Registry describe el conjunto de bóvedas registradas y cuál es la actual.
type Registry struct {
	Current string            `json:"current"`
	Vaults  map[string]string `json:"vaults"`
}

// ConfigDir devuelve el directorio de configuración de la aplicación
// (os.UserConfigDir()/asset).
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("no se pudo determinar el directorio de configuración: %w", err)
	}
	return filepath.Join(base, "asset"), nil
}

// RegistryPath devuelve la ruta del archivo de registro dentro de dir.
func RegistryPath(dir string) string {
	return filepath.Join(dir, "vaults.json")
}

// DefaultDBPath devuelve la ruta por defecto de la BD de una bóveda.
func DefaultDBPath(dir, name string) string {
	return filepath.Join(dir, "vaults", name, "assets.db")
}

// Load lee el registro de bóvedas desde dir. Si no existe, devuelve uno vacío.
func Load(dir string) (Registry, error) {
	r := Registry{Vaults: map[string]string{}}
	data, err := os.ReadFile(RegistryPath(dir))
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return r, fmt.Errorf("leyendo el registro de bóvedas: %w", err)
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("parseando el registro de bóvedas: %w", err)
	}
	if r.Vaults == nil {
		r.Vaults = map[string]string{}
	}
	return r, nil
}

// Save escribe el registro de bóvedas en dir.
func Save(dir string, r Registry) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creando directorio de configuración: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando el registro de bóvedas: %w", err)
	}
	if err := os.WriteFile(RegistryPath(dir), data, 0644); err != nil {
		return fmt.Errorf("escribiendo el registro de bóvedas: %w", err)
	}
	return nil
}
