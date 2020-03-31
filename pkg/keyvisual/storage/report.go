package storage

import (
	"bytes"
	"encoding/gob"
	"sort"
	"time"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

const (
	reportInterval   = time.Minute
	reportTimeRange  = time.Minute

	reportMaxDisplayY = 1536
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

type ReportManage struct {
	Db               *dbstore.DB
	ReportTime       time.Time
	ReportEndTimes []time.Time
}

func NewReportManage(db *dbstore.DB, reportTime time.Time) *ReportManage {
	return &ReportManage{
		Db:               db,
		ReportTime:       reportTime,
		ReportEndTimes: make([]time.Time, 0, 2),
	}
}

func (r *ReportManage) RestoreReport() error {
	if !r.Db.HasTable(&Report{}) {
		return r.Db.CreateTable(&Report{}).Error
	}
	var reports []Report
	err := r.Db.Table(tableReportName).Order("start_time").Find(&reports).Error
	if err == nil && len(reports) != 0 {
		r.ReportTime = reports[len(reports)-1].EndTime.Add(reportInterval)
		r.ReportEndTimes = make([]time.Time, len(reports))
		for i, report := range reports {
			r.ReportEndTimes[i] = report.EndTime
		}
	}
	return nil
}

func (r *ReportManage) IsNeedReport(nowTime time.Time) bool {
	return !r.ReportTime.After(nowTime)
}

// Not be used
func (r *ReportManage) UpdateReportTime(time time.Time) {
	nextTime := r.ReportTime.Add(reportInterval)
	if nextTime.After(time) {
		r.ReportTime = nextTime
	} else {
		r.ReportTime = time
	}
}

func (r *ReportManage) InsertReport(matrix matrix.Matrix) error {
	startTime := r.ReportTime.Add(-reportInterval)
	endTime := r.ReportTime
	report, err := NewReport(startTime, endTime, matrix)
	if err != nil {
		return err
	}
	err = r.Db.Table(tableReportName).Create(report).Error
	if err != nil {
		return err
	}
	r.ReportEndTimes = append(r.ReportEndTimes, endTime)
	r.ReportTime = r.ReportTime.Add(reportInterval)
	return nil
}

func (r *ReportManage) FindReport(endTime time.Time) (matrix matrix.Matrix, isFind bool, err error) {
	isFind = false
	err = nil
	if len(r.ReportEndTimes) == 0 {
		return
	}
	idx := sort.Search(len(r.ReportEndTimes), func(i int) bool {
		return !r.ReportEndTimes[i].Before(endTime)
	})
	if idx == len(r.ReportEndTimes) {
		return
	}
	var report Report
	err = r.Db.Table(tableReportName).Where("end_time = ?", r.ReportEndTimes[idx]).Find(&report).Error
	if err != nil {
		return
	}
	isFind = true
	var buf = bytes.NewBuffer(report.Matrix)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(&matrix)
	return
}