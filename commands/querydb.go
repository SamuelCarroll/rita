package commands

import (
	"encoding/csv"
	"os"
	"strconv"

	"github.com/ocmdev/rita/database"
	"github.com/ocmdev/rita/parser/parsetypes"
	"github.com/urfave/cli"
)

func init() {
	queryCommand := cli.Command{

		Name:  "query",
		Usage: "Queries a specific database & collection, prints results in a CSV",
		Flags: []cli.Flag{
			databaseFlag,
			configFlag,
		},
		Action: func(c *cli.Context) error {
			res := database.InitResources(c.String("config"))

			databaseName := c.String("database")

			if databaseName == "" {
				return cli.NewExitError(
					"Query failed.\n\tUse 'rita query' to query a collection inside a "+
						"specified database. No database specified with the -d flag.", -1)
			}

			res.DB.SelectDB(databaseName)
			allRes := res.DB.GetSrcDst()

			err := writeCSV(allRes)
			if err != nil {
				return cli.NewExitError("Couldn't write the CSV file error: "+err.Error(), -1)
			}

			return nil
		},
	}

	bootstrapCommands(queryCommand)
}

func writeCSV(srcDstConn []parsetypes.Conn) error {
	file, err := os.Create("srcDstPair.csv")
	if err != nil {
		return err
	}

	fileWriter := csv.NewWriter(file)

	header := makeHeader()

	err = fileWriter.Write(header)
	if err != nil {
		return err
	}

	for _, conn := range srcDstConn {
		connSlice := getConnSlice(conn)

		err = fileWriter.Write(connSlice)
		if err != nil {
			return err
		}
	}

	return nil
}

func makeHeader() []string {
	var header []string

	header = append(header, "Time Stamp")
	header = append(header, "Unique ID")
	header = append(header, "Origin IP")
	header = append(header, "Origin Port")
	header = append(header, "Response IP")
	header = append(header, "Response Port")
	header = append(header, "Protocol")
	header = append(header, "Service")
	header = append(header, "Duration")
	header = append(header, "Origin Bytes")
	header = append(header, "Response Bytes")
	header = append(header, "Connection State")
	header = append(header, "Local Origin")
	header = append(header, "Local Response")
	header = append(header, "Missed Bytes")
	header = append(header, "Connection History")
	header = append(header, "Origin Packets")
	header = append(header, "Origin IP Bytes")
	header = append(header, "Response Packets")
	header = append(header, "Response IP Bytes")

	return header
}

func getConnSlice(conn parsetypes.Conn) []string {
	var connSlice []string
	connSlice = append(connSlice, strconv.FormatInt(conn.TimeStamp, 10))
	connSlice = append(connSlice, conn.UID)
	connSlice = append(connSlice, conn.Source)
	connSlice = append(connSlice, strconv.Itoa(conn.SourcePort))
	connSlice = append(connSlice, conn.Destination)
	connSlice = append(connSlice, strconv.Itoa(conn.DestinationPort))
	connSlice = append(connSlice, conn.Proto)
	connSlice = append(connSlice, conn.Service)
	connSlice = append(connSlice, strconv.FormatFloat(conn.Duration, 'f', -1, 64))
	connSlice = append(connSlice, strconv.FormatInt(conn.OrigIPBytes, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.RespBytes, 10))
	connSlice = append(connSlice, conn.ConnState)
	connSlice = append(connSlice, strconv.FormatBool(conn.LocalOrigin))
	connSlice = append(connSlice, strconv.FormatBool(conn.LocalResponse))
	connSlice = append(connSlice, strconv.FormatInt(conn.MissedBytes, 10))
	connSlice = append(connSlice, conn.History)
	connSlice = append(connSlice, strconv.FormatInt(conn.OrigPkts, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.OrigIPBytes, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.RespPkts, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.RespIPBytes, 10))

	return connSlice
}
