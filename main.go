package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type Handler interface {
	Write(p []byte) (n int, err error)
	Close() error
}

type TimeRotatingFileHandler struct {
	fd *os.File

	baseName   string
	interval   int64
	suffix     string
	rolloverAt int64
	mux sync.Mutex
}

const (
	WhenSecond = iota
	WhenMinute
	WhenHour
	WhenDay
)

func newLogHandler(baseName string, when int8, interval int) (*TimeRotatingFileHandler, error) {
	dir := path.Dir(baseName)
	os.MkdirAll(dir, 0777)

	h := new(TimeRotatingFileHandler)

	h.baseName = baseName

	switch when {
	case WhenSecond:
		h.interval = 1
		h.suffix = "2006-01-02_15-04-05"
	case WhenMinute:
		h.interval = 60
		h.suffix = "2006-01-02_15-04"
	case WhenHour:
		h.interval = 3600
		h.suffix = "2006-01-02_15"
	case WhenDay:
		h.interval = 3600 * 24
		h.suffix = "2006-01-02"
	default:
		return nil, fmt.Errorf("invalid when_rotate: %d", when)
	}

	h.interval = h.interval * int64(interval)

	var err error
	h.fd, err = os.OpenFile(h.baseName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	fInfo, _ := h.fd.Stat()
	h.rolloverAt = fInfo.ModTime().Unix() + h.interval

	return h, nil
}

func (l *TimeRotatingFileHandler) doRollover() {
	now := time.Now()

	if l.rolloverAt <= now.Unix() {
		fName := l.baseName + now.Format(l.suffix)
		l.fd.Close()
		e := os.Rename(l.baseName, fName)
		if e != nil {
			panic(e)
		}

		l.fd, _ = os.OpenFile(l.baseName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

		l.rolloverAt = time.Now().Unix() + l.interval
	}
}

func (l *TimeRotatingFileHandler) Write(p []byte) (n int, err error) {
	l.mux.Lock()
	l.doRollover()
	l.mux.Unlock()
	return fmt.Fprintln(l.fd, string(p))
}

func (l *TimeRotatingFileHandler) Close() (error) {
	return l.fd.Close()
}


type ConfigStruct struct {
	logPath    string
	serverIp   string
	serverPort string
	handledNum uint64
	startTime  time.Time
}

func (s *ConfigStruct) Setup() {
	flag.StringVar(&s.logPath, "log-path", "/var/log/kong-log", "log path")
	flag.StringVar(&s.serverIp, "server-ip", "127.0.0.1", "listen ip")
	flag.StringVar(&s.serverPort, "server-port", "9513", "listen port")
}


func main() {
	c := &ConfigStruct{}
	c.Setup()
	flag.Parse()

	handler, err := newLogHandler(c.logPath, WhenHour, 3600)
	if err != nil {
		panic(err)
	}

	time.Now()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	workerNum := 2
	channel := make(chan []byte, workerNum)
	defer close(channel)

	for i := 0; i < workerNum; i++ {
		go handleLog(channel, handler, c)
	}

	r.POST("/kong-log", func(c *gin.Context) {
		data, err1 := c.GetRawData()
		if err1 != nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}

		//send data
		channel <- data

		c.JSON(http.StatusOK, gin.H{"status": "ok"})

	})

	r.GET("/kong-log-stat", func(content *gin.Context) {
		content.JSON(http.StatusOK, gin.H{
			"log-path": c.logPath,
			"server-ip": c.serverIp,
			"server-port": c.serverPort,
			"start-time": c.startTime,
			"duration": time.Now().Sub(c.startTime).String(),
			"handled": c.handledNum,
		})
	})



	c.startTime = time.Now()
	if err := r.Run(c.serverIp+":"+c.serverPort); err != nil {
		panic(err)
	}
}

func handleLog(logChannel chan []byte, writer Handler,c *ConfigStruct) {
	for json := range logChannel {
		writer.Write(json)
		c.handledNum++
	}
}
