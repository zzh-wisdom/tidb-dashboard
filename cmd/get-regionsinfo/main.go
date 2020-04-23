package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/pingcap/log"
	"go.uber.org/zap"
)

const AllFilePath = "./data"

var (
	PDEndPoint = flag.String("pd", "http://127.0.0.1:2379", "PD endpoint")
	Interval   = flag.Duration("interval", time.Minute, "Interval to collect metrics")
	Dir        = flag.String("dir", "", "the folder in ./data where saving regionsInfo")
)

type InfoUnit struct {
	closer    io.ReadCloser
	startTime time.Time
}

func NewInfoUnit(closer io.ReadCloser, startTime time.Time) *InfoUnit {
	return &InfoUnit{
		closer:    closer,
		startTime: startTime,
	}
}

func dataWrite(wg *sync.WaitGroup, ch chan *InfoUnit) {
	wg.Add(1)
	var startTime time.Time
	var endTime time.Time
	num := 0
	for info := range ch {
		if info == nil {
			break
		}
		num++
		if num == 1 {
			startTime = info.startTime
		}
		endTime = info.startTime

		fileName := info.startTime.Format("20060102-15-04-05.json")
		filePath := fmt.Sprintf("%s/%s/%s", AllFilePath, *Dir, fileName)
		file, err := os.Create(filePath)
		if err != nil {
			panic(fmt.Sprintf("Create file %s err %s", filePath, err.Error()))
		}
		buf, err := ioutil.ReadAll(info.closer)
		info.closer.Close()
		if err != nil {
			panic(err.Error())
		}
		log.Debug("New regionsInfo", zap.Time("Time", endTime), zap.Int("Bytes num", len(buf)))
		_, _ = file.Write(buf)
		file.Close()
	}
	readMeFile := fmt.Sprintf("%s/%s/readme.txt", AllFilePath, *Dir)
	file, err := os.Create(readMeFile)
	if err != nil {
		panic(fmt.Sprintf("Create file %s err %s", readMeFile, err.Error()))
	}
	readMeInfo := fmt.Sprintf("./bin/tidb-dashboard --debug --keyviz-file-start %v --keyviz-file-end %v --keyviz-file-path %s/%s --data-interval %s",
		startTime.Unix(), endTime.Unix(), AllFilePath, *Dir, Interval.String())
	_, _ = file.WriteString(readMeInfo)
	file.Close()

	wg.Add(-1)
}

func main() {
	// Flushing any buffered log entries
	defer log.Sync() //nolint:errcheck
	log.SetLevel(zapcore.DebugLevel)

	flag.Parse()
	if *Dir == "" {
		panic("Parameter --dir can't be empty.")
	}
	// 判断文件路径是否存在文件
	filePath := fmt.Sprintf("%s/%s", AllFilePath, *Dir)
	_, err := os.Stat(filePath)
	if err == nil {
		files, _ := ioutil.ReadDir(filePath)
		if len(files) != 0 {
			panic(fmt.Sprintf("The patn %s should be empty, files num %v", filePath, len(files)))
		}
	}
	if os.IsNotExist(err) {
		err := os.Mkdir(filePath, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	addr := fmt.Sprintf("%s/pd/api/v1/regions", *PDEndPoint)
	client := &http.Client{}
	getRegionsInfo := func() (closer io.ReadCloser, err error) {
		resp, err := client.Get(addr) //nolint:bodyclose,gosec
		if err == nil {
			return resp.Body, nil
		}
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)
		<-sc
		cancel()
	}()

	var wg sync.WaitGroup
	ch := make(chan *InfoUnit, 2)
	go dataWrite(&wg, ch)

	ticker := time.NewTicker(*Interval)
	defer ticker.Stop()
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			ch <- nil
			wg.Wait()
			return
		case <-ticker.C:
			closer, err := getRegionsInfo()
			if err != nil {
				log.Error("can not get RegionsInfo", zap.Error(err))
				continue
			}
			ch <- NewInfoUnit(closer, startTime)
			startTime = startTime.Add(*Interval)
		}
	}

}
