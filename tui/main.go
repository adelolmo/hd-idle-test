package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	//"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	textAndBorderColor = tcell.ColorDarkGrey
	backgroundColor    = tcell.ColorDefault
	socketFile         = "/tmp/hdtd.sock"
)

type Frame struct {
	Diskstats string `json:"diskstats"`
	Log       string `json:"log"`
}

var (
	recording     = false
	app           *tview.Application
	logsView      *tview.TextView
	recordingView *tview.TextView
	right         *tview.Flex
	statsView     *tview.TextView
	hdIdleLogView *tview.TextView
	frames        []Frame
	frameIndex    int
)

func main() {
	statsView = tview.NewTextView()
	statsView.SetText("").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("/proc/diskstats").
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)
	hdIdleLogView = tview.NewTextView()
	hdIdleLogView.SetText("").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("hd-idle log").
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)
	logsView = tview.NewTextView()
	logsView.SetText("").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)
	recordingView = tview.NewTextView()
	recordingView.SetText("").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)

	sessionsList := tview.NewList()
	sessionsList.SetBackgroundColor(backgroundColor).
		SetBorderPadding(1, 1, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("Sessions").
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)

	app = tview.NewApplication()

	right = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statsView, 0, 2, false).
		AddItem(hdIdleLogView, 0, 1, false)
	right.SetFocusFunc(func() {
		//logsView.SetText("Focus on right flex")
	})

	topRow := tview.NewFlex().
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(sessionsList, 18, 1, true).
			AddItem(right, 0, 1, false),
			0, 1, true)
	bottomRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(recordingView, 5, 1, false).
		AddItem(logsView, 0, 1, false)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topRow, 0, 1, true).
		AddItem(bottomRow, 3, 1, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			app.SetFocus(sessionsList)
		case tcell.KeyRight:
			frameIndex++
			if frameIndex >= len(frames) {
				frameIndex = len(frames) - 1
			}
			statsView.SetText(frames[frameIndex].Diskstats)
			hdIdleLogView.SetText(frames[frameIndex].Log)
		case tcell.KeyLeft:
			frameIndex--
			if frameIndex < 0 {
				frameIndex = 0
			}
			statsView.SetText(frames[frameIndex].Diskstats)
			hdIdleLogView.SetText(frames[frameIndex].Log)
		}
		return event
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			app.Stop()
		case 'r':
			if recording {
				logsView.SetText("Stop recording...")
				_, err := sendDaemon("/record", "{\"action\":\"stop\"}")
				if err != nil {
					panic(err)
				}
				go func() {
					refreshAvailableSessions(sessionsList, statsView, flex)
				}()
				recordingView.Clear()
			} else {
				logsView.SetText("Start recording...")
				_, err := sendDaemon("/record", "{\"action\":\"start\"}")
				if err != nil {
					panic(err)
				}
				recordingView.SetText("R")
			}
			recording = !recording
		}
		return event
	})

	go func() {
		refreshAvailableSessions(sessionsList, statsView, flex)
	}()
	go func() {
		updateStatus()
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

func refreshAvailableSessions(sessionsList *tview.List, statsView *tview.TextView, flex *tview.Flex) {
	sessions, err := requestSessionsFromDaemon()
	if err != nil {
		logsView.SetText("Unable to load sessions. " + err.Error())
		return
	}
	sessionsList.Clear()
	logsView.SetText("Loading sessions...")
	if len(sessions) == 0 {
		logsView.SetText("No sessions available.")
		statsView.Clear()
		return
	}

	frames, err = requestSessionFromDaemon(sessions[sessionsList.GetCurrentItem()])
	if err != nil {
		logsView.SetText("Error loading session. " + err.Error())
		statsView.Clear()
		return
	}
	logsView.SetText(fmt.Sprintf("Loading session %s...", sessions[sessionsList.GetCurrentItem()]))
	statsView.SetText(frames[0].Diskstats)
	logsView.SetText(frames[0].Log)

	for i := range sessions {
		runes := []rune(strconv.Itoa(i + 1))
		sessionsList.AddItem(sessions[i], "", runes[0], func() {
			logsView.SetText(fmt.Sprintf("Loading session %s...", sessions[i]))
			frames, err = requestSessionFromDaemon(sessions[sessionsList.GetCurrentItem()])
			if err != nil {
				logsView.SetText("Error loading session. " + err.Error())
				statsView.Clear()
				return
			}
			statsView.SetText(frames[0].Diskstats)
			logsView.SetText(frames[0].Log)
			logsView.SetText(fmt.Sprintf("Session %s", sessions[i]))

			app.SetFocus(right)
		})
	}
	app.Draw()
}

func updateStatus() {
	client, err := openClient()
	if err != nil {
		logsView.SetText("Error getting status. " + err.Error())
	}

	resp, err := client.Get("http://unix/status")
	if err != nil {
		//return nil, err
		logsView.SetText("Error getting status. " + err.Error())
		return
	}

	type Response struct {
		Recording bool `json:"recording"`
	}
	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logsView.SetText("Error getting status. " + err.Error())
		return
	}

	recordingView.Clear()
	if response.Recording {
		recording = true
		recordingView.SetText("R")
	}
}

func requestSessionsFromDaemon() ([]string, error) {
	client, err := openClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Get("http://unix/sessions")
	if err != nil {
		return nil, err
	}
	type Response struct {
		Sessions []string `json:"sessions"`
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("unable to parse response body. " + err.Error())
	}
	return response.Sessions, nil
}

func requestSessionFromDaemon(id string) ([]Frame, error) {
	client, err := openClient()
	if err != nil {
		panic(err)
	}
	resp, err := client.Get("http://unix/sessions/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type Response struct {
		Frames []Frame `json:"frames"`
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("unable to parse response body. " + err.Error())
	}
	return response.Frames, nil
}

func sendDaemon(endpoint, message string) (string, error) {
	client, err := openClient()
	if err != nil {
		panic(err)
	}
	resp, err := client.Post("http://unix"+endpoint, "application/json", strings.NewReader(message))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func openClient() (http.Client, error) {
	conn, err := net.Dial("unix", socketFile)
	if err != nil {
		return http.Client{}, err
	}

	c := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return conn, nil
			},
		},
	}
	return c, err
}
