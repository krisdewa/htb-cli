package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/GoToolSharing/htb-cli/config"
	"github.com/GoToolSharing/htb-cli/lib/utils"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	machineURL     = config.BaseHackTheBoxAPIURL + "/machine/paginated/?per_page=20"
	retiredURL     = config.BaseHackTheBoxAPIURL + "/machine/list/retired/paginated/?per_page=20&sort_by=release-date"
	scheduledURL   = config.BaseHackTheBoxAPIURL + "/machine/unreleased/"
	activeTitle    = "Active"
	retiredTitle   = "Retired"
	scheduledTitle = "Scheduled"
	CheckMark      = "\U00002705"
	CrossMark      = "\U0000274C"
	Penguin        = "\U0001F427"
	Computer       = "\U0001F5A5 "

	scheduledUnavailableMsg = "No scheduled machine data available (this API endpoint is currently disabled/renamed upstream)"
)

// stdoutIsTerminal reports whether stdout is attached to a real terminal.
// TUI rendering requires a TTY; without one we fall back to plain text.
func stdoutIsTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeCharDevice != 0
}

// printMachinesPlain renders the machine lists as plain text for non-TTY output.
func printMachinesPlain() {
	sections := []struct {
		url       string
		title     string
		scheduled bool
	}{
		{machineURL, activeTitle, false},
		{retiredURL, retiredTitle, false},
		{scheduledURL, scheduledTitle, true},
	}

	for _, section := range sections {
		resp, err := utils.HtbRequest(http.MethodGet, section.url, nil)
		fmt.Printf("\n== %s ==\n", section.title)
		if err != nil {
			fmt.Printf("  unavailable: %v\n", err)
			continue
		}

		info := utils.ParseJsonMessage(resp, "data")
		resp.Body.Close()

		items, ok := info.([]interface{})
		if !ok || len(items) == 0 {
			fmt.Printf("  %s\n", scheduledUnavailableMsg)
			continue
		}

		for _, value := range items {
			data, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			name := getStr(data, "name")
			osName := getStr(data, "os")
			difficulty := getStr(data, "difficultyText")
			if section.scheduled {
				difficulty = getStr(data, "difficulty_text")
				fmt.Printf("  %-20s %-8s %s\n", name, osName, difficulty)
				continue
			}

			userOwned, rootOwned := "  -", "  -"
			if getBool(data, "authUserInUserOwns") {
				userOwned = "user"
			}
			if getBool(data, "authUserInRootOwns") {
				rootOwned = "root"
			}
			release := getStr(data, "release")
			if parsed, err := time.Parse(time.RFC3339Nano, release); err == nil {
				release = parsed.Format("2006-01-02")
			}
			fmt.Printf("  %-20s %-8s %-8s user:%-4s root:%-4s %s\n",
				name, osName, difficulty, userOwned, rootOwned, release)
		}
	}
}

// getColorFromDifficultyText returns the color corresponding to the given difficulty.
func getColorFromDifficultyText(difficultyText string) string {
	switch difficultyText {
	case "Medium":
		return "[orange]"
	case "Easy":
		return "[green]"
	case "Hard":
		return "[red]"
	case "Insane":
		return "[purple]"
	default:
		return "[-]"
	}
}

// getOSEmoji returns an emoji corresponding to the given operating system
func getOSEmoji(os string) string {
	switch os {
	case "Linux":
		return Penguin
	case "Windows":
		return Computer
	default:
		return ""
	}
}

// getStr safely extracts a string field from a decoded JSON object.
func getStr(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

// getBool safely extracts a boolean field from a decoded JSON object.
func getBool(data map[string]interface{}, key string) bool {
	if value, ok := data[key].(bool); ok {
		return value
	}
	return false
}

// createFlex creates and returns a Flex view with machine information
func createFlex(info interface{}, title string, isScheduled bool) (*tview.Flex, error) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.SetBorder(true).SetTitle(title).SetTitleAlign(tview.AlignLeft)

	items, ok := info.([]interface{})
	if !ok || len(items) == 0 {
		flex.AddItem(tview.NewTextView().SetText(scheduledUnavailableMsg), 1, 0, false)
		return flex, nil
	}

	for _, value := range items {
		data, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		// Determining the color according to difficulty
		key := getStr(data, "difficultyText")
		if isScheduled {
			key = getStr(data, "difficulty_text")
		}
		color := getColorFromDifficultyText(key)
		osName := getStr(data, "os")
		osEmoji := getOSEmoji(osName)

		var formatString string

		// Choice of display format depending on the nature of the information
		if isScheduled {
			formatString = fmt.Sprintf("%-10s %s%-10s %s%-10s[-]",
				getStr(data, "name"), osEmoji, osName, color, getStr(data, "difficulty_text"))
		} else {
			// Convert and format date (tolerate unexpected/missing values)
			formattedDate := "unknown"
			if release := getStr(data, "release"); release != "" {
				if parsedDate, err := time.Parse(time.RFC3339Nano, release); err == nil {
					formattedDate = parsedDate.Format("02 January 2006")
				}
			}

			userEmoji := CrossMark + "User"
			if getBool(data, "authUserInUserOwns") {
				userEmoji = CheckMark + "User"
			}

			rootEmoji := CrossMark + "Root"
			if getBool(data, "authUserInRootOwns") {
				rootEmoji = CheckMark + "Root"
			}

			formatString = fmt.Sprintf("%-15s %s%-10s %s%-10s[-] %-5v %-5v %-7v %-30s",
				getStr(data, "name"), osEmoji, osName, color, getStr(data, "difficultyText"),
				data["star"], userEmoji, rootEmoji, formattedDate)
		}

		flex.AddItem(tview.NewTextView().SetText(formatString).SetDynamicColors(true), 1, 0, false)
	}

	return flex, nil
}

var machinesCmd = &cobra.Command{
	Use:   "machines",
	Short: "Displays active / retired machines and next machines to be released",
	Run: func(cmd *cobra.Command, args []string) {
		// The scheduled-machines endpoint has been retired upstream, so this
		// view must degrade gracefully instead of panicking on nil data.
		if !stdoutIsTerminal() {
			printMachinesPlain()
			return
		}

		app := tview.NewApplication()

		getAndDisplayFlex := func(url, title string, isScheduled bool, flex *tview.Flex) error {
			resp, err := utils.HtbRequest(http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("failed to get data from %s: %w", url, err)
			}

			info := utils.ParseJsonMessage(resp, "data")

			machineFlex, err := createFlex(info, title, isScheduled)
			if err != nil {
				return fmt.Errorf("failed to create flex for %s: %w", title, err)
			}

			flex.AddItem(machineFlex, 0, 1, false)
			return nil
		}

		leftFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		rightFlex := tview.NewFlex().SetDirection(tview.FlexRow)

		if err := getAndDisplayFlex(machineURL, activeTitle, false, leftFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		if err := getAndDisplayFlex(retiredURL, retiredTitle, false, leftFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		if err := getAndDisplayFlex(scheduledURL, scheduledTitle, true, rightFlex); err != nil {
			// Endpoint removed upstream -> show a notice instead of exiting.
			config.GlobalConfig.Logger.Warn("scheduled machines unavailable", zap.Error(err))
			rightFlex.AddItem(tview.NewTextView().SetText(scheduledUnavailableMsg), 1, 0, false)
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
	rootCmd.AddCommand(machinesCmd)
}
