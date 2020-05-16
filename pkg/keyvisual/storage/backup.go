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
)

const (
	tableDbPlaneName   = "db_planes"
	tableKeyInternName = "key_interns"
)

type KeyIntern struct {
	ID  uint64 `gorm:"primary_key"`
	Key string
}

func (KeyIntern) TableName() string {
	return tableKeyInternName
}

type DbAxis struct {
	KeysList   [][]uint64
	ValuesList [][]uint64
	IsSep      bool
}

type DbPlane struct {
	LayerNum      uint8 `gorm:"column:layer_num"`
	Time          time.Time
	AxisGobEncode []byte
}

func (DbPlane) TableName() string {
	return tableDbPlaneName
}

func NewDbPlane(num uint8, time time.Time, axis interface{}) (*DbPlane, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(axis)
	if err != nil {
		return nil, err
	}
	return &DbPlane{
		num,
		time,
		buf.Bytes(),
	}, nil
}

func (dp DbPlane) unmarshalDbAxis() (DbAxis, error) {
	var buf = bytes.NewBuffer(dp.AxisGobEncode)
	dec := gob.NewDecoder(buf)
	var axis DbAxis
	err := dec.Decode(&axis)
	return axis, err
}

func (dp DbPlane) unmarshalMemAxis() (MemAxis, error) {
	var buf = bytes.NewBuffer(dp.AxisGobEncode)
	dec := gob.NewDecoder(buf)
	var axis MemAxis
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
	IsKeyIntern bool
}

func NewBackUpManage(db *dbstore.DB, isKeyIntern bool) *BackUpManage {
	log.Info("NewBackUpManage", zap.Bool("IsKeyIntern", isKeyIntern))
	return &BackUpManage{
		Db:          db,
		Map:         sync.Map{},
		IsKeyIntern: isKeyIntern,
		//IsOpen: isOpen,
	}
}

func (b *BackUpManage) InsertDbPlane(num uint8, time time.Time, axis MemAxis) error {
	if b.IsKeyIntern {
		err := b.SaveKeys(axis.KeysList)
		if err != nil {
			return err
		}
		newAxis := DbAxis{
			KeysList:   make([][]uint64, len(axis.KeysList)),
			ValuesList: axis.ValuesList,
			IsSep:      axis.IsSep,
		}
		for i, keys := range axis.KeysList {
			ids := make([]uint64, len(keys))
			for j, key := range keys {
				ids[j] = getKeyID(key)
			}
			newAxis.KeysList[i] = ids
		}
		plane, err := NewDbPlane(num, time, newAxis)
		if err != nil {
			return err
		}
		return b.Db.Create(plane).Error
	}

	plane, err := NewDbPlane(num, time, axis)
	if err != nil {
		return err
	}
	return b.Db.Create(plane).Error
}

// SaveKeys interns all strings. and
// save the string, not exist in sync.Map, into db.
func (b *BackUpManage) SaveKeys(keysList [][]string) error {
	for _, keys := range keysList {
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
	}
	return nil
}

func (b *BackUpManage) storeKey(key string) error {
	//if !b.IsOpen {
	//	return nil
	//}
	id := getKeyID(key)
	return b.Db.Create(&KeyIntern{ID: id, Key: key}).Error
}

func (b *BackUpManage) DeleteDbPlane(num uint8, time time.Time, axis MemAxis) error {
	if b.IsKeyIntern {
		err := b.DeletKeys(axis.KeysList)
		if err != nil {
			return err
		}
	}
	return b.Db.
		Where("layer_num = ? AND time = ?", num, time).
		Delete(&DbPlane{}).
		Error
}

// DeletKeys check the count of every key firstly,
// if count = 1, delete it from sync.Map and db,
// or modify count = count - 1
func (b *BackUpManage) DeletKeys(keysList [][]string) error {
	var dKeys []string
	for _, keys := range keysList {
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
	}
	return b.eraseKey(dKeys)
}

func (b *BackUpManage) eraseKey(keys []string) error {
	//if !b.IsOpen {
	//	return nil
	//}
	IDs := make([]uint64, len(keys))
	for i, key := range keys {
		IDs[i] = getKeyID(key)
	}
	return b.Db.Where(IDs).Delete(&KeyIntern{}).Error
}

func (b *BackUpManage) CreateTablesIfNotExists() bool {
	isExist1, err := CreateTableIfNotExists(b.Db, &DbPlane{})
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
	return isExist1
}

// Restore restore all data from db
func (b *BackUpManage) Restore(stat *Stat, nowTime time.Time) {
	if b.IsKeyIntern {
		b.RestoreWithKeyIntern(stat, nowTime)
	} else {
		b.RestoreWithNoKeyIntern(stat, nowTime)
	}
}

