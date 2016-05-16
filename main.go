package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

type TimeStatus struct {
	BeginTime time.Time
	DoneTime  time.Time
}

func (t TimeStatus) Duration() time.Duration {
	return t.DoneTime.Sub(t.BeginTime)
}

var wg sync.WaitGroup
var mutex sync.Mutex
var doneChannel chan TimeStatus

var flag bool

func main() {
	doneChannel = make(chan TimeStatus)
	fmt.Printf("%v\n", os.Args)
	go ChannelDrain(doneChannel)
	HttpServe()
}

func HttpServe() {
	r := mux.NewRouter()
	r.HandleFunc("/start", CallHandler)
	http.Handle("/", r)
	n := negroni.Classic()
	n.UseHandler(r)

	log.Fatal(http.ListenAndServe(":44005", n))
}

func CallHandler(rw http.ResponseWriter, req *http.Request) {
	if flag == false {
		go CallCmd(doneChannel)
		fmt.Fprintf(rw, "[%v] called %v\n", time.Now().String(), os.Args[1:])
	} else {
		fmt.Fprintf(rw, "current task is running\n")
	}

}

func CallCmd(ch chan TimeStatus) {
	var ts TimeStatus
	ts.BeginTime = time.Now()
	mutex.Lock()
	flag = true
	mutex.Unlock()

	var OutBuf bytes.Buffer
	var ErrBuf bytes.Buffer
	params := os.Args[2:]
	cmd := exec.Command(os.Args[1], params...)
	cmd.Stdout = &OutBuf
	cmd.Stderr = &ErrBuf

	err := cmd.Start()
	if err != nil {
		log.Panicf("Error starting %v: %v\n", os.Args[1], err)
	}

	err = cmd.Wait()
	if err != nil && err.Error() != "exit status 1" {
		fmt.Printf("stdout:\n%v\n", OutBuf.String())
		fmt.Printf("stderr:\n%v\n", ErrBuf.String())
		log.Panicf("Error waiting %v: %v\n", os.Args[1], err)
	}

	mutex.Lock()
	flag = false
	mutex.Unlock()

	ts.DoneTime = time.Now()
	ch <- ts
}

func ChannelDrain(ch chan TimeStatus) {
	for b := range ch {
		var body []byte
		log.Printf("request done: %v\n", b.Duration())

		url := "http://192.168.122.1:44005/status"
		rawjsonstr := fmt.Sprintf("{\"status\":\"%v\", \"From\":\"%v\"}", "ok", "SSD")
		jsonstr := []byte(rawjsonstr)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonstr))
		if err != nil {
			log.Panicf("Error Creating Request: %v\n", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error requesting: %v\n", err)
			goto EndFunc
		}
		body, _ = ioutil.ReadAll(resp.Body)
		log.Printf("status: %v\nResponse Header: %v\nResponse Body: %v\n", resp.Status, resp.Header, string(body))
		//Nasty bastard defer nil panic sh*t
		defer resp.Body.Close()
	EndFunc:
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in ChannelDrain %v\n", r)
			}
		}()
	}
}
