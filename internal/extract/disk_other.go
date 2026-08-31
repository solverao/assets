//go:build !linux

package extract

import "fmt"

// isRotational no está disponible fuera de Linux; se devuelve error para que
// resolveExtractWorkers use el fallback conservador.
func isRotational(dir string) (bool, error) {
	return false, fmt.Errorf("detección de disco rotacional no disponible en esta plataforma")
}
