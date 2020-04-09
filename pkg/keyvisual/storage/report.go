package storage

import (
	"bytes"
	"encoding/gob"
	"sort"
	"time"

	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

const tableReportName = "matrix_reports"

type Report struct {
	StartTime time.Time `gorm:"column:start_time"`
	EndTime   time.Time `gorm:"column:end_time;primary_key"`
	Matrix    []byte    `gorm:"column:matrix"`
}

func (Report) TableName() string {
	return tableReportName
}

func NewReport(startTime, endTime time.Time, matrix matrix.Matrix) (*Report, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(matrix)
	if err != nil {
		return nil, err
	}
	return &Report{
		StartTime: startTime,
		EndTime:   endTime,
		Matrix:    buf.Bytes(),
	}, nil
}

// ReportConfig is the configuration of ReportManage.
type ReportConfig struct {
	ReportInterval    time.Duration
	ReportTimeRange   time.Duration
	ReportMaxDisplayY int
	MaxReportNum      int
}

// Check checks whether ReportConfig is legal.
// if not, return the reason
func (cfg ReportConfig) Check() string {
	if cfg.ReportMaxDisplayY <= 0 {
		return "ReportMaxDisplayY must be greater than 0"
	}
	if cfg.MaxReportNum <= 0 {
		return "MaxReportNum must be greater than 0"
	}
	return ""
}

type ReportManage struct {
	Db         *dbstore.DB
	ReportTime time.Time

	ReportEndTimes []time.Time
	Head           int
	Tail           int
	Empty          bool

	ReportInterval    time.Duration
	ReportTimeRange   time.Duration
	ReportMaxDisplayY int
	MaxReportNum      int
}

func NewReportManage(db *dbstore.DB, nowTime time.Time, cfg ReportConfig) *ReportManage {
	reasonMsg := cfg.Check()
	if reasonMsg != "" {
		panic(reasonMsg)
	}
	return &ReportManage{
		Db:                db,
		ReportTime:        nowTime.Add(cfg.ReportInterval),
		ReportEndTimes:    make([]time.Time, cfg.MaxReportNum),
		Head:              0,
		Tail:              0,
		Empty:             true,
		ReportInterval:    cfg.ReportInterval,
		ReportTimeRange:   cfg.ReportTimeRange,
		ReportMaxDisplayY: cfg.ReportMaxDisplayY,
		MaxReportNum:      cfg.MaxReportNum,
	}
}

func (r *ReportManage) RestoreReport() error {
	if !r.Db.HasTable(&Report{}) {
		return r.Db.CreateTable(&Report{}).Error
	}
	var reports []Report
	err := r.Db.Table(tableReportName).Order("start_time").Find(&reports).Error
	length := len(reports)
	log.Debug("RestoreReport", zap.Int("Len", length))
	if err == nil && length != 0 {
		startIdx := 0
		if length > r.MaxReportNum {
			startIdx = length - r.MaxReportNum
			dStartTime := reports[0].EndTime
			dEndTime := reports[startIdx-1].EndTime
			err := r.Db.
				Table(tableReportName).
				Where("end_time >= ? AND end_time <= ?", dStartTime, dEndTime).
				Delete(&Report{}).Error
			if err != nil {
				return err
				// log.Fatal("Delete Report error", zap.Error(err))
			}
			log.Warn("The number of Report in DB is too large. Have deleted extra reports",
				zap.Int("len(reports)", length), zap.Int("MaxReportNum", r.MaxReportNum))
		}

		r.ReportTime = reports[length-1].EndTime.Add(r.ReportInterval)
		r.Head = 0
		r.Tail = (length - startIdx) % r.MaxReportNum
		r.Empty = false
		for i, report := range reports[startIdx:length] {
			r.ReportEndTimes[i] = report.EndTime
		}
	}
	return err
}

func (r *ReportManage) IsNeedReport(nowTime time.Time) bool {
	return !r.ReportTime.After(nowTime)
}

func (r *ReportManage) InsertReport(matrix matrix.Matrix) error {
	if r.Head == r.Tail && !r.Empty {
		err := r.DeleteReport()
		if err != nil {
			return err
		}
	}
	startTime := r.ReportTime.Add(-r.ReportInterval)
	endTime := r.ReportTime
	report, err := NewReport(startTime, endTime, matrix)
	if err != nil {
		return err
	}
	err = r.Db.Table(tableReportName).Create(report).Error
	if err != nil {
		return err
	}
	r.ReportEndTimes[r.Tail] = endTime
	r.Empty = false
	r.Tail = (r.Tail + 1) % r.MaxReportNum
	// update ReportTime
	r.ReportTime = r.ReportTime.Add(r.ReportInterval)
	return nil
}

func (r *ReportManage) DeleteReport() error {
	if r.Empty {
		return nil
	}
	EndTime := r.ReportEndTimes[r.Head]
	err := r.Db.
		Table(tableReportName).
		Where("end_time == ?", EndTime).
		Delete(&Report{}).Error
	if err != nil {
		return err
	}
	r.Head = (r.Head + 1) % r.MaxReportNum
	if r.Head == r.Tail {
		r.Empty = true
	}
	return nil
}

func (r *ReportManage) FindReport(endTime time.Time) (matrix matrix.Matrix, isFind bool, err error) {
	isFind = false
	err = nil
	if r.Empty {
		return
	}
	num := r.MaxReportNum
	if r.Tail != r.Head {
		num = (r.Tail - r.Head + r.MaxReportNum) % r.MaxReportNum
	}
	idx := sort.Search(num, func(i int) bool {
		return !r.ReportEndTimes[(r.Head+i)%r.MaxReportNum].Before(endTime)
	})
	if idx == num {
		return
	}
	targetTime := r.ReportEndTimes[(r.Head+idx)%r.MaxReportNum]
	var report Report
	err = r.Db.Table(tableReportName).Where("end_time = ?", targetTime).Find(&report).Error
	if err != nil {
		return
	}
	isFind = true
	var buf = bytes.NewBuffer(report.Matrix)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(&matrix)
	return
}