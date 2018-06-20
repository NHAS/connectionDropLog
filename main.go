package main

import (
	"bufio"
	"errors"
	"log"
	"strings"
	"sync"

	"os/exec"

	"github.com/gizak/termui"
)

type List struct {
	lock sync.RWMutex
	list []string
}

func (l *List) Push(e string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.list = append(l.list, e)
}

func (l *List) Get(index int) (string, error) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	if len(l.list) == 0 {
		return "", errors.New("List empty")
	}

	if index < 0 || index > len(l.list) {
		return "", errors.New("Index out of range")
	}

	return l.list[len(l.list)-1-index], nil
}

func (l *List) GetRange(end int) []string {
	l.lock.RLock()
	defer l.lock.RUnlock()

	if end <= 0 {
		return []string{}
	}

	if len(l.list) == 0 {
		return []string{}
	}

	if end > len(l.list) {
		end = len(l.list)
	}

	return l.list[len(l.list)-end : len(l.list)-1]
}

func (l *List) Size() int {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return len(l.list)
}

func main() {
	cmd := exec.Command("journalctl", "-exf")
	var external, internal List

	go readLog(cmd, &external, &internal)

	err := termui.Init()
	if err != nil {
		panic(err)
	}
	defer termui.Close()

	internalLogs := internal.GetRange(termui.TermHeight() - 2)
	internalDisplay := termui.NewList()
	internalDisplay.Items = internalLogs
	internalDisplay.ItemFgColor = termui.ColorRed
	internalDisplay.BorderLabel = "Internal dropped"
	internalDisplay.Height = termui.TermHeight()
	internalDisplay.Width = termui.TermWidth() / 2
	internalDisplay.Y = 0

	externalLogs := external.GetRange(termui.TermHeight() - 2)
	externalDisplay := termui.NewList()
	externalDisplay.Items = externalLogs
	externalDisplay.ItemFgColor = termui.ColorYellow
	externalDisplay.BorderLabel = "External dropped"
	externalDisplay.Height = termui.TermHeight()
	externalDisplay.Width = termui.TermWidth() / 2
	externalDisplay.Y = 0
	externalDisplay.X = termui.TermWidth() / 2

	termui.Handle("/sys/kbd/q", func(termui.Event) {
		cmd.Process.Kill()
		termui.StopLoop()

	})

	termui.Handle("/timer/1s", func(e termui.Event) {
		externalDisplay.Items = external.GetRange(termui.TermHeight() - 2)
		internalDisplay.Items = internal.GetRange(termui.TermHeight() - 2)

		termui.Render(internalDisplay, externalDisplay)
	})

	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		internalDisplay.Height = e.Data.(termui.EvtWnd).Height
		internalDisplay.Width = e.Data.(termui.EvtWnd).Width / 2

		externalDisplay.Height = e.Data.(termui.EvtWnd).Height
		externalDisplay.Width = e.Data.(termui.EvtWnd).Width / 2
		externalDisplay.X = termui.TermWidth() / 2
	})

	termui.Loop()
}

func readLog(cmd *exec.Cmd, external, internal *List) {

	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {

			messageParts := strings.Split(scanner.Text(), " ")
			if len(messageParts) < 5 {
				log.Println("Warning malformed message. Size: ", len(messageParts))
				continue
			}

			if messageParts[5] == "EXTERNAL_DROPPED:" || messageParts[5] == "INTERNAL_DROPPED:" {

				importantInformation := strings.Join(messageParts[0:3], " ")
				importantInformation += " " + messageParts[9]

				for _, e := range messageParts[9:] {
					if strings.Contains(e, "DPT=") {
						importantInformation += " " + e
						break
					} else if strings.Contains(e, "PROTO=") {
						importantInformation += " " + e
					}

				}

				switch messageParts[5] {
				case "EXTERNAL_DROPPED:":
					{
						external.Push(importantInformation)
					}
					break

				case "INTERNAL_DROPPED:":
					{
						internal.Push(importantInformation)
					}
					break

				default:
					{
						break
					}

				}
			}
		}
	}()
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}

}
