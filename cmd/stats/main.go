package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/filecoin-project/lassie-event-recorder/statsrunner"
	"github.com/ipfs/go-log/v2"
	"github.com/olekukonko/tablewriter"
)

var logger = log.Logger("lassie/statsrunner")

func main() {
	dbDSN := flag.String("dbDSN", "", "The database Data Source Name. Alternatively, it may be specified via LASSIE_EVENT_RECORDER_DB_DSN environment variable. If both are present, the environment variable takes precedence.")
	logLevel := flag.String("logLevel", "info", "The logging level. Only applied if GOLOG_LOG_LEVEL environment variable is unset.")
	wipeTable := flag.Bool("wipeTable", false, "tells the command to wipe the table after running the query")
	flag.Parse()

	if _, set := os.LookupEnv("GOLOG_LOG_LEVEL"); !set {
		_ = log.SetLogLevel("*", *logLevel)
	}

	if v, set := os.LookupEnv("LASSIE_EVENT_RECORDER_DB_DSN"); set {
		dbDSN = &v
	}

	ctx := context.Background()

	statsRunner, err := statsrunner.New(*dbDSN)
	if err != nil {
		logger.Fatalw("error setting up stats runner", "err", err)
	}

	if err := statsRunner.Start(ctx); err != nil {
		logger.Fatalw("error starting stats runner", "err", err)
	}

	summary, err := statsRunner.GetEventSummary(ctx)
	if err != nil {
		logger.Fatalw("error running query", "err", err)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Total Attempts", "Attempted Bitswap", "Attempted GraphSync", "Attempted Both", "Attempted Either", "Bitswap Successes", "GraphSync Successes", "Average Bandwidth", "Time to first byte", "Download Size", "GraphSync Attempts Past Query"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.Append([]string{
		fmt.Sprintf("%d", summary.TotalAttempts),
		fmt.Sprintf("%d", summary.AttemptedBitswap),
		fmt.Sprintf("%d", summary.AttemptedGraphSync),
		fmt.Sprintf("%d", summary.AttemptedBoth),
		fmt.Sprintf("%d", summary.AttemptedEither),
		fmt.Sprintf("%d", summary.BitswapSuccesses),
		fmt.Sprintf("%d", summary.GraphSyncSuccesses),
		fmt.Sprintf("%d", summary.AvgBandwidth),
		fmt.Sprintf("%v", summary.FirstByte),
		fmt.Sprintf("%v", summary.DownloadSize),
		fmt.Sprintf("%d", summary.GraphsyncAttemptsPastQuery),
	}) // Add Bulk Data
	table.Render()

	if *wipeTable {
		statsRunner.WipeTable(ctx)
		logger.Infow("Successfully ran summary and cleared DB")
	}

	statsRunner.Close()

}
