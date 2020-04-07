package storage

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"time"
	"unsafe"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

const (
	tablePlaneName     = "planes"
	tableKeyInternName = "key_interns"
)

type Axis struct {
	Keys       []uint64
	ValuesList [][]uint64
}

type KeyIntern struct {
	ID  uint64 `gorm:"primary_key"`
	Key string
}

func (KeyIntern) TableName() string {
	return tableKeyInternName
}

func getStringID(str *string) uint64 {
	p := (*reflect.StringHeader)(unsafe.Pointer(str))
	return uint64(p.Data)
}

func InsertKey(db *dbstore.DB, key *string) {
	id := getStringID(key)
	db.Where(KeyIntern{ID: id}).Assign(KeyIntern{Key: *key}).FirstOrCreate(&KeyIntern{})
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

func (p Plane) UnmarshalAxis() (Axis, error) {
	var buf = bytes.NewBuffer(p.Axis)
	dec := gob.NewDecoder(buf)
	var axis Axis
	err := dec.Decode(&axis)
	return axis, err
}

// if the table `table` exists, return true, nil
// or create table `table`
func CreateTableIfNotExists(db *dbstore.DB, table interface{}) (bool, error) {
	if db.HasTable(table) {
		return true, nil
	}
	return false, db.CreateTable(table).Error
}

func ClearTable(db *dbstore.DB, table interface{}) error {
	return db.Delete(table).Error
}

func InsertPlane(db *dbstore.DB, num uint8, time time.Time, axis matrix.Axis) error {
	newAxis := Axis{
		Keys:       make([]uint64, len(axis.Keys)),
		ValuesList: axis.ValuesList,
	}
	for i := range axis.Keys {
		newAxis.Keys[i] = getStringID(&axis.Keys[i])
		InsertKey(db, &axis.Keys[i])
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

func ReadAllPlane(db *dbstore.DB) (matrixPlanes []matrix.Plane, err error) {
	var keyInterns []KeyIntern
	err = db.Find(&keyInterns).Error
	if err != nil {
		return
	}
	// clear table KeyIntern
	err = ClearTable(db, &KeyIntern{})
	if err != nil {
		return
	}

	keyIDStringMap := make(map[uint64]string, len(keyInterns))
	keyIDIDMap := make(map[uint64]uint64, len(keyInterns))
	for i := range keyInterns {
		keyIDStringMap[keyInterns[i].ID] = keyInterns[i].Key
		keyIDIDMap[keyInterns[i].ID] = getStringID(&keyInterns[i].Key)
		InsertKey(db, &keyInterns[i].Key)
	}

	var allPlanes []Plane
	num := 0
	for ; ; num++ {
		planes, err := FindPlaneOrderByTime(db, uint8(num))
		if err != nil || len(planes) == 0 {
			break
		}
		mPlane := matrix.Plane{
			Times: make([]time.Time, len(planes)),
			Axes:  make([]matrix.Axis, len(planes)),
		}
		for i := range planes {
			mPlane.Times[i] = planes[i].Time
			axis, _ := planes[i].UnmarshalAxis()
			mPlane.Axes[i].ValuesList = axis.ValuesList
			mPlane.Axes[i].Keys = make([]string, len(axis.Keys))
			for j := range axis.Keys {
				mPlane.Axes[i].Keys[j] = keyIDStringMap[axis.Keys[j]]
				axis.Keys[j] = keyIDIDMap[axis.Keys[j]]
			}
			plane, _ := NewPlane(uint8(num), planes[i].Time, axis)
			allPlanes = append(allPlanes, *plane)
		}

		matrixPlanes = append(matrixPlanes, mPlane)
	}

	// clear table Plane
	err = ClearTable(db, &Plane{})
	if err != nil {
		return
	}
	for i := range allPlanes {
		err = db.Create(&allPlanes[i]).Error
		if err != nil {
			break
		}
	}
	return
}
