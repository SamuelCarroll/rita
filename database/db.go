package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/activecm/mgosec"
	"github.com/activecm/rita/config"
	"github.com/ocmdev/rita/datatypes/beacon"
	"github.com/ocmdev/rita/datatypes/scanning"
	"github.com/ocmdev/rita/parser/parsetypes"
	log "github.com/sirupsen/logrus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// DB is the workhorse container for messing with the database
type DB struct {
	Session  *mgo.Session
	log      *log.Logger
	selected string
}

//NewDB constructs a new DB struct
func NewDB(conf *config.Config, log *log.Logger) (*DB, error) {
	// Jump into the requested database
	session, err := connectToMongoDB(conf, log)
	if err != nil {
		return nil, err
	}
	session.SetSocketTimeout(conf.S.MongoDB.SocketTimeout)
	session.SetSyncTimeout(conf.S.MongoDB.SocketTimeout)
	session.SetCursorTimeout(0)

	return &DB{
		Session:  session,
		log:      log,
		selected: "",
	}, nil
}

//connectToMongoDB connects to MongoDB possibly with authentication and TLS
func connectToMongoDB(conf *config.Config, logger *log.Logger) (*mgo.Session, error) {
	connString := conf.S.MongoDB.ConnectionString
	authMechanism := conf.R.MongoDB.AuthMechanismParsed
	tlsConfig := conf.R.MongoDB.TLS.TLSConfig
	if conf.S.MongoDB.TLS.Enabled {
		return mgosec.Dial(connString, authMechanism, tlsConfig)
	}
	return mgosec.DialInsecure(connString, authMechanism)
}

//SelectDB selects a database for analysis
func (d *DB) SelectDB(db string) {
	d.selected = db
}

//GetSelectedDB retrieves the currently selected database for analysis
func (d *DB) GetSelectedDB() string {
	return d.selected
}

//CollectionExists returns true if collection exists in the currently
//selected database
func (d *DB) CollectionExists(table string) bool {
	ssn := d.Session.Copy()
	defer ssn.Close()
	coll, err := ssn.DB(d.selected).CollectionNames()
	if err != nil {
		d.log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Failed collection name lookup")
		return false
	}
	for _, name := range coll {
		if name == table {
			return true
		}
	}
	return false
}

//CreateCollection creates a new collection in the currently selected
//database with the required indeces
func (d *DB) CreateCollection(name string, id bool, indeces []mgo.Index) error {
	// Make a copy of the current session
	session := d.Session.Copy()
	defer session.Close()

	if len(name) < 1 {
		return errors.New("name error: check collection name in yaml file and config")
	}

	// Check if ollection already exists
	if d.CollectionExists(name) {
		return errors.New("collection already exists")
	}

	d.log.Debug("Building collection: ", name)

	// Create new collection by referencing to it, no need to call Create
	err := session.DB(d.selected).C(name).Create(
		&mgo.CollectionInfo{
			DisableIdIndex: !id,
		},
	)

	// Make sure it actually got created
	if err != nil {
		return err
	}

	collection := session.DB(d.selected).C(name)
	for _, index := range indeces {
		err := collection.EnsureIndex(index)
		if err != nil {
			return err
		}
	}

	return nil
}

//AggregateCollection builds a collection via a MongoDB pipeline
func (d *DB) AggregateCollection(sourceCollection string,
	session *mgo.Session, pipeline []bson.D) *mgo.Iter {

	// Identify the source collection we will aggregate information from into the new collection
	if !d.CollectionExists(sourceCollection) {
		d.log.Warning("Failed aggregation: (Source collection: ",
			sourceCollection, " doesn't exist)")
		return nil
	}
	collection := session.DB(d.selected).C(sourceCollection)

	// Create the pipe
	pipe := collection.Pipe(pipeline).AllowDiskUse()

	iter := pipe.Iter()

	// If error, Throw computer against wall and drink 2 angry beers while
	// questioning your life, purpose, and relationships.
	if iter.Err() != nil {
		d.log.WithFields(log.Fields{
			"error": iter.Err().Error(),
		}).Error("Failed aggregate operation")
		return nil
	}
	return iter
}

