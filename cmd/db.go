package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"asset/internal/database"

	"github.com/spf13/cobra"
)

func NewDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Gestiona la base de datos SQLite",
	}

	cmd.AddCommand(newDBInitCmd())
	cmd.AddCommand(newDBInfoCmd())
	cmd.AddCommand(newDBMigrateCmd())
	cmd.AddCommand(newDBListCmd())
	cmd.AddCommand(newDBDeleteCmd())

	return cmd
}

func newDBInitCmd() *cobra.Command {
	var path string

	c := &cobra.Command{
		Use:     "init",
		Aliases: []string{"create"},
		Short:   "Crea la base de datos y aplica las migraciones",
		RunE: func(cmd *cobra.Command, args []string) error {
			target := path
			if target == "" {
				target = dbPath
			}
			if err := database.Create(target); err != nil {
				return err
			}
			fmt.Printf("Base de datos creada en %s\n", target)
			return nil
		},
	}

	c.Flags().StringVar(&path, "path", "", "Ruta de la base de datos (por defecto, --db)")

	return c
}

func newDBInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Muestra el estado de la base de datos",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := database.Info(dbPath)
			if err != nil {
				return err
			}
			fmt.Printf("Ruta:        %s\n", info.Path)
			if !info.Exists {
				fmt.Println("Estado:      no existe")
				return nil
			}
			fmt.Printf("Tamaño:      %d bytes\n", info.Size)
			fmt.Printf("Migraciones: %d\n", info.Migrations)
			fmt.Printf("Assets:      %d\n", info.Assets)
			fmt.Printf("Blobs:       %d\n", info.Blobs)
			return nil
		},
	}
}

func newDBMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Aplica las migraciones pendientes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := database.Create(dbPath); err != nil {
				return err
			}
			fmt.Println("Migraciones aplicadas.")
			return nil
		},
	}
}

func newDBListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "migrations",
		Aliases: []string{"list"},
		Short:   "Lista las migraciones aplicadas",
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := database.ListMigrations(dbPath)
			if err != nil {
				return err
			}
			if len(versions) == 0 {
				fmt.Println("No hay migraciones aplicadas.")
				return nil
			}
			for _, v := range versions {
				fmt.Println(v)
			}
			return nil
		},
	}
}

func newDBDeleteCmd() *cobra.Command {
	var yes bool

	c := &cobra.Command{
		Use:   "delete",
		Short: "Borra la base de datos y sus archivos auxiliares",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				fmt.Printf("¿Borrar la base de datos %s? [s/N] ", dbPath)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "s" && answer != "si" && answer != "y" && answer != "yes" {
					fmt.Println("Operación cancelada.")
					return nil
				}
			}
			if err := database.Delete(dbPath); err != nil {
				return err
			}
			fmt.Printf("Base de datos %s borrada.\n", dbPath)
			return nil
		},
	}

	c.Flags().BoolVar(&yes, "yes", false, "Borrar sin pedir confirmación")

	return c
}
