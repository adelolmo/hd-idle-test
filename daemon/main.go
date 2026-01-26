package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madflojo/tasks"
)

const (
	socketFile          = "/tmp/hdtd.sock"
	hdidleLogFile       = "/var/log/hd-idle.log"
	hdidleStdoutFile    = "/tmp/hd-idle.out"
	diskMappingFileName = "disk_mapping.txt"
)

var (
	recording          = make(chan bool, 1)
	hdidleStdoutLength = 0
	hdidleLogLength    = 0
)

func main() {
	router := gin.Default()

	scheduler := tasks.New()
	defer scheduler.Stop()

	configDir, _ := os.UserConfigDir()

	dataDir := filepath.Join(configDir, "hdtd")
	err := os.MkdirAll(dataDir, 0750)
	if err != nil {
		panic(err)
	}

	_, err = os.Stat(filepath.Join(dataDir, diskMappingFileName))
	if os.IsNotExist(err) {
		file, err := os.Create(filepath.Join(dataDir, diskMappingFileName))
		if err != nil {
			panic(err)
		}
		defer file.Close()
	}

	router.GET("/sessions", func(c *gin.Context) {
		type Response struct {
			Sessions []string `json:"sessions"`
		}

		var sessionDirs []string
		sessionDirNames, err := os.ReadDir(dataDir)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for i := range sessionDirNames {
			dirEntry := sessionDirNames[i]
			if !dirEntry.IsDir() {
				continue
			}
			sessionDirs = append(sessionDirs, dirEntry.Name())
		}

		c.JSON(http.StatusOK, Response{Sessions: sessionDirs})
	})

	router.GET("/sessions/:id", func(c *gin.Context) {
		sessionDir := filepath.Join(dataDir, c.Param("id"))

		type Frame struct {
			Id        string `json:"id"`
			Diskstats string `json:"diskstats"`
			Log       string `json:"log"`
			Stdout    string `json:"stdout"`
		}
		type Response struct {
			Frames []Frame `json:"frames"`
		}

		frameDirs, err := os.ReadDir(sessionDir)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var frames []Frame
		for _, e := range frameDirs {
			if !e.IsDir() {
				continue
			}
			var diskStatsBytes, logBytes, stdoutBytes []byte
			diskStatsBytes, err = os.ReadFile(filepath.Join(sessionDir, e.Name(), "diskstats"))
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			logBytes, err = os.ReadFile(filepath.Join(sessionDir, e.Name(), "log"))
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			stdoutBytes, err = os.ReadFile(filepath.Join(sessionDir, e.Name(), "stdout"))
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			frames = append(frames, Frame{
				Id:        e.Name(),
				Diskstats: string(diskStatsBytes),
				Log:       string(logBytes),
				Stdout:    string(stdoutBytes),
			})
		}

		c.JSON(http.StatusOK, Response{Frames: frames})
	})

	router.GET("/status", func(c *gin.Context) {
		taskLen := len(scheduler.Tasks())

		diskMappingFile, err := os.Open(filepath.Join(dataDir, diskMappingFileName))
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer diskMappingFile.Close()

		mapping := make(map[string]string)
		scanner := bufio.NewScanner(diskMappingFile)
		for scanner.Scan() {
			mappingParts := strings.Split(scanner.Text(), ":")
			mapping[mappingParts[0]] = mappingParts[1]
		}

		if err := scanner.Err(); err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		type Response struct {
			Recording   bool              `json:"recording"`
			DiskMapping map[string]string `json:"disk_mapping"`
		}

		rec := false
		if taskLen > 0 {
			rec = true
		}
		c.JSON(http.StatusOK,
			Response{Recording: rec,
				DiskMapping: mapping,
			})
	})

	router.GET("/record", func(c *gin.Context) {
		type Response struct {
			Recording bool `json:"recording"`
		}

		select {
		case record := <-recording:
			c.JSON(http.StatusOK, Response{Recording: record})
		}
	})

	router.POST("/record", func(c *gin.Context) {
		type Request struct {
			Action string `json:"action"`
		}
		var request Request
		err := c.ShouldBind(&request)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if request.Action == "start" {
			recording <- true
			hdidleStdoutLength = 0
			hdidleLogLength = 0
			sessionDir := filepath.Join(dataDir, fmt.Sprintf("%d", time.Now().Unix()))
			_, err = scheduler.Add(&tasks.Task{
				Interval:          5 * time.Second,
				RunSingleInstance: true,
				TaskFunc: func() error {
					return collectStats(dataDir, sessionDir)
				},
			})
			if err != nil {
				panic(err)
			}
			log.Printf("Starting recording...")
		}
		if request.Action == "stop" {
			recording <- false
			scheduler.Stop()
			log.Printf("Stopping recording...")
		}

		c.Status(http.StatusOK)
	})

	_ = os.Remove(socketFile)
	listener, err := net.Listen("unix", socketFile)
	if err != nil {
		panic(err)
	}
	err = os.Chmod(socketFile, 0777)
	if err != nil {
		panic(err)
	}
	err = http.Serve(listener, router)
	if err != nil {
		panic(err)
	}
}

