package main

import (
	"bytes"
	"fmt"
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
	if err != nil {
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
		log.Printf("request done: %v\n", b.Duration())
	}
}