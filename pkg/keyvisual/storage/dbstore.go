package storage

import (
	"bytes"
	"encoding/gob"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
	"time"
	"unsafe"
)

const (
	tablePlaneName = "planes"
	tableKeyInternName = "key_interns"
)



type Axis struct {
	Keys       []uint64
	ValuesList [][]uint64
}

type KeyIntern struct {
	ID uint64 `gorm:"primary_key"`
	Key string
}

func (KeyIntern) TableName() string {
	return tableKeyInternName
}

type Plane struct {
	LayerNum uint8 `gorm:"column:layer_num"`
	Time     time.Time
	Axis     []byte
}

func (Plane) TableName() string {
	return tablePlaneName
}

func NewPlane(num uint8, time time.Time, axis Axis) (*Plane, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(axis)
	if err != nil {
		return nil, err
	}
	return &Plane{
		num,
		time,
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

// if the table `Plane` exists, return true, nil
// or create table `Plane`
func CreateTablePlaneIfNotExists(db *dbstore.DB) (bool, error) {
	if db.HasTable(&Plane{}) {
		return true, nil
	}
	return false, db.CreateTable(&Plane{}).Error
}

func ClearTablePlane(db *dbstore.DB) error {
	return db.Table(tablePlaneName).Delete(&Plane{}).Error
}

func InsertPlane(db *dbstore.DB, num uint8, time time.Time, axis matrix.Axis) error {
	newAxis := Axis {
		Keys: make([]uint64, len(axis.Keys)),
		ValuesList: axis.ValuesList,
	}
	for i := range axis.Keys {
		id := uint64(uintptr(unsafe.Pointer(&axis.Keys[i])))
		str := axis.Keys[i]
		db.Where(KeyIntern{ID: id}).Assign(KeyIntern{Key: str}).FirstOrCreate(&KeyIntern{})

		newAxis.Keys[i] = id
	}

	plane, err := NewPlane(num, time, newAxis)
	if err != nil {
		return err
	}
	return db.Table(tablePlaneName).Create(plane).Error
}

func DeletePlane(db *dbstore.DB, num uint8, time time.Time) error {
	return db.
		Table(tablePlaneName).
		Where("layer_num = ? AND time = ?", num, time).
		Delete(&Plane{}).
		Error
}

func FindPlaneOrderByTime(db *dbstore.DB, num uint8) ([]Plane, error) {
	var planes []Plane
	err := db.
		Table(tablePlaneName).
		Where("layer_num = ?", num).
		Order("Time").
		Find(&planes).
		Error
	return planes, err
}

/// todo:恢复 重构
