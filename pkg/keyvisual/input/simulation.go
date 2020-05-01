package input

import (
	"context"
	"fmt"
	"time"

	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/storage"
)

const (
	// RegionStandardSize = 96 * 2<<10 * 2<<10
	RegionMaxSize = 144 * (1 << 10) * (1 << 10)

	Interval = time.Minute
	// ThroughputPerWork = RegionMaxSize * SplitTimesPerWork

	// RowKeySize = 19
	RowKVSize = 4 * 2 << 10 //* 2<<10
	// RowNumPerWork = ThroughputPerWork / RowKVSize
	// RowNumPerRegion = RegionStandardSize / RowKVSize

	// IndexKeySize = 28
	IndexKVSize = 1 //* 2<<10
	// IndexNumPerWork = RowNumPerWork
	MaxIndexNumberRegion = RegionMaxSize / IndexKVSize

	RowAndIndexRatio = RowKVSize / IndexKVSize

	//for faster
	RegionInfoPerWork         = RegionMaxSize / 2
	RowNumPerSplit            = RegionMaxSize / RowKVSize / 2
	IndexNumPerSplit          = RowNumPerSplit
	TableNum                  = 4
	SplitTimesPerTablePerWork = 1
	InitRegionNum             = 1
	WorkTimes                 = 10 //  5 * 7 * 24 * 60
	// DetaRegionNUm = WorkTimes * MaxSplitTimesPerWork
)

type Region struct {
	StartKey     string
	StartID      int64
	KVNum        int64
	WrittenBytes uint64
}

func NewRegion(startKey string, startID int64, kvNum int64, writtenBytes uint64) *Region {
	return &Region{
		StartKey:     startKey,
		StartID:      startID,
		KVNum:        kvNum,
		WrittenBytes: writtenBytes,
	}
}

func (r *Region) cleanInfo() {
	r.WrittenBytes = 0
}

func (r *Region) resize(regionNum int64, tableID int64) []*Region {
	if regionNum <= 1 {
		return nil
	}
	step := r.KVNum/regionNum + 1
	regions := make([]*Region, 1, regionNum)
	regions[0] = &Region{
		StartKey:     r.StartKey,
		StartID:      r.StartID,
		KVNum:        step,
		WrittenBytes: 0,
	}
	nowNum := step
	for i := int64(0); i < regionNum-1; i++ {
		startID := r.StartID + nowNum
		startKey := buildRowKey(tableID, startID)
		num := step
		if step+nowNum > r.KVNum {
			num = r.KVNum - nowNum
		}
		nowNum += num

		region := &Region{
			StartKey:     startKey,
			StartID:      startID,
			KVNum:        num,
			WrittenBytes: 0,
		}
		regions = append(regions, region)
	}
	return regions
}

type TableType int

const (
	IndexTable TableType = 0
	RowTable   TableType = 1
)

type Table struct {
	ID        int64
	tableType TableType

	regions    []*Region
	indexTable *Table

	notZeroRegionsIndex []int

	splitTimesPerWork int
}

func NewTable(tableID int64, regionsLen, regionsCap int64, splitTimesPerWork int) *Table {
	// 初始时有RegionInfoPerWork的写入量，方便后续统一处理
	table := &Table{
		ID:                  tableID,
		tableType:           RowTable,
		regions:             make([]*Region, regionsLen, regionsCap),
		indexTable:          nil,
		notZeroRegionsIndex: make([]int, 0, SplitTimesPerTablePerWork),
		splitTimesPerWork:   splitTimesPerWork,
	}
	var startID int64 = 0
	for i := int64(0); i < regionsLen; i++ {
		startKey := buildRowKey(table.ID, startID)
		table.regions[i] = NewRegion(startKey, startID, RowNumPerSplit, 0)
		startID += RowNumPerSplit
	}

	indexRegionCap := regionsCap/RowAndIndexRatio + 1
	table.indexTable = &Table{
		ID:                  0,
		tableType:           IndexTable,
		regions:             make([]*Region, 1, indexRegionCap),
		indexTable:          nil,
		notZeroRegionsIndex: make([]int, 0, 1),
	}
	startKey := buildIndexKey(table.ID, 0)
	table.indexTable.regions[0] = NewRegion(startKey, 0, 0, 0)
	table.indexTable.addIndex(table.ID, regionsLen*RowNumPerSplit)
	table.indexTable.cleanInfo()

	return table
}

