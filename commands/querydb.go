package commands

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"github.com/ocmdev/rita/database"
	"github.com/ocmdev/rita/datatypes/beacon"
	"github.com/ocmdev/rita/datatypes/scanning"
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
			beacons, scans := res.DB.GetAnomalies()

			srcs, dsts := getSrcDstStrings(beacons, scans)

			normConns, anomConns := res.DB.GetSrcDst(srcs, dsts)

			fmt.Println(unsafe.Sizeof(normConns[0]))

			fmt.Println("Anomalous size:", len(anomConns))
			fmt.Println("Normal size:", len(normConns))

			err := writeCSV(normConns, anomConns)
			if err != nil {
				return cli.NewExitError("Couldn't write the CSV file error: "+err.Error(), -1)
			}

			return nil
		},
	}

	bootstrapCommands(queryCommand)
}

func writeCSV(normConns []parsetypes.Conn, anomConns []parsetypes.Conn) error {
	normalFile, err := os.Create("normalTraffic.csv")
	if err != nil {
		return err
	}
	anomalyFile, err := os.Create("anomalousTraffic.csv")
	if err != nil {
		return err
	}

	normalWriter := csv.NewWriter(normalFile)
	anomalyWriter := csv.NewWriter(anomalyFile)

	header := makeHeader()

	err = normalWriter.Write(header)
	if err != nil {
		return err
	}
	err = anomalyWriter.Write(header)
	if err != nil {
		return err
	}

	writeConns(normConns, normalWriter)
	writeConns(anomConns, anomalyWriter)

	return nil
}

func writeConns(connections []parsetypes.Conn, csvWriter *csv.Writer) error {

	for _, conn := range connections {
		histSlice := getHistSlice(conn.History)
		connSlice := getConnSlice(conn, histSlice)

		csvWriter.Write(connSlice)
	}

	return nil
}

func getSrcDstStrings(beacons []beacon.BeaconAnalysisView, scans []scanning.Scan) ([]string, []string) {
	var srcStrs []string
	var dstStrs []string

	for _, scan := range scans {
		srcStrs = append(srcStrs, scan.Src)
		dstStrs = append(dstStrs, scan.Dst)
	}

	for _, beac := range beacons {
		srcStrs = append(srcStrs, beac.Src)
		dstStrs = append(dstStrs, beac.Dst)
	}

	return srcStrs, dstStrs
}

func getHistSlice(hist string) []string {
	var histSlice []string

	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "S")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "s")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "H")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "h")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "A")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "a")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "D")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "d")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "F")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "f")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "R")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "r")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "C")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "c")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "I")))
	histSlice = append(histSlice, boolToInt(strings.Contains(hist, "i")))

	return histSlice
}

func boolToInt(boolIn bool) string {
	if boolIn == true {
		return "1"
	}

	return "0"
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
	header = append(header, "Src Syn Set")
	header = append(header, "Dst Syn Set")
	header = append(header, "Src Syn-Ack Set")
	header = append(header, "Dst Syn-Ack Set")
	header = append(header, "Src Ack Set")
	header = append(header, "Dst Ack Set")
	header = append(header, "Src Data Set")
	header = append(header, "Dst Data Set")
	header = append(header, "Src Fin Set")
	header = append(header, "Dst Fin Set")
	header = append(header, "Src Rst Set")
	header = append(header, "Dst Rst Set")
	header = append(header, "Src Bad Checksum")
	header = append(header, "Dst Bad Checksum")
	header = append(header, "Src Inconsistent Packet")
	header = append(header, "Dst Inconsistent Packet")
	header = append(header, "Origin Packets")
	header = append(header, "Origin IP Bytes")
	header = append(header, "Response Packets")
	header = append(header, "Response IP Bytes")

	return header
}

func getConnSlice(conn parsetypes.Conn, connHistory []string) []string {
	var connSlice []string
	connSlice = append(connSlice, strconv.FormatInt(conn.TimeStamp, 10))
	connSlice = append(connSlice, conn.UID)
	connSlice = append(connSlice, conn.Source)
	connSlice = append(connSlice, strconv.Itoa(conn.SourcePort))
	connSlice = append(connSlice, conn.Destination)
	connSlice = append(connSlice, strconv.Itoa(conn.DestinationPort))
	connSlice = append(connSlice, transportToInt(conn.Proto))
	connSlice = append(connSlice, serviceToInt(conn.Service))
	connSlice = append(connSlice, strconv.FormatFloat(conn.Duration, 'f', -1, 64))
	connSlice = append(connSlice, strconv.FormatInt(conn.OrigIPBytes, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.RespBytes, 10))
	connSlice = append(connSlice, connStInt(conn.ConnState))
	connSlice = append(connSlice, boolToInt(conn.LocalOrigin))
	connSlice = append(connSlice, boolToInt(conn.LocalResponse))
	connSlice = append(connSlice, strconv.FormatInt(conn.MissedBytes, 10))
	for i := 0; i < len(connHistory); i++ {
		connSlice = append(connSlice, connHistory[i])
	}
	connSlice = append(connSlice, strconv.FormatInt(conn.OrigPkts, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.OrigIPBytes, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.RespPkts, 10))
	connSlice = append(connSlice, strconv.FormatInt(conn.RespIPBytes, 10))

	return connSlice
}

func serviceToInt(service string) string {
	switch service {
	case "ssl":
		return "1"
	case "dns":
		return "2"
	case "dhcp":
		return "3"
	case "sip":
		return "4"
	case "snmp":
		return "5"
	case "ssh":
		return "6"
	case "teredo":
		return "7"
	case "dnp3_udp":
		return "8"
	case "smtp":
		return "9"
	case "krb":
		return "10"
	case "smtp,ssl":
		return "11"
	case "ssl,smtp":
		return "11"
	case "http":
		return "12"
	case "pop3":
		return "13"
	case "ftp":
		return "14"
	case "ftp-data":
		return "15"
	case "ssl,http":
		return "16"
	case "http,ssl":
		return "16"
	case "rdp":
		return "17"
	case "irc":
		return "17"
	case "irc,http":
		return "19"
	case "http,irc":
		return "19"
	case "socks":
		return "20"
	}

	return "0"
}

func transportToInt(tpl string) string {
	switch tpl {
	case "tcp":
		return "1"
	case "udp":
		return "2"
	case "icmp":
		return "3"
	default:
		return "0"
	}
}

func connStInt(connState string) string {
	switch connState {
	case "S0":
		return "1"
	case "S1":
		return "2"
	case "SF":
		return "3"
	case "REJ":
		return "4"
	case "S2":
		return "5"
	case "S3":
		return "6"
	case "RSTO":
		return "7"
	case "RSTR":
		return "8"
	case "RSTOS0":
		return "9"
	case "RSTRH":
		return "10"
	case "SH":
		return "11"
	case "SHR":
		return "12"
	}

	return "0"

}
