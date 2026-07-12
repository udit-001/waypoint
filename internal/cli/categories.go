package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// categories is the parent command.
var categoriesCmd = &cobra.Command{
	Use:     "categories",
	Short:   "Manage job categories",
	Long:    `Create, list, rename, and delete job categories.\n\nCategories help organize your job applications into groups such as Tech, Finance, Healthcare, etc.`,
	Aliases: []string{"cat"},
}

// --- list ---

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all categories",
	Long: `List all categories with the number of jobs in each.

Examples:
  waypoint categories list
  waypoint categories list --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cats, err := store.GetCategoriesWithCounts()
		if err != nil {
			return formatError("failed to list categories", err)
		}

		if jsonOut {
			printJSON(cats)
			return nil
		}

		fmt.Println()
		if len(cats) == 0 {
			fmt.Println("  No categories found.")
			fmt.Println()
			return nil
		}

		rows := make([][]string, 0, len(cats))
		for _, c := range cats {
			rows = append(rows, []string{
				strconv.FormatInt(c.ID, 10),
				c.Name,
				strconv.Itoa(c.JobCount),
			})
		}

		fmt.Println(formatTable(
			[]string{"ID", "Name", "Jobs"},
			rows,
		))
		fmt.Println()
		return nil
	},
}

// --- add ---

var categoriesAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new category",
	Long: `Add a new job category.

Examples:
  waypoint categories add "Remote"
  waypoint categories add "Startups"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		has, err := store.HasCategory(name)
		if err != nil {
			return formatError("failed to check category", err)
		}
		if has {
			return fmt.Errorf("category %q already exists", name)
		}

		created, err := store.AddCategory(name)
		if err != nil {
			return formatError("failed to add category", err)
		}

		if jsonOut {
			printJSON(created)
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Category added: %s (ID: %d)\n", created.Name, created.ID)
		fmt.Println()
		return nil
	},
}

// --- rename ---

var categoriesRenameCmd = &cobra.Command{
	Use:   "rename <id> <new-name>",
	Short: "Rename a category",
	Long: `Rename a category by its ID. All jobs in the category keep their assignment.

Examples:
  waypoint categories rename 2 "Technology"
  waypoint categories rename 3 "Misc"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid category ID: %s", args[0])
		}
		newName := args[1]

		cat, err := store.GetCategoryByID(id)
		if err != nil {
			return err
		}

		if cat.Name == newName {
			return fmt.Errorf("name is already %q", newName)
		}

		has, err := store.HasCategory(newName)
		if err != nil {
			return formatError("failed to check category", err)
		}
		if has {
			return fmt.Errorf("category %q already exists", newName)
		}

		if err := store.RenameCategory(id, newName); err != nil {
			return formatError("failed to rename category", err)
		}

		if jsonOut {
			printJSON(map[string]string{"id": args[0], "renamed": cat.Name, "to": newName})
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Category renamed: %s → %s\n", cat.Name, newName)
		fmt.Println()
		return nil
	},
}

// --- delete ---

var categoriesDeleteFlags struct {
	force bool
}

var categoriesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a category",
	Long: `Delete a category by its ID. Prompts for confirmation
unless --force is used.

Jobs in the deleted category are moved to "Uncategorized".

Examples:
  waypoint categories delete 3
  waypoint categories delete 4 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid category ID: %s", args[0])
		}

		cat, err := store.GetCategoryByID(id)
		if err != nil {
			return err
		}

		count, _ := store.CategoryJobCount(id)

		if !categoriesDeleteFlags.force {
			fmt.Printf("  Delete category %q (%d jobs will move to Uncategorized)? [y/N]: ", cat.Name, count)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" && confirm != "yes" {
				fmt.Println("  Cancelled.")
				return nil
			}
		}

		if err := store.DeleteCategory(id); err != nil {
			return formatError("failed to delete category", err)
		}

		if jsonOut {
			printJSON(map[string]any{"deleted": true, "id": id, "category": cat.Name, "jobsMoved": count})
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Deleted category: %s\n", cat.Name)
		if count > 0 {
			fmt.Printf("    %d job(s) moved to Uncategorized\n", count)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(categoriesCmd)
	categoriesCmd.AddCommand(categoriesListCmd)
	categoriesCmd.AddCommand(categoriesAddCmd)
	categoriesCmd.AddCommand(categoriesRenameCmd)
	categoriesCmd.AddCommand(categoriesDeleteCmd)

	categoriesDeleteCmd.Flags().BoolVar(&categoriesDeleteFlags.force, "force", false, "Skip confirmation prompt")
}
