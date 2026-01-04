package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	//"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	textAndBorderColor = tcell.ColorDarkGrey
	backgroundColor    = tcell.ColorDefault
	colorBlack         = tcell.Color16
	socketFile         = "/tmp/hdtd.sock"

	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[37m"
	White   = "\033[97m"
)

type Frame struct {
	Id        string `json:"id"`
	Diskstats string `json:"diskstats"`
	Log       string `json:"log"`
	Stdout    string `json:"stdout"`
}

func (f Frame) timestamp() string {
	return formatFromUnixTime(f.Id)
}

func (f Frame) adaptedDiskstats() string {
	reader := strings.NewReader(f.Diskstats)
	scanner := bufio.NewScanner(reader)
	lineIndex := 0
	statsLines := strings.Split(f.Diskstats, "\n")
	for scanner.Scan() {
		text := scanner.Text()
		cols := strings.Fields(text)
		if len(cols) < 10 {
			continue
		}

		statsLines[lineIndex] = fmt.Sprintf("%4s%8s", cols[0], cols[1])
		for i := 2; i < len(cols); i++ {
			value := cols[i]
			switch i {
			case 5, 9:
				value = fmt.Sprintf("[white]%s[#999999]", value)
			}
			statsLines[lineIndex] += fmt.Sprintf(" %s", value)
		}

		lineIndex++
	}
	return strings.Join(statsLines, "\n")
}

func (f Frame) adaptedLog() string {
	for disk, path := range diskMapping {
		redacted := fmt.Sprintf("%s [%s]", path, disk)
		f.Log = strings.ReplaceAll(f.Log, disk, redacted)
	}
	return f.Log
}

var (
	recording        = false
	diskMapping      map[string]string
	app              *tview.Application
	paginationView   *tview.TextView
	framesView       *tview.TextView
	logsView         *tview.TextView
	recordingView    *tview.TextView
	right            *tview.Flex
	statsView        *tview.TextView
	hdIdleLogView    *tview.TextView
	hdIdleStdoutView *tview.TextView
	frames           []Frame
	frameIndex       int
	line             int
)

