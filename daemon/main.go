package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madflojo/tasks"
)

const (
	socketFile    = "/tmp/demo.sock"
	hdidleLogFile = "/var/log/hd-idle.log"
)

var (
	hdidleLogLengh = 0
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

	router.GET("/sessions", func(c *gin.Context) {
		type Response struct {
			Sessions []string `json:"sessions"`
		}

		var sessionDirs []string
		sessionDirNames, err := os.ReadDir(dataDir)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		for i := range sessionDirNames {
			sessionDirs = append(sessionDirs, sessionDirNames[i].Name())
		}

		c.JSON(http.StatusOK, Response{Sessions: sessionDirs})
	})

	router.GET("/sessions/:id", func(c *gin.Context) {
		sessionDir := filepath.Join(dataDir, c.Param("id"))

		type Frame struct {
			Diskstats string `json:"diskstats"`
			Log       string `json:"log"`
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
			diskStatsBytes, err := os.ReadFile(filepath.Join(sessionDir, e.Name(), "diskstats"))
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			logBytes, err := os.ReadFile(filepath.Join(sessionDir, e.Name(), "log"))
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			frames = append(frames, Frame{
				Diskstats: string(diskStatsBytes),
				Log:       string(logBytes),
			})
		}

		c.JSON(http.StatusOK, Response{Frames: frames})
	})

	router.GET("/status", func(c *gin.Context) {
		taskLen := len(scheduler.Tasks())
		type Response struct {
			Recording bool `json:"recording"`
		}

		if taskLen == 0 {
			c.JSON(http.StatusOK, Response{Recording: false})
		} else {
			c.JSON(http.StatusOK, Response{Recording: true})
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
			hdidleLogLengh = 0
			sessionDir := filepath.Join(dataDir, fmt.Sprintf("%d", time.Now().Unix()))
			_, err = scheduler.Add(&tasks.Task{
				Interval: 5 * time.Second,
				TaskFunc: func() error {
					return collectStats(sessionDir)
				},
			})
			if err != nil {
				panic(err)
			}
			log.Printf("Starting recording...")
		}
		if request.Action == "stop" {
			scheduler.Stop()
			log.Printf("Stopping recording...")
		}

		c.Status(http.StatusOK)
	})

	err = os.Remove(socketFile)
	if err != nil {
		panic(err)
	}
	listener, err := net.Listen("unix", socketFile)
	if err != nil {
		panic(err)
	}

	err = http.Serve(listener, router)
	if err != nil {
		panic(err)
	}

}

func collectStats(sessionDir string) error {
	fmt.Println("Working...")
	frameDir := filepath.Join(sessionDir, fmt.Sprintf("%d", time.Now().Unix()))
	err := os.MkdirAll(frameDir, 0750)
	if err != nil {
		return err
	}

	err = collectDiskstats(frameDir)
	if err != nil {
		return err
	}
	err = collectHdIdleLog(frameDir)
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

func collectHdIdleLog(frameDir string) error {
	file, err := os.Open(hdidleLogFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	if hdidleLogLengh == 0 {
		lineCount := 0
		for scanner.Scan() {
			lineCount++
		}
		hdidleLogLengh = lineCount
		return os.WriteFile(filepath.Join(frameDir, "log"), []byte{}, 0644)
	}

	var l = ""
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > hdidleLogLengh {
			l += scanner.Text() + "\n"
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(frameDir, "log"), []byte(l), 0644)
}