//MapReduceCollection builds collections via javascript map reduce jobs
func (d *DB) MapReduceCollection(sourceCollection string, job mgo.MapReduce) bool {
	// Make a copy of the current session
	session := d.Session.Copy()
	defer session.Close()

	// Identify the source collection we will aggregate information from into the new collection
	if !d.CollectionExists(sourceCollection) {
		d.log.Warning("Failed map reduce: (Source collection: ", sourceCollection, " doesn't exist)")
		return false
	}
	collection := session.DB(d.selected).C(sourceCollection)

	// Map reduce that shit
	_, err := collection.Find(nil).MapReduce(&job, nil)

	// If error, Throw computer against wall and drink 2 angry beers while
	// questioning your life, purpose, and relationships.
	if err != nil {
		d.log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Failed map reduce")
		return false
	}

	return true
}

//GetAnomalies returns the beacon and scan tables
func (d *DB) GetAnomalies(beaconName, scanName string) ([]beacon.BeaconAnalysisView, []scanning.Scan) {
	session := d.Session.Copy()
	defer session.Close()

	var beacons []beacon.BeaconAnalysisView
	var scans []scanning.Scan
	var retBeacons []beacon.BeaconAnalysisView
	var retScans []scanning.Scan

	coll := session.DB(d.selected).C(beaconName)
	err := coll.Find(nil).All(&beacons)
	if err != nil {
		d.log.Warning("Error with Beacon query")
		return nil, nil
	}
	for _, beacon := range beacons {
		if !localTraffic(beacon.Src, beacon.Dst) {
			retBeacons = append(retBeacons, beacon)
		}
	}

	coll = session.DB(d.selected).C(scanName)
	err = coll.Find(nil).All(&scans)
	if err != nil {
		d.log.Warning("Error with Scan query")
		return retBeacons, nil
	}
	for _, scan := range scans {
		if !localTraffic(scan.Src, scan.Dst) {
			retScans = append(retScans, scan)
		}
	}

	return retBeacons, retScans
}

func localTraffic(addr1, addr2 string) bool {
	addr1Split := strings.LastIndex(addr1, ".")
	addr2Split := strings.LastIndex(addr2, ".")

	if addr1Split == -1 || addr2Split == -1 {
		return false
	}

	addr1 = addr1[:addr1Split]
	addr2 = addr2[:addr2Split]

	if strings.Compare(addr1, addr2) == 0 {
		return true
	}

	return false
}

// GetSrcDst will get the highest source destination pair and return a slice of conn collections
func (d *DB) GetSrcDst(srcs, dsts []string, connName string) ([]parsetypes.Conn, []parsetypes.Conn) {
	session := d.Session.Copy()
	defer session.Close()

	coll := session.DB(d.selected).C(connName)

	var normConns []parsetypes.Conn
	var anomConns []parsetypes.Conn

	srcsNotIn := bson.M{"$nin": srcs}
	dstsNotIn := bson.M{"$nin": dsts}
	srcsIn := bson.M{"$in": srcs}
	dstsIn := bson.M{"$in": dsts}

	normQuery := bson.M{"$and": []bson.M{bson.M{"id_origin_h": srcsNotIn}, bson.M{"id_resp_h": dstsNotIn}}}
	anomQuery := bson.M{"$and": []bson.M{bson.M{"id_origin_h": srcsIn}, bson.M{"id_resp_h": dstsIn}}}

	err := coll.Find(anomQuery).All(&anomConns)
	if err != nil {
		d.log.Warning("Error with conn anomalous query")
		return nil, nil
	}
	fmt.Println("Got the anamalies")

	//Since we can work with big datasets we want to limit the results we get back
	totLen := 10 * len(anomConns)

	err = coll.Find(normQuery).Limit(totLen).All(&normConns)
	if err != nil {
		d.log.Warning("Error with conn normal query")
		return nil, nil
	}
	fmt.Println("Got the normies")

	return normConns, anomConns
}
