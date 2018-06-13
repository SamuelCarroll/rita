package commands

import (
	"github.com/SamuelCarroll/rita/machineLearning"
	"github.com/activecm/rita/database"
	"github.com/activecm/rita/parser/parsetypes"
	"github.com/urfave/cli"
)

func init() {
	rfCommand := cli.Command{
		Name:  "forest-eval",
		Usage: "Analyzes a dataset using a random forest technique (NOTICE: still experimental)",
		Flages: []cli.Flag{
			databaseFlag,
			configFlag,
		},
		Action: func(c *cli.Context) error {
			res := database.InitResources(c.String("config"))

			databaseName := c.String("database")
			if databaseName == "" {
				return cli.NewExitError("Random Forest Evaluation failed.\n\tUse "+
					"'rita forest-eval' to evaluate a dataset using a random forest.", -1)
			}

			res.DB.SelectDB(databaseName)
			beacons, scans := res.DB.GetAnomalies()

			srcs, dsts := getSrcDstStrings(beacons, scans)

			normConns, anomConns := res.DB.GetSrcDst(srcs, dsts)

			allData := preprocessData(normConns, anomConns)

			return nil
		},
	}

	bootstrapCommands(rfCommand)
}

func preprocessData([]parsetypes.Conn, []parsetypes.Conn) []*ritaML.Data {
	var newData []*ritaML.Data
}
