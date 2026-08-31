package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"asset/internal/database"
	"asset/internal/vault"

	"github.com/spf13/cobra"
)

var vaultNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func NewVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Gestiona bóvedas (bases de datos con nombre)",
	}

	cmd.AddCommand(newVaultListCmd())
	cmd.AddCommand(newVaultCreateCmd())
	cmd.AddCommand(newVaultUseCmd())
	cmd.AddCommand(newVaultCurrentCmd())
	cmd.AddCommand(newVaultDeleteCmd())
	cmd.AddCommand(newVaultPathCmd())

	return cmd
}

func vaultDir() (string, error) {
	return vault.ConfigDir()
}

func newVaultListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista las bóvedas registradas",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			r, err := vault.Load(dir)
			if err != nil {
				return err
			}
			if len(r.Vaults) == 0 {
				fmt.Println("No hay bóvedas registradas.")
				return nil
			}
			for name, path := range r.Vaults {
				marker := " "
				if name == r.Current {
					marker = "*"
				}
				fmt.Printf("%s %s\t%s\n", marker, name, path)
			}
			return nil
		},
	}
}

func newVaultCreateCmd() *cobra.Command {
	var pathDir string

	c := &cobra.Command{
		Use:   "create <nombre>",
		Short: "Crea una bóveda y la registra",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !vaultNameRe.MatchString(name) {
				return fmt.Errorf("nombre de bóveda inválido: %s", name)
			}
			dir, err := vaultDir()
			if err != nil {
				return err
			}

			r, err := vault.Load(dir)
			if err != nil {
				return err
			}
			if _, exists := r.Vaults[name]; exists {
				return fmt.Errorf("la bóveda %q ya existe", name)
			}

			dbPath := vault.DefaultDBPath(dir, name)
			if pathDir != "" {
				dbPath = filepath.Join(pathDir, name, "assets.db")
			}
			if err := database.Create(dbPath); err != nil {
				return err
			}

			r.Vaults[name] = dbPath
			if r.Current == "" {
				r.Current = name
			}
			if err := vault.Save(dir, r); err != nil {
				return err
			}
			fmt.Printf("Bóveda %q creada en %s\n", name, dbPath)
			return nil
		},
	}

	c.Flags().StringVar(&pathDir, "path", "", "Directorio base de la bóveda (por defecto, el de configuración)")

	return c
}

func newVaultUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <nombre>",
		Short: "Fija la bóveda actual",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			r, err := vault.Load(dir)
			if err != nil {
				return err
			}
			if _, exists := r.Vaults[name]; !exists {
				return fmt.Errorf("la bóveda %q no existe", name)
			}
			r.Current = name
			if err := vault.Save(dir, r); err != nil {
				return err
			}
			fmt.Printf("Bóveda actual: %s\n", name)
			return nil
		},
	}
}

func newVaultCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Muestra la bóveda actual",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			r, err := vault.Load(dir)
			if err != nil {
				return err
			}
			if r.Current == "" {
				fmt.Println("No hay bóveda actual.")
				return nil
			}
			fmt.Printf("%s\t%s\n", r.Current, r.Vaults[r.Current])
			return nil
		},
	}
}

func newVaultDeleteCmd() *cobra.Command {
	var yes, files bool

	c := &cobra.Command{
		Use:   "delete <nombre>",
		Short: "Desregistra una bóveda (y opcionalmente borra sus archivos)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			r, err := vault.Load(dir)
			if err != nil {
				return err
			}
			path, exists := r.Vaults[name]
			if !exists {
				return fmt.Errorf("la bóveda %q no existe", name)
			}

			if !yes {
				fmt.Printf("¿Desregistrar la bóveda %q? [s/N] ", name)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "s" && answer != "si" && answer != "y" && answer != "yes" {
					fmt.Println("Operación cancelada.")
					return nil
				}
			}

			if files {
				if err := database.Delete(path); err != nil {
					return err
				}
			}

			delete(r.Vaults, name)
			if r.Current == name {
				r.Current = ""
			}
			if err := vault.Save(dir, r); err != nil {
				return err
			}
			fmt.Printf("Bóveda %q desregistrada.\n", name)
			return nil
		},
	}

	c.Flags().BoolVar(&yes, "yes", false, "No pedir confirmación")
	c.Flags().BoolVar(&files, "files", false, "Borrar también los archivos de la base de datos")

	return c
}

func newVaultPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <nombre>",
		Short: "Muestra la ruta de una bóveda",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			r, err := vault.Load(dir)
			if err != nil {
				return err
			}
			path, exists := r.Vaults[args[0]]
			if !exists {
				return fmt.Errorf("la bóveda %q no existe", args[0])
			}
			fmt.Println(path)
			return nil
		},
	}
}