func (t *Table) addIndex(tableID int64, num int64) {
	lastIndex := len(t.regions) - 1
	lastRegion := t.regions[lastIndex]
	if lastRegion.KVNum+num >= MaxIndexNumberRegion {
		if lastRegion.KVNum < MaxIndexNumberRegion/2 {
			lastRegion.WrittenBytes += uint64((MaxIndexNumberRegion/2 - lastRegion.KVNum) * IndexKVSize)
			lastRegion.KVNum = MaxIndexNumberRegion / 2

			t.notZeroRegionsIndex = append(t.notZeroRegionsIndex, lastIndex)
		}
		nextStartKey := buildIndexKey(tableID, t.ID)
		nextKVNum := num
		if num > MaxIndexNumberRegion/2 {
			nextKVNum = MaxIndexNumberRegion / 2
		}
		nextStartID := lastRegion.StartID + MaxIndexNumberRegion/2
		nextRegion := NewRegion(nextStartKey, nextStartID, nextKVNum, uint64(nextKVNum*IndexKVSize))
		t.regions = append(t.regions, nextRegion)

		t.notZeroRegionsIndex = append(t.notZeroRegionsIndex, lastIndex+1)
	} else {
		lastRegion.KVNum += num
		lastRegion.WrittenBytes += uint64(num * IndexKVSize)

		t.notZeroRegionsIndex = append(t.notZeroRegionsIndex, lastIndex)
	}
	//fmt.Println("addIndex", t.notZeroRegionsIndex)
}

func (t *Table) splitLastRowRegion() {
	length := len(t.regions)
	//t.regions[length-1].WrittenBytes = RegionInfoPerWork
	//t.regions[length-1].KVNum = RowNumPerSplit
	nextStartID := t.regions[length-1].StartID + t.regions[length-1].KVNum
	nextStartKey := buildRowKey(t.ID, nextStartID)
	nextRegion := NewRegion(nextStartKey, nextStartID, RowNumPerSplit, RegionInfoPerWork)
	t.regions = append(t.regions, nextRegion)

	// 修改索引表
	t.indexTable.addIndex(t.ID, IndexNumPerSplit)

	t.notZeroRegionsIndex = append(t.notZeroRegionsIndex, length)
	//fmt.Println("addIndex", t.notZeroRegionsIndex)
}

func (t *Table) work() {
	if len(t.notZeroRegionsIndex) != 0 {
		panic("Table info is not clean before working")
	}
	for i := 0; i < t.splitTimesPerWork; i++ {
		t.splitLastRowRegion()
	}
}

func (t *Table) cleanInfo() {
	if t.indexTable != nil {
		t.indexTable.cleanInfo()
	}
	for _, n := range t.notZeroRegionsIndex {
		t.regions[n].cleanInfo()
	}
	t.notZeroRegionsIndex = t.notZeroRegionsIndex[:0]
}

func (t *Table) getRegionsInfo() *RegionsInfo {
	var indexRegionsInfo = &RegionsInfo{}
	//if t.indexTable != nil {
	//	indexRegionsInfo = t.indexTable.getRegionsInfo()
	//}

	length := len(t.regions)
	rowRegionsInfo := &RegionsInfo{
		Count:   length,
		Regions: make([]*RegionInfo, length),
	}
	for i, region := range t.regions {
		rowRegionsInfo.Regions[i] = &RegionInfo{
			StartKey:     region.StartKey,
			WrittenBytes: region.WrittenBytes,
		}
		if i > 0 {
			rowRegionsInfo.Regions[i-1].EndKey = region.StartKey
		}
	}
	rowRegionsInfo.Regions[length-1].EndKey = ""

	regios := &RegionsInfo{
		Count:   indexRegionsInfo.Count + rowRegionsInfo.Count,
		Regions: append(indexRegionsInfo.Regions, rowRegionsInfo.Regions...),
	}
	return regios
}

func (t *Table) resize(regionNum int64) {
	fullregionNum := len(t.regions) - 1
	resizePerRegion := regionNum/int64(fullregionNum) + 1
	var regions []*Region
	for i := 1; i < fullregionNum; i++ {
		temp := t.regions[i].resize(resizePerRegion, t.ID)
		regions = append(regions, temp...)
	}
	regions = append(regions, t.regions[fullregionNum])
	t.regions = regions
}

type SimulationDB struct {
	Tables []*Table

	IsResize bool
	NowTime  time.Time
	//TargetRegionNum int64
}

