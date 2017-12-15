package mgorus

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//Hooker is a logrus hooker for mongodb
type Hooker struct {
	c *mgo.Collection
}

//NewHooker dials a mongodb server without authentication or encryption
//and readies the hook to place logs inside of the given collection
func NewHooker(mgoURL, db, collection string) (*Hooker, error) {
	session, err := mgo.Dial(mgoURL)
	if err != nil {
		return nil, err
	}

	return &Hooker{c: session.DB(db).C(collection)}, nil
}

//NewHookerFromSession makes a copy of an existing mongodb session
//and readies the hook to place logs inside of the given collection
func NewHookerFromSession(session *mgo.Session, db, collection string) *Hooker {
	return &Hooker{c: session.Copy().DB(db).C(collection)}
}

//Fire places a logrus entry into the log collection
func (h *Hooker) Fire(entry *logrus.Entry) error {
	data := make(logrus.Fields)
	data["Level"] = entry.Level
	data["Time"] = entry.Time
	data["Message"] = entry.Message

	for k, v := range entry.Data {
		if errData, isError := v.(error); logrus.ErrorKey == k && v != nil && isError {
			data[k] = errData.Error()
		} else {
			data[k] = v
		}
	}

	mgoErr := h.c.Insert(bson.M(data))

	if mgoErr != nil {
		return fmt.Errorf("Failed to send log entry to mongodb: %v", mgoErr)
	}

	return nil
}

//Levels returns the logrus levels the hook supports
func (h *Hooker) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
