package storage

import (
	"bytes"
	"encoding/gob"
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

type Axis struct {
	Keys       [][]byte   `json:"keys,omitempty"`
	ValuesList [][]uint64 `json:"values_list,omitempty"`
}

func buildAxisFromMatrixAxis(axis matrix.Axis) Axis {
	newAxis := Axis{
		make([][]byte, len(axis.Keys)),
		axis.ValuesList,
	}
	for i, key := range axis.Keys {
		newAxis.Keys[i] = []byte(key)
	}
	return newAxis
}

func buildMatrixAxisFromAxis(axis Axis) matrix.Axis {
	newAxis := matrix.Axis{
		Keys:       make([]string, len(axis.Keys)),
		ValuesList: axis.ValuesList,
	}
	for i, key := range axis.Keys {
		newAxis.Keys[i] = string(key)
	}
	return newAxis
}

type Plane struct {
	Time time.Time
	LayerNum  uint8  `gorm:"column:layer_num"`
	//StartKey string  `gorm:"type:text"`
	Axis []byte
}

func (Plane) TableName() string {
	return tableName
}
/**********************************
 Gob编码，Axis做了中间转换
 ***********************************/
func NewPlaneGobChanged(time time.Time, num uint8, axis matrix.Axis) (*Plane, error) {
	newAxis := buildAxisFromMatrixAxis(axis)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(newAxis)
	if err != nil {
		return nil, err
	}
	return &Plane{
		time,
		num,
		buf.Bytes(),
	}, nil
}
func (p Plane)UnmarshalGobChanged() (matrix.Axis, error)  {
	var buf = bytes.NewBuffer(p.Axis)
	dec := gob.NewDecoder(buf)
	var axis Axis
	err := dec.Decode(&axis)
	if err != nil {
		return matrix.Axis{}, err
	}
	var newAxis = buildMatrixAxisFromAxis(axis)
	return newAxis, nil
}

/**********************************
Gob编码，Axis没有做了中间转换
***********************************/
func NewPlaneGobUnchanged(time time.Time, num uint8, axis matrix.Axis) (*Plane, error) {
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
func (p Plane) UnmarshalGobUnchanged() (matrix.Axis, error)  {
	var buf = bytes.NewBuffer(p.Axis)
	dec := gob.NewDecoder(buf)
	var axis matrix.Axis
	err := dec.Decode(&axis)
	return axis, err
}

/**********************************
Json编码，Axis做了中间转换
***********************************/
func NewPlaneJsonChanged(time time.Time, num uint8, axis matrix.Axis) (*Plane, error) {
	newAxis := buildAxisFromMatrixAxis(axis)
	bytes, err := json.Marshal(newAxis)
	if err != nil {
		return nil, err
	}
	return &Plane{
		time,
		num,
		bytes,
	}, nil
}
func (p Plane)UnmarshalJsonChanged() (matrix.Axis, error)  {
	var axis Axis
	err := json.Unmarshal(p.Axis, &axis)
	var newAxis = buildMatrixAxisFromAxis(axis)
	return newAxis, err
}

/**********************************
Json编码，Axis没有做了中间转换
***********************************/
func NewPlaneJsonUnchanged(time time.Time, num uint8, axis matrix.Axis) (*Plane, error) {
	bytes, err := json.Marshal(axis)
	if err != nil {
		return nil, err
	}
	return &Plane{
		time,
		num,
		bytes,
	}, nil
}
func (p Plane)UnmarshalJsonUnchanged() (matrix.Axis, error)  {
	var axis matrix.Axis
	err := json.Unmarshal(p.Axis, &axis)
	return axis, err
}

const Mode  = 2

/**********************************
1: Gob编码，Axis做了中间转换
2: Gob编码，Axis没有做了中间转换
3: Json编码，Axis做了中间转换
4: Json编码，Axis没有做了中间转换
***********************************/
func NewPlaneMode(time time.Time, num uint8, axis matrix.Axis, mode int) (*Plane, error) {
	var plane *Plane
	var err error
	switch mode {
	case 1:
		plane, err = NewPlaneGobChanged(time, num, axis)
	case 2:
		plane, err = NewPlaneGobUnchanged(time, num, axis)
	case 3:
		plane, err = NewPlaneJsonChanged(time, num, axis)
	case 4:
		plane, err = NewPlaneJsonUnchanged(time, num, axis)
	}
	return plane, err
}
func (p Plane)UnmarshalMode(mode int) (matrix.Axis, error)  {
	var axis matrix.Axis
	var err error
	switch mode {
	case 1:
		axis, err = p.UnmarshalGobChanged()
	case 2:
		axis, err = p.UnmarshalGobUnchanged()
	case 3:
		axis, err = p.UnmarshalJsonChanged()
	case 4:
		axis, err = p.UnmarshalJsonUnchanged()
	}
	return axis, err
}



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
