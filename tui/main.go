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
	listHelp = "[white:gray]Navigation [↑↓][-:-] " +
		"[white:gray]Select [⮠][-:-] " +
		"[white:gray]Reload [r[][-:-] " +
		"[white:gray]Rec start/stop [^r][-:-] " +
		"[white:gray]Quit [q[][-:-]"
	frameHelp = "[white:gray]Next [→][⇧→][^→][-:-] " +
		"[white:gray]Previous [←][⇧←][^←][-:-] " +
		"[white:gray]Scroll [↑↓][-:-] " +
		"[white:gray]Back [esc[][-:-] " +
		"[white:gray]Reload [r[][-:-] " +
		"[white:gray]Rec start/stop [^r][-:-] " +
		"[white:gray]Quit [q[][-:-]"

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
	helpView         *tview.TextView

	frames     []Frame
	frameIndex int

	statsViewLine        int
	hdIdleLogViewLine    int
	hdIdleStdoutViewLine int
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
	statsView = newDataTextView("/proc/diskstats")
	hdIdleLogView = newDataTextView("hd-idle log")
	hdIdleStdoutView = newDataTextView("hd-idle stdout")
	logsView = newDataTextView("")
	recordingView = newDataTextView("")

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

	helpView = tview.NewTextView().
		SetDynamicColors(true)
	helpView.SetBackgroundColor(backgroundColor)
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
			AddItem(sessionsList, 19, 1, true).
			AddItem(right, 0, 1, false),
			0, 1, true)
	bottomRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(recordingView, 5, 1, false).
		AddItem(logsView, 0, 1, false)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topRow, 0, 1, true).
		AddItem(bottomRow, 3, 1, false).
		AddItem(helpView, 1, 1, false)

	right.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			helpView.SetText(listHelp)
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
			printRightPanel(frames[frameIndex])
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
			printRightPanel(frames[frameIndex])
		case tcell.KeyDown:
			scrollDown(&statsViewLine, statsView)
			scrollDown(&hdIdleStdoutViewLine, hdIdleStdoutView)
			scrollDown(&hdIdleLogViewLine, hdIdleLogView)
		case tcell.KeyUp:
			scrollUp(&statsViewLine, statsView)
			scrollUp(&hdIdleStdoutViewLine, hdIdleStdoutView)
			scrollUp(&hdIdleLogViewLine, hdIdleLogView)
		}
		return event
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlR:
			if recording {
				logsView.SetText("Stop recording...")
				_, err := sendDaemon("/record", `{"action":"stop"}`)
				if err != nil {
					panic(err)
				}
				go func() {
					refreshAvailableSessions(sessionsList)
				}()
				recordingView.Clear()
			} else {
				logsView.SetText("Start recording...")
				_, err := sendDaemon("/record", `{"action":"start"}`)
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
				go refreshAvailableSessions(sessionsList)
			}
		}

		return event
	})

	go refreshAvailableSessions(sessionsList)
	go func() {
		updateRecording()
		updateStatus()
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

func newDataTextView(title string) *tview.TextView {
	view := tview.NewTextView().
		SetText("").
		SetDynamicColors(true).
		SetTextColor(textAndBorderColor).
		SetWordWrap(true)
	view.
		SetBackgroundColor(backgroundColor).
		SetBorderPadding(0, 0, 1, 1).
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true).
		SetBorderStyle(tcell.StyleDefault.Dim(true)).
		SetTitleColor(textAndBorderColor).
		SetBackgroundColor(backgroundColor).
		SetTitle(title)
	return view
}

func scrollDown(line *int, view *tview.TextView) {
	*line++
	_, _, _, height := view.GetRect()
	if *line > view.GetWrappedLineCount()-height+1 {
		*line = view.GetWrappedLineCount() - height + 1
	}
	view.ScrollTo(*line, 0)
}

func scrollUp(line *int, view *tview.TextView) {
	*line--
	if *line < 0 {
		*line = 0
	}
	view.ScrollTo(*line, 0)
}

func refreshAvailableSessions(sessionsList *tview.List) {
	sessions, err := requestSessionsFromDaemon()
	if err != nil {
		logsView.SetText("Unable to load sessions. " + err.Error())
		return
	}
	sessionsList.Clear()
	helpView.SetText(listHelp)
	logsView.SetText("Loading sessions...")
	if len(sessions) == 0 {
		logsView.SetText("No sessions available.")
		clearRightPanel()
		return
	}

	app.QueueUpdateDraw(func() {
		for i := range sessions {
			sessionsList.AddItem(sessions[i], formatFromUnixTime(sessions[i]), 0, nil)
		}
	})

	logsView.Clear()

	onSelect := func(i int, mainText, secondaryText string, shortcut rune) {

		go func() {
			helpView.SetText(frameHelp)
			logsView.SetText(fmt.Sprintf("Loading session %s...", mainText))
			clearRightPanel()
			app.SetFocus(right)
			app.Draw()
		}()

		go func() {
			frames, err = requestSessionFromDaemon(sessions[i])
			if err != nil {
				clearRightPanel()
				logsView.SetText("Error loading session. " + err.Error())
				return
			}
			frameIndex = 0
			paginationView.SetText(fmt.Sprintf("1 of %d", len(frames)))
			printRightPanel(frames[0])
			logsView.SetText(fmt.Sprintf("Session %s", mainText))
			app.Draw()
		}()

	}
	sessionsList.SetSelectedFunc(onSelect)
}

func printRightPanel(frame Frame) {
	framesView.SetText(frame.timestamp())
	statsView.SetText(frame.adaptedDiskstats())
	hdIdleStdoutView.SetText(frame.Stdout)
	hdIdleLogView.SetText(frame.adaptedLog())
}

func clearRightPanel() {
	paginationView.SetText("0 of 0")
	framesView.Clear()
	statsView.Clear()
	hdIdleStdoutView.Clear()
	hdIdleLogView.Clear()
}

func updateStatus() {
	client, err := openClient()
	if err != nil {
		logsView.SetText("Error getting status. " + err.Error())
		return
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
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
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

func updateRecording() {
	for {
		client, err := openClient()
		if err != nil {
			logsView.SetText("Error getting recording. " + err.Error())
			continue
		}

		resp, err := client.Get("http://unix/record")
		if err != nil {
			logsView.SetText("Error getting record. " + err.Error())
			return
		}

		type Response struct {
			Recording bool `json:"recording"`
		}
		var response Response
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			logsView.SetText("Error getting status. " + err.Error())
			return
		}

		recordingView.Clear()
		if response.Recording {
			recording = true
			recordingView.SetText("R")
		}
		app.Draw()
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

	if resp.StatusCode == 500 {
		type Response struct {
			Error string `json:"error"`
		}
		var response Response
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("unable to parse response body. " + err.Error())
		}
		return nil, fmt.Errorf("server error: %s", response.Error)
	}

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
