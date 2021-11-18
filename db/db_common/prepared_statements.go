package db_common

import (
	"context"
	"fmt"
	"log"
	"strings"

	typehelpers "github.com/turbot/go-kit/types"
	"github.com/turbot/steampipe/steampipeconfig/modconfig"
	"github.com/turbot/steampipe/utils"
)

func CreatePreparedStatements(ctx context.Context, resourceMaps *modconfig.WorkspaceResourceMaps, client Client) error {
	log.Printf("[TRACE] CreatePreparedStatements")

	utils.LogTime("db.CreatePreparedStatements start")
	defer utils.LogTime("db.CreatePreparedStatements end")

	// first get the SQL to create all prepared statements
	sqlMap := GetPreparedStatementsSQL(resourceMaps)
	if len(sqlMap) == 0 {
		return nil
	}
	// first try to run the whole thing in one query
	var queries []string
	for _, q := range sqlMap {
		queries = append(queries, q)
	}

	// execute the query, passing 'true' to disable the spinner
	_, err := client.ExecuteSync(ctx, strings.Join(queries, ";\n"), true)

	// if there was an error - we would like to know which query or control failed, so try to create them one by one
	if err != nil {
		the_err := err
		the_bad := "?"
		log.Printf("[WARN] There was some error %v", err)
		for name, sql := range sqlMap {
			if _, err = client.ExecuteSync(ctx, sql, true); err != nil {
				s := fmt.Sprintf("%v", err)
				if strings.Contains(s, "prepared statement") && strings.Contains(s, "already exists") {
					log.Printf("[WARN] skipping [%s]", s)
					// Successful creation in previous batch.
					continue
				}
				the_err = err
				the_bad = name
				log.Printf("[WARN] error %v in %v", the_err, the_bad)
			}
		}
		return fmt.Errorf("failed to create prepared statement for %s: %v", the_bad, the_err)
	}

	// return context error - this enables calling code to respond to cancellation
	return ctx.Err()
}

func GetPreparedStatementsSQL(resourceMaps *modconfig.WorkspaceResourceMaps) map[string]string {
	// make map of resource name to create SQL
	sqlMap := make(map[string]string)
	for _, query := range resourceMaps.Queries {
		// query map contains long and short names for queries - have we already created this query
		if _, ok := sqlMap[query.FullName]; ok {
			continue
		}

		// remove trailing semicolons from sql as this breaks the prepare statement
		rawSql := strings.TrimRight(strings.TrimSpace(typehelpers.SafeString(query.SQL)), ";")
		preparedStatementName := query.GetPreparedStatementName()
		sqlMap[query.FullName] = fmt.Sprintf("PREPARE %s AS (\n%s\n)", preparedStatementName, rawSql)
	}

	for _, control := range resourceMaps.Controls {
		// query map contains long and short names for queries - have we already created this query
		if _, ok := sqlMap[control.FullName]; ok {
			continue
		}
		// only create prepared statements for controls with inline SQL
		if control.SQL == nil {
			continue
		}

		// remove trailing semicolons from sql as this breaks the prepare statement
		rawSql := strings.TrimRight(strings.TrimSpace(typehelpers.SafeString(control.SQL)), ";")
		preparedStatementName := control.GetPreparedStatementName()
		sqlMap[control.FullName] = fmt.Sprintf("PREPARE %s AS (\n%s\n)", preparedStatementName, rawSql)
	}

	return sqlMap

}

// UpdatePreparedStatements first attempts to deallocate all prepared statements in workspace, then recreates them
func UpdatePreparedStatements(ctx context.Context, prevResourceMaps, currentResourceMaps *modconfig.WorkspaceResourceMaps, client Client) error {
	log.Printf("[WARN] UpdatePreparedStatements")

	utils.LogTime("db.UpdatePreparedStatements start")
	defer utils.LogTime("db.UpdatePreparedStatements end")

	var sql []string
	for name, query := range prevResourceMaps.Queries {
		// query map contains long and short names for queries - avoid dupes
		if !strings.HasPrefix(name, "query.") {
			continue
		}
		sql = append(sql, fmt.Sprintf("DEALLOCATE %s;", query.GetPreparedStatementName()))
	}
	for name, control := range prevResourceMaps.Controls {
		// query map contains long and short names for controls - avoid dupes
		if !strings.HasPrefix(name, "control.") {
			continue
		}
		// do not create prepared statements for controls which reference another query
		if control.Query != nil {
			continue
		}
		sql = append(sql, fmt.Sprintf("DEALLOCATE %s;", control.GetPreparedStatementName()))
	}

	// execute the query, passing 'true' to disable the spinner
	for _, s := range sql {
		_, err := client.ExecuteSync(ctx, s, true)
		if err != nil {
			log.Printf("[WARN] failed to update prepared statements - deallocate returned error %v", err)
			// Don't return, give a chance to re-create the statements.
			// return err
		}
	}

	// now recreate them
	return CreatePreparedStatements(ctx, currentResourceMaps, client)

}
