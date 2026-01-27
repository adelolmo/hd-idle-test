package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const (
	socketFile     = "/tmp/spd.sock"
	plugOneBaseUrl = "http://192.168.178.21"
	plugTwoBaseUrl = "http://192.168.178.22"
)

var mapping = map[string]string{
	"sda": plugOneBaseUrl,
	"sdb": plugTwoBaseUrl,
}

func main() {
	router := gin.Default()

	router.GET("/devices/:id", func(c *gin.Context) {
		deviceId := c.Param("id")
		if mapping[deviceId] == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		fmt.Println(deviceId, mapping[deviceId])

		status, err := smartPlugStatus(mapping[deviceId])
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, status)
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

type DeviceStatusResponse struct {
	Up bool `json:"up"`
}

func smartPlugStatus(baseUrl string) (DeviceStatusResponse, error) {
	type Response struct {
		Apower float64 `json:"apower"`
	}

	resp, err := http.Get(baseUrl + "/rpc/Switch.GetStatus?id=0")
	if err != nil {
		return DeviceStatusResponse{}, err
	}
	var response Response
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return DeviceStatusResponse{}, err
	}

	return DeviceStatusResponse{Up: response.Apower > 0}, nil
}
