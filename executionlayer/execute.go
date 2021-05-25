package executionlayer

import (
	"context"

	"github.com/turbot/steampipe/db"
	"github.com/turbot/steampipe/report/reportevents"
	"github.com/turbot/steampipe/report/reportexecute"
	"github.com/turbot/steampipe/workspace"
)

func ExecuteReport(ctx context.Context, reportName string, workspace *workspace.Workspace) error {
	// get a db client
	client, err := db.NewClient(true)
	if err != nil {
		return err
	}

	executionTree, err := reportexecute.NewReportExecutionTree(reportName, workspace, client)
	if err != nil {
		return err
	}

	go func() {
		defer client.Close()
		workspace.PublishReportEvent(&reportevents.ExecutionStarted{Report: executionTree.Root})

		if err := executionTree.Execute(ctx); err != nil {
			if executionTree.Root.Error == nil {
				executionTree.Root.SetError(err)
			}
		}
		workspace.PublishReportEvent(&reportevents.ExecutionComplete{Report: executionTree.Root})
	}()

	return nil
}