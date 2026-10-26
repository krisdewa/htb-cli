package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/GoToolSharing/htb-cli/config"
	"github.com/GoToolSharing/htb-cli/lib/sherlocks"
	"github.com/GoToolSharing/htb-cli/lib/utils"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// printSherlocksPlain renders Sherlock lists as plain text for non-TTY output.
func printSherlocksPlain() {
	sections := []struct {
		url   string
		title string
	}{
		{sherlocks.SherlocksURL, sherlocks.ActiveSherlocksTitle},
		{sherlocks.RetiredSherlocksURL, sherlocks.RetiredSherlocksTitle},
		{sherlocks.ScheduledSherlocksURL, sherlocks.ScheduledSherlocksTitle},
	}

	for _, section := range sections {
		fmt.Printf("\n== %s ==\n", section.title)
		resp, err := utils.HtbRequest(http.MethodGet, section.url, nil)
		if err != nil {
			fmt.Printf("  unavailable: %v\n", err)
			continue
		}

		info := utils.ParseJsonMessage(resp, "data")
		resp.Body.Close()

		items, ok := info.([]interface{})
		if !ok || len(items) == 0 {
			fmt.Println("  (no data)")
			continue
		}

		for _, value := range items {
			data, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			owned := "  -"
			if getBool(data, "is_owned") {
				owned = "own"
			}
			fmt.Printf("  %-28s %-10s %-6s solves:%-6v rating:%-4v %s\n",
				getStr(data, "name"), getStr(data, "difficulty"), owned,
				data["solves"], data["rating"], getStr(data, "category_name"))
		}
	}
}

var sherlocksCmd = &cobra.Command{
	Use:   "sherlocks",
	Short: "Play Sherlocks mode (blue team)",
	Run: func(cmd *cobra.Command, args []string) {
		sherlockNameParam, err := cmd.Flags().GetString("sherlock_name")
		if err != nil {
			fmt.Println(err)
			return
		}

		sherlockDownloadPath, err := cmd.Flags().GetString("download")
		if err != nil {
			fmt.Println(err)
			return
		}

		sherlockTaskID, err := cmd.Flags().GetInt("task")
		if err != nil {
			fmt.Println(err)
			return
		}

		sherlockHint, err := cmd.Flags().GetBool("hint")
		if err != nil {
			fmt.Println(err)
			return
		}

		if sherlockNameParam != "" {
			sherlockID, err := sherlocks.SearchIDByName(sherlockNameParam)
			if err != nil {
				fmt.Println(err)
				return
			}
			config.GlobalConfig.Logger.Debug(fmt.Sprintf("SherlockID: %s", sherlockID))

			if sherlockTaskID != 0 {
				err := sherlocks.GetTaskByID(sherlockID, sherlockTaskID, sherlockHint)
				if err != nil {
					fmt.Println(err)
					return
				}
				return
			}

			err = sherlocks.GetGeneralInformations(sherlockID, sherlockDownloadPath)

			if err != nil {
				fmt.Println(err)
				return
			}

			data, err := sherlocks.GetTasks(sherlockID)
			if err != nil {
				fmt.Println(err)
				return
			}
			for _, task := range data.Tasks {
				if task.Completed {
					fmt.Printf("\n%s (DONE) :\n%s\n\n", task.Title, task.Description)
				} else {
					fmt.Printf("\n%s :\n%s\n\n", task.Title, task.Description)
				}
			}
			return
		}

		// Listing Sherlocks requires a TUI; without a TTY print plain text instead.
		if !stdoutIsTerminal() {
			printSherlocksPlain()
			return
		}

		app := tview.NewApplication()

		getAndDisplayFlex := func(url, title string, isScheduled bool, flex *tview.Flex) error {
			resp, err := utils.HtbRequest(http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("failed to get data from %s: %w", url, err)
			}

			info := utils.ParseJsonMessage(resp, "data")

			machineFlex, err := sherlocks.CreateFlex(info, title, isScheduled)
			if err != nil {
				return fmt.Errorf("failed to create flex for %s: %w", title, err)
			}

			flex.AddItem(machineFlex, 0, 1, false)
			return nil
		}

		leftFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		rightFlex := tview.NewFlex().SetDirection(tview.FlexRow)

		if err := getAndDisplayFlex(sherlocks.SherlocksURL, sherlocks.ActiveSherlocksTitle, false, leftFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		if err := getAndDisplayFlex(sherlocks.RetiredSherlocksURL, sherlocks.RetiredSherlocksTitle, false, leftFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		if err := getAndDisplayFlex(sherlocks.ScheduledSherlocksURL, sherlocks.ScheduledSherlocksTitle, true, rightFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		rightFlex.AddItem(tview.NewTextView().SetText("").SetDynamicColors(true), 0, 0, false)

		mainFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(leftFlex, 0, 3, false).
			AddItem(rightFlex, 0, 1, false)

		if err := app.SetRoot(mainFlex, true).Run(); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

	},
}

func init() {
	rootCmd.AddCommand(sherlocksCmd)
	sherlocksCmd.Flags().StringP("sherlock_name", "s", "", "Sherlock Name")
	sherlocksCmd.Flags().StringP("download", "d", "", "Download Sherlock Resources")
	sherlocksCmd.Flags().IntP("task", "t", 0, "Task ID")
	sherlocksCmd.Flags().BoolP("hint", "", false, "Hint")
}