func collectStats(dataDir, sessionDir string) error {
	frameDir := filepath.Join(sessionDir, fmt.Sprintf("%d", time.Now().Unix()))
	err := os.MkdirAll(frameDir, 0750)
	if err != nil {
		return err
	}

	err = collectDiskstats(frameDir)
	if err != nil {
		return err
	}
	err = collectHdIdleLog(dataDir, frameDir)
	if err != nil {
		return err
	}
	err = collectHdIdleStdout(dataDir, frameDir)
	if err != nil {
		return err
	}

	return nil
}

func collectDiskstats(frameDir string) error {
	bytesRead, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(frameDir, "diskstats"), bytesRead, 0644)
}

func collectHdIdleLog(dataDir, frameDir string) error {
	return collectLog(dataDir, hdidleLogFile, filepath.Join(frameDir, "log"), &hdidleLogLength)
}
func collectHdIdleStdout(dataDir, frameDir string) error {
	return collectLog(dataDir, hdidleStdoutFile, filepath.Join(frameDir, "stdout"), &hdidleStdoutLength)
}

func collectLog(dataDir, originLogPath, destLogPath string, logLen *int) error {
	file, err := os.Open(originLogPath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	if *logLen == 0 {
		lineCount := 0
		for scanner.Scan() {
			lineCount++
		}
		*logLen = lineCount
		return os.WriteFile(destLogPath, []byte{}, 0644)
	}

	var hdLog = ""
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > *logLen {
			line := scanner.Text()
			disk := getDisk(line)
			if err = handleDiskMapping(dataDir, disk); err != nil {
				return err
			}

			hdLog += line + "\n"
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return os.WriteFile(destLogPath, []byte(hdLog), 0644)
}

func handleDiskMapping(dataDir, disk string) error {
	if !strings.HasPrefix(disk, "/") {
		return nil
	}

	diskMappingFile := filepath.Join(dataDir, diskMappingFileName)
	if data, err := os.ReadFile(diskMappingFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, disk) {
				return nil
			}
		}
	}

	s, err := os.Readlink(disk)
	if err != nil {
		return err
	}

	device := filepath.Base(s)

	f, err := os.OpenFile(diskMappingFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	mappingLine := fmt.Sprintf("%s:%s", disk, device)
	if _, err = f.WriteString(mappingLine + "\n"); err != nil {
		return err
	}
	return nil
}

func getDisk(line string) string {
	dataEntries := strings.Split(line, ",")
	for i := range dataEntries {
		dataPair := strings.Split(dataEntries[i], ":")
		if dataPair[0] == "disk" {
			return strings.TrimSpace(dataPair[1])
		}
	}
	return ""
}
