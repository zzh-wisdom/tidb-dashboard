package storage

import (
	"encoding/json"
	"time"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

const tableName = "planes"

//type RingData struct {
//	RingTimes []time.Time    `json:"ring_times,omitempty"`
//	RingAxes  []matrix.Axis  `json:"ring_axes,omitempty"`
//}
//
//func BuildRingData(ringTimes []time.Time, ringAxes []matrix.Axis) RingData {
//	return RingData{
//		RingTimes: ringTimes,
//		RingAxes:  ringAxes,
//	}
//}

type Plane struct {
	Time time.Time `gorm:"primary_key"`
	Num  uint8     `gorm:"primary_key"`
	//StartKey string  `gorm:"type:text"`
	Axis string `gorm:"type:text"`
}

func (Plane) TableName() string {
	return tableName
}

func NewPlane(time time.Time, num uint8, axis matrix.Axis) (*Plane, error) {
	bytes, err := json.Marshal(axis)
	if err != nil {
		return nil, err
	}
	return &Plane{
		Time: time,
		Num:  num,
		Axis: string(bytes),
	}, nil
}

//func (p *Plane)Unmarshal() (RingData, error)  {
//	var ringData RingData
//	err := json.Unmarshal([]byte(p.Axes), &ringData)
//	if err != nil {
//		return ringData, err
//	}
//	return ringData, err
//}

// Check if the `table` exists
func checkTable(db *dbstore.DB, table string) bool {
	return db.HasTable(table)
}

//func insert(db *dbstore.DB, table string, data interface{}) error {
//	err := db.
//		Table(table).
//		Create(data).
//		Error
//	return err
//}
//
//func delete(db *dbstore.DB, table string, data interface{}) error {
//	err := db.
//		Table(table).
//		Delete(data).
//		Error
//	return err
//}

//func find(db *dbstore.DB, table string, where interface{}, order interface{}, results interface{}) error {
//	err := db.
//		Table(table).
//		Where(where).
//		Order(order).
//		Find(results).
//		Error
//	return err
//}
