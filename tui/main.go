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
	socketFile         = "/tmp/demo.sock"
)

var (
	recording = false
	diskstats []string
	logsView  *tview.TextView
)

func main() {
	client, err := openClient()
	if err != nil {
		panic(err)
	}

	statsView := tview.NewTextView()
	statsView.SetText("generateContent(displayContent)").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(1, 1, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("/proc/diskstats").
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)
	hdIdleLogView := tview.NewTextView()
	hdIdleLogView.SetText("").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(1, 1, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("hd-idle log").
		SetBorderColor(textAndBorderColor).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)
	logsView = tview.NewTextView()
	logsView.SetText("Test").
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

	app := tview.NewApplication()

	right := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statsView, 0, 2, false).
		AddItem(hdIdleLogView, 0, 1, false)

	top := tview.NewFlex().
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(sessionsList, 18, 1, true).
			AddItem(right, 0, 1, false),
			0, 1, true)
	bottom := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(logsView, 0, 1, true)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(top, 0, 1, true).
		AddItem(bottom, 3, 1, false)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			app.Stop()
		case 'r':
			if recording {
				logsView.SetText("Stop recording...")
				_, err := sendDaemon(client, "/record", "{\"action\":\"stop\"}")
				if err != nil {
					panic(err)
				}
				//statsView.SetText(response)
				go func() {
					refreshAvailableSessions(client, sessionsList, statsView, app, flex)
				}()
			} else {
				logsView.SetText("Start recording...")
				_, err := sendDaemon(client, "/record", "{\"action\":\"start\"}")
				if err != nil {
					panic(err)
				}
				//statsView.SetText(response)
			}
			recording = !recording
		}
		return event
	})

	go func() {
		refreshAvailableSessions(client, sessionsList, statsView, app, flex)
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}

}

func refreshAvailableSessions(client http.Client, sessionsList *tview.List, statsView *tview.TextView, app *tview.Application, flex *tview.Flex) {
	sessions, err := requestSessionsFromDaemon(client)
	if err != nil {
		panic(err)
	}
	sessionsList.Clear()
	logsView.SetText("Loading sessions...")
	if len(sessions) == 0 {
		logsView.SetText("No sessions available.")
		statsView.Clear()
		return
	}

	diskstats, err = requestSessionFromDaemon(client, sessions[sessionsList.GetCurrentItem()])
	if err != nil {
		logsView.SetText("Error loading session. " + err.Error())
		statsView.Clear()
		return
	}
	logsView.SetText(fmt.Sprintf("Loading session %s...", sessions[sessionsList.GetCurrentItem()]))
	statsView.SetText(diskstats[0])

	for i := range sessions {
		runes := []rune(strconv.Itoa(i + 1))
		sessionsList.AddItem(sessions[i], "", runes[0], func() {
			logsView.SetText(fmt.Sprintf("Loading session %s...", sessions[i]))
			diskstats, err = requestSessionFromDaemon(client, sessions[sessionsList.GetCurrentItem()])
			if err != nil {
				logsView.SetText("Error loading session. " + err.Error())
				statsView.Clear()
				return
			}
			statsView.SetText(diskstats[0])

			app.SetFocus(flex)
		})
	}
	app.Draw()
}

func requestSessionsFromDaemon(c http.Client) ([]string, error) {
	client, err := openClient()
	if err != nil {
		panic(err)
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

func requestSessionFromDaemon(c http.Client, id string) ([]string, error) {
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
		Diskstats []string `json:"diskstats"`
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("unable to parse response body. " + err.Error())
	}
	return response.Diskstats, nil
}

func sendDaemon(c http.Client, endpoint, message string) (string, error) {

	// Send a GET request to the server
	client, err := openClient()
	if err != nil {
		panic(err)
	}
	resp, err := client.Post("http://unix"+endpoint, "application/json", strings.NewReader(message))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func openClient() (http.Client, error) {
	conn, err := net.Dial("unix", socketFile)
	if err != nil {
		panic(err)
	}

	// Create an HTTP client with the Unix socket connection
	c := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return conn, nil
			},
		},
	}
	return c, err
}