func NewSimulationDB(tableNum int64, initRegionNum int64) *SimulationDB {
	s := &SimulationDB{
		Tables:  make([]*Table, tableNum),
		NowTime: time.Now().Add(-Interval * WorkTimes),
	}

	//regionNumPerTable := targetRegionNum / tableNum + 1
	//initTableRegionNum := int64(1)
	//splitTimesPerWork := MaxSplitTimesPerWork
	//for ; splitTimesPerWork > 1 ; splitTimesPerWork-- {
	//	if regionNumPerTable > int64(splitTimesPerWork * WorkTimes) {
	//		initTableRegionNum = regionNumPerTable - int64(splitTimesPerWork * WorkTimes)
	//		break
	//	}
	//}
	//if splitTimesPerWork < 1 || initTableRegionNum < 1 {
	//	panic("NewSimulationDB error")
	//}

	initTableRegionNum := initRegionNum/tableNum + 1
	for i := range s.Tables {
		s.Tables[i] = NewTable(int64(i), initTableRegionNum, initTableRegionNum, SplitTimesPerTablePerWork)
	}
	log.Info("Simulation", zap.Int64("TableNum", tableNum), zap.Int64("InitTableRegionNum", initTableRegionNum),
		zap.Int("SplitTimesPerTablePerWork", SplitTimesPerTablePerWork), zap.Int("WorkTimes", WorkTimes))
	return s
}

func (s *SimulationDB) work() {
	for _, t := range s.Tables {
		t.work()
	}
}

func (s *SimulationDB) cleanInfo() {
	for _, t := range s.Tables {
		t.cleanInfo()
	}
}

func (s *SimulationDB) getRegionsInfo() *RegionsInfo {
	regions := &RegionsInfo{}
	for _, t := range s.Tables {
		info := t.getRegionsInfo()
		regions.Count += info.Count
		regions.Regions = append(regions.Regions, info.Regions...)
	}
	//sort.Slice(regions.Regions, func(i, j int) bool {
	//	return regions.Regions[i].StartKey < regions.Regions[j].StartKey
	//})
	return regions
}

func (s *SimulationDB) GetStartTime() time.Time {
	return s.NowTime
}

func (s *SimulationDB) Background(ctx context.Context, stat *storage.Stat) {
	for i := 0; i < WorkTimes; i++ {
		s.cleanInfo()
		s.work()
		regions := s.getRegionsInfo()
		s.NowTime = s.NowTime.Add(Interval)

		stat.FillData(regions, s.NowTime)
	}
	RegionNum := 0
	for _, t := range s.Tables {
		RegionNum += len(t.regions)
		RegionNum += len(t.indexTable.regions)
	}
	log.Info("Background end", zap.Int("RegionNum", RegionNum))
}

func (s *SimulationDB) WorkAndGetInfo() (*RegionsInfo, time.Time) {
	s.cleanInfo()
	s.work()
	regions := s.getRegionsInfo()
	s.NowTime = s.NowTime.Add(Interval)
	return regions, s.NowTime
}

func (s *SimulationDB) Resize(regionNum int64) {
	if s.IsResize {
		panic("Can't resize twice")
	}
	RegionNum := 0
	for _, t := range s.Tables {
		RegionNum += len(t.regions)
		RegionNum += len(t.indexTable.regions)
	}
	if regionNum <= int64(RegionNum) {
		return
	}
	s.IsResize = true

	tableNum := len(s.Tables)
	resizeRegionNumPerTable := regionNum/int64(tableNum) + 1
	for _, t := range s.Tables {
		t.resize(resizeRegionNumPerTable)
	}
}

var (
	tablePrefix    = []byte{'t'}
	rowPrefixSep   = []byte("_r")
	indexPrefixSep = []byte("_i")
)

func buildRowKey(tableID, rowID int64) string {
	// return regionpkg.String(tablePrefix)+strconv.Itoa(int(tableID))+regionpkg.String(rowPrefixSep)+strconv.Itoa(int(rowID))
	//tableBytes := codec.EncodeInt(tablePrefix, tableID)
	//rowBytes := codec.EncodeInt(rowPrefixSep, rowID)
	//result := append(tableBytes, rowBytes...)
	//return regionpkg.String(codec.EncodeBytes(result))
	return fmt.Sprintf("%08d/%08d/%08d", tableID, tableID, rowID)
}

func buildIndexKey(tableID, indexID int64) string {
	// return regionpkg.String(tablePrefix)+strconv.Itoa(int(tableID))+regionpkg.String(indexPrefixSep)+strconv.Itoa(int(indexID))
	//tableBytes := codec.EncodeInt(tablePrefix, tableID)
	//indexBytes := codec.EncodeInt(indexPrefixSep, indexID)
	//result := append(tableBytes, indexBytes...)
	//return regionpkg.String(codec.EncodeBytes(result))
	return fmt.Sprintf("%08d/%08d/%08d", tableID, tableID, indexID)
}