func main() {
	dim := tcell.StyleDefault.Dim(true)

	paginationView = tview.NewTextView()
	paginationView.SetText("0 of 0").
		SetTextAlign(tview.AlignCenter).
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetBorder(true).
		SetBorderStyle(dim).
		SetBackgroundColor(backgroundColor)
	framesView = tview.NewTextView()
	framesView.SetText("").
		SetTextAlign(tview.AlignCenter).
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetBorderStyle(dim).
		SetBackgroundColor(backgroundColor)
	statsView = tview.NewTextView()
	statsView.SetText("").
		SetDynamicColors(true).
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("/proc/diskstats").
		SetBorderStyle(dim).
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
		SetBorderStyle(dim).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor)
	hdIdleStdoutView = tview.NewTextView()
	hdIdleStdoutView.SetText("").
		SetTextColor(textAndBorderColor).
		SetWordWrap(true).
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetTitle("hd-idle stdout").
		SetBorderStyle(dim).
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
		SetBackgroundColor(backgroundColor).
		SetFocusFunc(func() {

		},
		)

	app = tview.NewApplication()

	paginationColumn := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(paginationView, 0, 1, false).
		AddItem(framesView, 0, 6, false)
	logs := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(hdIdleStdoutView, 0, 1, false).
		AddItem(hdIdleLogView, 0, 4, false)

	right = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(paginationColumn, 3, 1, false).
		AddItem(statsView, 0, 3, false).
		AddItem(logs, 0, 1, false)
	right.SetBorder(true).SetBorderStyle(tcell.StyleDefault)
	right.SetFocusFunc(func() {
		//logsView.SetText("Focus on right flex")
	})

	topRow := tview.NewFlex().
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(sessionsList, 23, 1, true).
			AddItem(right, 0, 1, false),
			0, 1, true)
	bottomRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(recordingView, 5, 1, false).
		AddItem(logsView, 0, 1, false)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topRow, 0, 1, true).
		AddItem(bottomRow, 3, 1, false)

	right.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			app.SetFocus(sessionsList)
		case tcell.KeyRight:
			switch event.Modifiers() {
			case tcell.ModShift:
				frameIndex += 10
			case tcell.ModCtrl:
				frameIndex += 100
			default:
				frameIndex++
			}
			if frameIndex >= len(frames) {
				frameIndex = len(frames) - 1
			}
			paginationView.SetText(fmt.Sprintf("%d of %d", frameIndex+1, len(frames)))
			framesView.SetText(frames[frameIndex].timestamp())
			statsView.SetText(frames[frameIndex].adaptedDiskstats())
			hdIdleStdoutView.SetText(frames[frameIndex].Stdout)
			hdIdleLogView.SetText(frames[frameIndex].adaptedLog())
		case tcell.KeyLeft:
			switch event.Modifiers() {
			case tcell.ModShift:
				frameIndex -= 10
			case tcell.ModCtrl:
				frameIndex -= 100
			default:
				frameIndex--
			}
			if frameIndex < 0 {
				frameIndex = 0
			}
			paginationView.SetText(fmt.Sprintf("%d of %d", frameIndex+1, len(frames)))
			framesView.SetText(frames[frameIndex].timestamp())
			statsView.SetText(frames[frameIndex].adaptedDiskstats())
			hdIdleStdoutView.SetText(frames[frameIndex].Stdout)
			hdIdleLogView.SetText(frames[frameIndex].adaptedLog())

		case tcell.KeyDown:
			line++
			_, _, _, height := statsView.GetRect()
			if line > statsView.GetWrappedLineCount()-height+1 {
				line = statsView.GetWrappedLineCount() - height + 1
			}
			statsView.ScrollTo(line, 0)
		case tcell.KeyUp:
			line--
			if line < 0 {
				line = 0
			}
			statsView.ScrollTo(line, 0)
		}
		return event
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlR:
			if recording {
				logsView.SetText("Stop recording...")
				_, err := sendDaemon("/record", "{\"action\":\"stop\"}")
				if err != nil {
					panic(err)
				}
				go func() {
					refreshAvailableSessions(sessionsList, statsView)
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
		default:
			switch event.Rune() {
			case 'q':
				app.Stop()
			case 'r':
				go func() {
					refreshAvailableSessions(sessionsList, statsView)
				}()
				go func() {
					updateStatus()
				}()
			}
		}

		return event
	})

	go func() {
		refreshAvailableSessions(sessionsList, statsView)
	}()
	go func() {
		updateStatus()
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

func refreshAvailableSessions(sessionsList *tview.List, statsView *tview.TextView) {
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
	paginationView.SetText(fmt.Sprintf("1 of %d", len(sessions)))
	framesView.SetText(frames[0].timestamp())
	logsView.SetText(fmt.Sprintf("Loading session %s...", sessions[sessionsList.GetCurrentItem()]))
	statsView.SetText(frames[0].adaptedDiskstats())
	hdIdleStdoutView.SetText(frames[0].Stdout)
	hdIdleLogView.SetText(frames[0].Log)

	for i := range sessions {
		runes := []rune(strconv.Itoa(i + 1))
		sessionsList.AddItem(sessions[i], formatFromUnixTime(sessions[i]), runes[0], func() {
			logsView.SetText(fmt.Sprintf("Loading session %s...", sessions[i]))
			frames, err = requestSessionFromDaemon(sessions[sessionsList.GetCurrentItem()])
			if err != nil {
				logsView.SetText("Error loading session. " + err.Error())
				statsView.Clear()
				return
			}
			frameIndex = 0
			paginationView.SetText(fmt.Sprintf("1 of %d", len(frames)))
			framesView.SetText(frames[frameIndex].timestamp())
			statsView.SetText(frames[frameIndex].adaptedDiskstats())
			hdIdleStdoutView.SetText(frames[frameIndex].Stdout)
			hdIdleLogView.SetText(frames[frameIndex].Log)
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
		logsView.SetText("Error getting status. " + err.Error())
		return
	}

	type Response struct {
		Recording   bool              `json:"recording"`
		DiskMapping map[string]string `json:"disk_mapping"`
	}
	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logsView.SetText("Error getting status. " + err.Error())
		return
	}

	diskMapping = response.DiskMapping
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

func formatFromUnixTime(date string) string {
	unixTimestamp, _ := strconv.ParseInt(date, 10, 64)
	return time.Unix(unixTimestamp, 0).Format("02 Jan 15:04:05")
}