// Restore restore all data from db
func (b *BackUpManage) RestoreWithKeyIntern(stat *Stat, nowTime time.Time) {
	//if !b.IsOpen {
	//	return
	//}
	layerCount := len(stat.layers)
	// establish start `Plane` for each layer
	createStartDbPlanes := func() {
		for i := 0; i < layerCount; i++ {
			err := b.InsertDbPlane(uint8(i), nowTime, MemAxis{})
			log.Debug("Insert startTime plane", zap.Uint8("Num", uint8(i)), zap.Time("StartTime", nowTime), zap.Error(err))
		}
	}

	isExist := b.CreateTablesIfNotExists()
	if !isExist {
		createStartDbPlanes()
		return
	}

	IDKeyMap, err := b.scanAllKeysFromDB()
	if err != nil {
		log.Fatal("scanAllKeysFromDB error", zap.Error(err))
	}

	var timesList [][]time.Time
	var memAxesList [][]MemAxis

	for num := 0; num < layerCount; num++ {
		planes, err := b.findDbPlaneOrderByTime(uint8(num))
		if err != nil {
			break
		}
		if len(planes) == 0 && num == 0 {
			createStartDbPlanes()
			break
		} else if len(planes) == 0 {
			break
		}
		if len(planes) == 1 && num == 0 {
			// no valid data was stored，clear
			err := ClearTable(b.Db, &DbPlane{})
			log.Debug("Clear table plane")
			if err != nil {
				log.Fatal("Clear table plane", zap.Error(err))
			}
			createStartDbPlanes()
			return
		}

		log.Debug("Load planes", zap.Uint8("Num", uint8(num)), zap.Int("Len", len(planes)-1))
		stat.layers[num].Empty = len(planes) <= 1
		stat.layers[num].StartTime = planes[0].Time
		stat.layers[num].Head = 0
		n := len(planes) - 1
		if n > stat.layers[num].Len {
			log.Fatal("n cannot be longer than layers[num].Len", zap.Int("n", n), zap.Int("layers[num].Len", stat.layers[num].Len), zap.Int("num", num))
		}
		stat.layers[num].EndTime = planes[n].Time
		stat.layers[num].Tail = (stat.layers[num].Head + n) % stat.layers[num].Len

		times := []time.Time{planes[0].Time}
		axes := []MemAxis{{}}
		tempTimes := make([]time.Time, len(planes)-1)
		tempAxes := make([]MemAxis, len(planes)-1)
		for i, plane := range planes[1:] {
			tempTimes[i] = plane.Time
			axis, err := plane.unmarshalDbAxis()
			if err != nil {
				log.Fatal("unexpected error", zap.Error(err))
			}
			tempAxes[i].ValuesList = axis.ValuesList
			tempAxes[i].KeysList = make([][]string, len(axis.KeysList))
			tempAxes[i].IsSep = axis.IsSep
			for j := range axis.KeysList {
				tempAxes[i].KeysList[j] = make([]string, len(axis.KeysList[j]))
				for k := range axis.KeysList[j] {
					tempAxes[i].KeysList[j][k] = IDKeyMap[axis.KeysList[j][k]]
				}
			}

			stat.layers[num].RingTimes[i] = plane.Time
			stat.layers[num].RingAxes[i] = tempAxes[i]
		}

		times = append(times, tempTimes...)
		axes = append(axes, tempAxes...)
		timesList = append(timesList, times)
		memAxesList = append(memAxesList, axes)
	}

	b.clean()
	for num := range memAxesList {
		for i, axis := range memAxesList[num] {
			err := b.InsertDbPlane(uint8(num), timesList[num][i], axis)
			if err != nil {
				log.Fatal("InsertDbPlane error", zap.Error(err))
			}
		}
	}
}

// Restore restore all data from db
func (b *BackUpManage) RestoreWithNoKeyIntern(stat *Stat, nowTime time.Time) {
	layerCount := len(stat.layers)
	// establish start `Plane` for each layer
	createStartDbPlanes := func() {
		for i := 0; i < layerCount; i++ {
			err := b.InsertDbPlane(uint8(i), nowTime, MemAxis{})
			log.Debug("Insert startTime plane", zap.Uint8("Num", uint8(i)), zap.Time("StartTime", nowTime), zap.Error(err))
		}
	}

	isExist := b.CreateTablesIfNotExists()
	if !isExist {
		createStartDbPlanes()
		return
	}

	for num := 0; num < layerCount; num++ {
		planes, err := b.findDbPlaneOrderByTime(uint8(num))
		if err != nil {
			break
		}
		if len(planes) == 0 && num == 0 {
			createStartDbPlanes()
			break
		} else if len(planes) == 0 {
			break
		}
		if len(planes) == 1 && num == 0 {
			// no valid data was stored，clear
			err := ClearTable(b.Db, &DbPlane{})
			log.Debug("Clear table plane")
			if err != nil {
				log.Fatal("Clear table plane", zap.Error(err))
			}
			createStartDbPlanes()
			break
		}

		log.Debug("Load planes", zap.Uint8("Num", uint8(num)), zap.Int("Len", len(planes)-1))
		stat.layers[num].Empty = len(planes) <= 1
		stat.layers[num].StartTime = planes[0].Time
		stat.layers[num].Head = 0
		n := len(planes) - 1
		if n > stat.layers[num].Len {
			log.Fatal("n cannot be longer than layers[num].Len", zap.Int("n", n), zap.Int("layers[num].Len", stat.layers[num].Len), zap.Int("num", num))
		}
		stat.layers[num].EndTime = planes[n].Time
		stat.layers[num].Tail = (stat.layers[num].Head + n) % stat.layers[num].Len

		for i, plane := range planes[1:] {
			axis, err := plane.unmarshalMemAxis()
			if err != nil {
				log.Fatal("unexpected error", zap.Error(err))
			}
			stat.layers[num].RingTimes[i] = plane.Time
			stat.layers[num].RingAxes[i] = axis
		}
	}
}

func (b *BackUpManage) clean() {
	// clear table Plane
	_ = ClearTable(b.Db, &DbPlane{})
	//if err != nil {
	//	log.Fatal("Clear table plane error", zap.Error(err))
	//}
	// clear table KeyIntern
	_ = ClearTable(b.Db, &KeyIntern{})
	//if err != nil {
	//	log.Fatal("Clear table KeyIntern error", zap.Error(err))
	//}
}

func (b *BackUpManage) scanAllKeysFromDB() (IDKeyMap map[uint64]string, err error) {
	//if !b.IsOpen {
	//	return nil, nil
	//}
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

func (b *BackUpManage) findDbPlaneOrderByTime(num uint8) ([]DbPlane, error) {
	//if !b.IsOpen {
	//	return nil, nil
	//}
	var planes []DbPlane
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