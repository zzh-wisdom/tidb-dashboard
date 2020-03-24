package storage

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

const tableName = "planes"

type Plane struct {
	Time     time.Time
	LayerNum uint8 `gorm:"column:layer_num"`
	Axis []byte
}

func (Plane) TableName() string {
	return tableName
}

func NewPlane(time time.Time, num uint8, axis matrix.Axis) (*Plane, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(axis)
	if err != nil {
		return nil, err
	}
	return &Plane{
		time,
		num,
		buf.Bytes(),
	}, nil
}

func (p Plane) UnmarshalAxis() (matrix.Axis, error) {
	var buf = bytes.NewBuffer(p.Axis)
	dec := gob.NewDecoder(buf)
	var axis matrix.Axis
	err := dec.Decode(&axis)
	return axis, err
}

// Check if the `table` exists
func checkTable(db *dbstore.DB, table string) bool {
	return db.HasTable(table)
}