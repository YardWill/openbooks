package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/evan-buss/openbooks-cli/dcc"
	"github.com/evan-buss/openbooks-cli/irc"
)

// Establishes connection with IRC server, displays menu, spawns goroutines
func main() {
	fmt.Println("=======================================")
	fmt.Println("          Welcome to OpenBooks         ")
	fmt.Println("=======================================")
	currentUser, _ := user.Current()
	irc := irc.New(currentUser.Name, currentUser.Name)
	irc.Connect("irc.irchighway.net")

	time.Sleep(time.Second * 2)

	irc.JoinChannel("ebooks")

	statusC := make(chan bool)
	stateC := make(chan bool)

	go readDaemon(irc, statusC, stateC)

	fmt.Println("Connection established...")

	for i := 30; i > 0; i-- {
		fmt.Print("\rServer rules mandate a " + strconv.Itoa(i) + " second wait period   ")
		time.Sleep(time.Second)
	}

	fmt.Print("\r")

	reader := bufio.NewReader(os.Stdin)
	isDownloading := false // Hide menu when download in progress

	for {
		select {
		case <-statusC:
			isDownloading = false
		default:
		}

		if !isDownloading {
			fmt.Print("\ns)search\ng)et book\nd)one\n~> ")

			input, _ := reader.ReadString('\n')

			switch input {
			case "s\n":
				fmt.Print("@search ")
				message, _ := reader.ReadString('\n')
				irc.SendMessage("@search " + message)
				stateC <- false
				isDownloading = true
			case "g\n":
				fmt.Print("Download String: ")
				message, _ := reader.ReadString('\n')
				irc.SendMessage(message)
				stateC <- true
				isDownloading = true
			case "d\n":
				fmt.Println("disonnecting")
				irc.Disconnect()
				os.Exit(0)
			default:
				fmt.Println("Invalid Selection")
			}
		}
		time.Sleep(time.Millisecond * 100)
	}
}

// Designed to be launched as a goroutine. Listens for specific messages and
// responds accordingly
func readDaemon(irc *irc.Conn, statusC chan<- bool, stateC <-chan bool) {
	f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.WriteString("\n==================== NEW LOG ======================\n")

	if err != nil {
		panic(err)
	}
	defer f.Close()

	isBook := false

	doneChan := make(chan bool)

	for {
		text := irc.GetMessage()
		// fmt.Println(text)
		f.WriteString(text)

		select {
		// Get state of app from menu. isBook = download book; !isBook = search
		case isBook = <-stateC:
		default:
		}

		if strings.Contains(text, "DCC SEND") {
			go dcc.NewDownload(text, isBook, doneChan)
		} else if strings.Contains(text, "NOTICE") {
			// n := strings.LastIndex(text, "evan_bot") + len("evan_bot") + 1
			// fmt.Println("Server Message: " + text[n:])

			if strings.Contains(text, "Sorry") {
				// There were no results
				fmt.Println("No results returned for that search...")
				statusC <- !isBook
			} else if strings.Contains(text, "try another server") {
				fmt.Println("That server is not available. Try again...")
				statusC <- !isBook
			} else if strings.Contains(text, "has been accepted") {
				fmt.Println("Search has been accepted. Please wait.")
			} else if strings.Contains(text, "matches") {
				start := strings.LastIndex(text, "returned") + 9
				end := strings.LastIndex(text, "matches") - 1
				fmt.Println("Your search returned " + text[start:end] + " matches.")
			}
		}

		select {
		case <-doneChan:
			// Send message when finished downloading
			statusC <- true
		default:
		}
	}
}
