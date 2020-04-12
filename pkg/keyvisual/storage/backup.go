package storage

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/pingcap/log"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

const (
	tablePlaneName     = "planes"
	tableKeyInternName = "key_interns"
)

type KeyIntern struct {
	ID  uint64 `gorm:"primary_key"`
	Key string
}

func (KeyIntern) TableName() string {
	return tableKeyInternName
}

type Axis struct {
	Keys       []uint64
	ValuesList [][]uint64
	IsSep      bool
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

func (p Plane) unmarshalAxis() (Axis, error) {
	var buf = bytes.NewBuffer(p.Axis)
	dec := gob.NewDecoder(buf)
	var axis Axis
	err := dec.Decode(&axis)
	return axis, err
}

type keyCount struct {
	Key   string
	Count uint
}

func newKeyCount(key string) *keyCount {
	return &keyCount{
		Key:   key,
		Count: 1,
	}
}

type BackUpManage struct {
	Db *dbstore.DB
	sync.Map
}

func NewBackUpManage(db *dbstore.DB) *BackUpManage {
	return &BackUpManage{
		Db:  db,
		Map: sync.Map{},
	}
}

func (b *BackUpManage) InsertPlane(num uint8, time time.Time, axis matrix.Axis) error {
	err := b.SaveKeys(axis.Keys)
	if err != nil {
		return err
	}

	newAxis := Axis{
		Keys:       make([]uint64, len(axis.Keys)),
		ValuesList: axis.ValuesList,
		IsSep:      axis.IsSep,
	}
	for i, key := range axis.Keys {
		newAxis.Keys[i] = getKeyID(key)
	}
	plane, err := NewPlane(num, time, newAxis)
	if err != nil {
		return err
	}
	return b.Db.Create(plane).Error
}

// SaveKeys interns all strings. and
// save the string, not exist in sync.Map, into db.
func (b *BackUpManage) SaveKeys(keys []string) error {
	for i := range keys {
		kc := newKeyCount(keys[i])
		pKeyCount, ok := b.LoadOrStore(keys[i], kc)
		if ok {
			pKeyCount.(*keyCount).Count++
			keys[i] = pKeyCount.(*keyCount).Key
		} else {
			err := b.storeKey(keys[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BackUpManage) storeKey(key string) error {
	id := getKeyID(key)
	return b.Db.Create(&KeyIntern{ID: id, Key: key}).Error
}

func (b *BackUpManage) DeletePlane(num uint8, time time.Time, axis matrix.Axis) error {
	err := b.DeletKeys(axis.Keys)
	if err != nil {
		return err
	}
	return b.Db.
		Where("layer_num = ? AND time = ?", num, time).
		Delete(&Plane{}).
		Error
}

// DeletKeys check the count of every key firstly,
// if count = 1, delete it from sync.Map and db,
// or modify count = count - 1
func (b *BackUpManage) DeletKeys(keys []string) error {
	var dKeys []string
	for _, key := range keys {
		pKeyCount, ok := b.Load(key)
		if !ok {
			panic("unreachable")
		}
		if pKeyCount.(*keyCount).Count > 1 {
			pKeyCount.(*keyCount).Count--
		} else {
			dKeys = append(dKeys, key)
		}
	}
	return b.eraseKey(dKeys)
}

func (b *BackUpManage) eraseKey(keys []string) error {
	IDs := make([]uint64, len(keys))
	for i, key := range keys {
		IDs[i] = getKeyID(key)
	}
	return b.Db.Where(IDs).Delete(&KeyIntern{}).Error
}

// Restore restore all data from db
func (b *BackUpManage) Restore(layerCount int, nowTime time.Time) (matrixPlanes []matrix.Plane) {
	// establish start `Plane` for each layer
	createStartPlanes := func() {
		for i := 0; i < layerCount; i++ {
			err := b.InsertPlane(uint8(i), nowTime, matrix.Axis{})
			log.Debug("Insert startTime plane", zap.Uint8("Num", uint8(i)), zap.Time("StartTime", nowTime), zap.Error(err))
		}
	}

	isExist1, err := CreateTableIfNotExists(b.Db, &Plane{})
	if err != nil {
		log.Panic("Create table Plane fail", zap.Error(err))
	}
	isExist2, err := CreateTableIfNotExists(b.Db, &KeyIntern{})
	if err != nil {
		log.Panic("Create table KeyIntern fail", zap.Error(err))
	}
	if isExist1 != isExist2 {
		log.Panic("table Plane and KeyIntern should exist or not exist at the same time.")
	}
	if !isExist1 {
		createStartPlanes()
		return
	}

	IDKeyMap, err := b.scanAllKeysFromDB()
	if err != nil {
		log.Fatal("scanAllKeysFromDB error", zap.Error(err))
	}

	for num := 0; num < layerCount; num++ {
		planes, err := b.findPlaneOrderByTime(uint8(num))
		if err != nil {
			break
		}
		if len(planes) == 0 && num == 0 {
			createStartPlanes()
			break
		} else if len(planes) == 0 {
			break
		}
		if len(planes) == 1 && num == 0 {
			// no valid data was stored，clear
			err := ClearTable(b.Db, &Plane{})
			log.Debug("Clear table plane")
			if err != nil {
				log.Fatal("Clear table plane", zap.Error(err))
			}
			createStartPlanes()
			break
		}

		times := make([]time.Time, 1, len(planes))
		axes := make([]matrix.Axis, len(planes)-1)
		tempTimes := make([]time.Time, len(planes)-1)
		times[0] = planes[0].Time

		validPlanes := planes[1:]
		for i := range validPlanes {
			tempTimes[i] = validPlanes[i].Time
			axis, err := validPlanes[i].unmarshalAxis()
			if err != nil {
				log.Fatal("unexpected error", zap.Error(err))
			}
			axes[i].ValuesList = axis.ValuesList
			axes[i].Keys = make([]string, len(axis.Keys))
			axes[i].IsSep = axis.IsSep
			for j := range axis.Keys {
				axes[i].Keys[j] = IDKeyMap[axis.Keys[j]]
			}
		}
		times = append(times, tempTimes...)
		mPlane := matrix.Plane{
			Times: times,
			Axes:  axes,
		}
		matrixPlanes = append(matrixPlanes, mPlane)
	}

	// clear table Plane
	err = ClearTable(b.Db, &Plane{})
	if err != nil {
		log.Fatal("Clear table plane error", zap.Error(err))
	}
	// clear table KeyIntern
	err = ClearTable(b.Db, &KeyIntern{})
	if err != nil {
		log.Fatal("Clear table KeyIntern error", zap.Error(err))
	}
	for num := range matrixPlanes {
		// startPlane
		err := b.InsertPlane(uint8(num), matrixPlanes[num].Times[0], matrix.Axis{})
		if err != nil {
			log.Fatal("InsertPlane error", zap.Error(err))
		}
		tempTimes := matrixPlanes[num].Times[1:]
		for i, axis := range matrixPlanes[num].Axes {
			err := b.InsertPlane(uint8(num), tempTimes[i], axis)
			if err != nil {
				log.Fatal("InsertPlane error", zap.Error(err))
			}
		}
	}
	return
}

func (b *BackUpManage) scanAllKeysFromDB() (IDKeyMap map[uint64]string, err error) {
	var keyInterns []KeyIntern
	err = b.Db.Find(&keyInterns).Error
	if err != nil {
		return
	}
	// clear table KeyIntern
	err = ClearTable(b.Db, &KeyIntern{})
	if err != nil {
		return
	}

	IDKeyMap = make(map[uint64]string, len(keyInterns))
	// IDIDMap = make(map[uint64]uint64, len(keyInterns))
	for _, ki := range keyInterns {
		IDKeyMap[ki.ID] = ki.Key
		// IDIDMap[ki.ID] = getKeyID(ki.Key)
		err = b.storeKey(ki.Key)
		if err != nil {
			return
		}
	}
	return
}

func (b *BackUpManage) findPlaneOrderByTime(num uint8) ([]Plane, error) {
	var planes []Plane
	err := b.Db.
		Where("layer_num = ?", num).
		Order("Time").
		Find(&planes).
		Error
	return planes, err
}

// InternKey interns a string.
func (b *BackUpManage) InternKey(key *string) {
	pKeyCount, ok := b.Load(*key)
	if ok {
		*key = pKeyCount.(*keyCount).Key
	}
}

// if the table `table` exists, return true, nil
// or create table `table`
func CreateTableIfNotExists(db *dbstore.DB, table interface{}) (bool, error) {
	if db.HasTable(table) {
		return true, nil
	}
	return false, db.CreateTable(table).Error
}

// ClearTable clear all data in `table`
func ClearTable(db *dbstore.DB, table interface{}) error {
	return db.Delete(table).Error
}

func getKeyID(key string) uint64 {
	p := (*reflect.StringHeader)(unsafe.Pointer(&key))
	return uint64(p.Data)
}