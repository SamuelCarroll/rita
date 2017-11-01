package database

import (
	"errors"

	"github.com/ocmdev/rita/datatypes/structure"
	"github.com/ocmdev/rita/parser/parsetypes"
	log "github.com/sirupsen/logrus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// DB is the workhorse container for messing with the database
type DB struct {
	Session   *mgo.Session
	resources *Resources
	selected  string
}

///////////////////////////////////////////////////////////////////////////////
////////////////////////// SUPPORTING FUNCTIONS ///////////////////////////////
///////////////////////////////////////////////////////////////////////////////

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
		d.resources.Log.WithFields(log.Fields{
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

	d.resources.Log.Debug("Building collection: ", name)

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
		d.resources.Log.Warning("Failed aggregation: (Source collection: ",
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
		d.resources.Log.WithFields(log.Fields{
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
		d.resources.Log.Warning("Failed map reduce: (Source collection: ", sourceCollection, " doesn't exist)")
		return false
	}
	collection := session.DB(d.selected).C(sourceCollection)

	// Map reduce that shit
	_, err := collection.Find(nil).MapReduce(&job, nil)

	// If error, Throw computer against wall and drink 2 angry beers while
	// questioning your life, purpose, and relationships.
	if err != nil {
		d.resources.Log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Failed map reduce")
		return false
	}

	return true
}

// GetSrcDst will get the highest source destination pair and return a slice of conn collections
func (d *DB) GetSrcDst() []parsetypes.Conn {
	session := d.Session.Copy()
	defer session.Close()

	src, dst := d.getHighUConn()

	if src == "" || dst == "" {
		d.resources.Log.Warning("Error getting source destination pair")
		return nil
	}

	connName := d.resources.Config.T.Structure.ConnTable
	coll := session.DB(d.selected).C(connName)

	var srcDstConns []parsetypes.Conn
	srcDstQuery := bson.M{"id_origin_h": src, "id_resp_h": dst}
	err := coll.Find(srcDstQuery).All(&srcDstConns)

	if err != nil {
		d.resources.Log.Warning("Error with conn query")
		return nil
	}

	return srcDstConns
}

func (d *DB) getHighUConn() (string, string) {
	session := d.Session.Copy()
	defer session.Close()

	uconnName := d.resources.Config.T.Structure.UniqueConnTable
	if !d.CollectionExists(uconnName) {
		d.resources.Log.Warning("Couldn't get ", uconnName, " please make sure you've analyzed first")
		return "", ""
	}
	localQuery := bson.M{"$or": []bson.M{bson.M{"local_src": true}, bson.M{"local_dst": true}}}

	uconn := session.DB(d.selected).C(uconnName).Find(localQuery)

	var high structure.UniqueConnection
	err := uconn.Sort("-connection_count").One(&high)

	if err != nil {
		d.resources.Log.Warning("Couldn't get src dst from uconn")
		return "", ""
	}

	src := high.Src
	dst := high.Dst

	return src, dst
}
