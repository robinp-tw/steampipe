package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/karrick/gows"
	"github.com/turbot/steampipe/control/controldisplay"
	"github.com/turbot/steampipe/control/execute"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/steampipe-plugin-sdk/logging"
	"github.com/turbot/steampipe/cmdconfig"
	"github.com/turbot/steampipe/constants"
	"github.com/turbot/steampipe/db"
	"github.com/turbot/steampipe/utils"
	"github.com/turbot/steampipe/workspace"
)

// CheckCmd :: represents the check command
func CheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:              "check",
		TraverseChildren: true,
		Args:             cobra.ArbitraryArgs,
		Run:              runCheckCmd,
		Short:            "Execute one or more controls",
		Long:             `Execute one or more controls."`,
	}

	cmdconfig.
		OnCmd(cmd).
		AddBoolFlag(constants.ArgHeader, "", true, "Include column headers csv and table output").
		AddStringFlag(constants.ArgSeparator, "", ",", "Separator string for csv output").
		AddStringFlag(constants.ArgOutput, "", "table", "Output format: line, csv, json or table").
		AddBoolFlag(constants.ArgTimer, "", false, "Turn on the timer which reports check time.").
		AddBoolFlag(constants.ArgWatch, "", true, "Watch SQL files in the current workspace (works only in interactive mode)").
		AddStringSliceFlag(constants.ArgSearchPath, "", []string{}, "Set a custom search_path for the steampipe user for a check session (comma-separated)").
		AddStringSliceFlag(constants.ArgSearchPathPrefix, "", []string{}, "Set a prefix to the current search path for a check session (comma-separated)").
		AddStringFlag(constants.ArgWhere, "", "", "SQL 'where' clause , or named query, used to filter controls ")

	return cmd
}

func runCheckCmd(cmd *cobra.Command, args []string) {
	logging.LogTime("runCheckCmd start")
	cmdconfig.Viper().Set(constants.ConfigKeyShowInteractiveOutput, false)

	defer func() {
		logging.LogTime("runCheckCmd end")
		if r := recover(); r != nil {
			utils.ShowError(helpers.ToError(r))
		}
	}()

	ctx, _ := context.WithCancel(context.Background())

	// start db if necessary
	err := db.EnsureDbAndStartService(db.InvokerCheck)
	utils.FailOnErrorWithMessage(err, "failed to start service")
	defer db.Shutdown(nil, db.InvokerCheck)

	// load the workspace
	workspace, err := workspace.Load(viper.GetString(constants.ArgWorkspace))
	utils.FailOnErrorWithMessage(err, "failed to load workspace")
	defer workspace.Close()

	// first get a client - do this once for all controls
	client, err := db.NewClient(true)
	utils.FailOnError(err)
	defer client.Close()

	// populate the reflection tables
	err = db.CreateMetadataTables(workspace.GetResourceMaps(), client)
	utils.FailOnError(err)

	// treat each arg as a separate execution
	failures := 0
	for _, arg := range args {
		executionTree, err := execute.NewExecutionTree(ctx, workspace, client, arg)
		utils.FailOnErrorWithMessage(err, "failed to resolve controls from argument")

		// for now we execute controls syncronously
		// Execute returns the number of failures
		executionTree.Execute(ctx, client)

		//bytes, err := json.MarshalIndent(executionTree.Root, "", "  ")

		DisplayControlResults(executionTree)
	}

	// set global exit code
	exitCode = failures
}

func DisplayControlResults(executionTree *execute.ExecutionTree) {

	maxCols, _, _ := gows.GetWinSize()
	renderer := controldisplay.NewTableRenderer(executionTree, maxCols)
	fmt.Println(renderer.Render())
	//
	//fmt.Println()
	//// NOTE: for now we can assume all results are complete
	//// todo summary and hierarchy
	//for _, res := range executionTree.Root.ControlRuns {
	//	fmt.Println()
	//	fmt.Printf("%s [%s]\n", typeHelpers.SafeString(res.Control.Title), res.Control.ShortName)
	//	if res.Error != nil {
	//		fmt.Printf("  Execution error: %v\n", res.Error)
	//		continue
	//	}
	//	for _, item := range res.Result.Rows {
	//		if item == nil {
	//			// should never happen!
	//			panic("NIL RESULT")
	//		}
	//		resString := fmt.Sprintf("  [%s] [%s] %s", item.Status, item.Resource, item.Reason)
	//		dimensionString := getDimension(item)
	//		fmt.Printf("%s %s\n", resString, dimensionString)
	//
	//	}
	//}
	//fmt.Println()
}

func getDimension(item *execute.ResultRow) string {
	var dimensions []string

	for _, v := range item.Dimensions {
		dimensions = append(dimensions, v)
	}

	return strings.Join(dimensions, "  ")
}