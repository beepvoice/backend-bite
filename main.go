package main

import (
  "net/http"
  "log"
  "os"
  "strconv"
  "time"

  "github.com/julienschmidt/httprouter"
  "github.com/nats-io/go-nats"
  "github.com/golang/protobuf/proto"
  "github.com/joho/godotenv"
)

var listen string
var natsHost string

var natsConn *nats.Conn

func main() {
  // Load .env
  err := godotenv.Load()
  if err != nil {
    log.Fatal("Error loading .env file")
  }
  listen = os.Getenv("LISTEN")
  natsHost = os.Getenv("NATS")

  // NATS client
  natsConn, err := nats.Connect(natsHost)
  if err != nil {
    log.Println(err)
    return
  }
  natsConn.Subscribe("new_bite", NewBite)
  natsConn.Subscribe("new_bite_user", NewBiteUser)
  defer natsConn.Close()

  // Routes
	router := httprouter.New()
	router.GET("/conversation/:key/scan", ScanBites) // Scanning
  router.GET("/conversation/:key/start/:start", GetBite) // GET bites
  router.GET("/conversation/:key/start/:start/user", GetBiteUser) // GET bite_users

  // Start server
  log.Printf("starting server on %s", listen)
	log.Fatal(http.ListenAndServe(listen, router))
}

func ParseStartString(start string) (uint64, error) {
	return strconv.ParseUint(start, 10, 64)
}

// Sub handlers
// m.data = Bite protobuf
func NewBite(m *nats.Msg) {
  New("bite", m)
}

func NewBiteUser(m *nats.Msg) {
  New("user", m)
}

func New(t string, m *nats.Msg) {
  bite := Bite{}
  if err := proto.Unmarshal(m.Data, &bite); err != nil {
    log.Println(err)
    return
  }

  storeRequest := Store {
    Type: t,
    Bite: &bite,
  }
  reqBytes, err := proto.Marshal(&storeRequest)

  if err != nil {
    log.Print(err)
    return
	}
  natsConn.Publish("new_store", reqBytes)
}

// Route handlers
func ScanBites(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  from, err := ParseStartString(r.FormValue("from"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	to, err := ParseStartString(r.FormValue("to"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

  scanRequest := ScanRequest {
    Key: p.ByName("key"),
    From: from,
    To: to,
    Type: "bite",
  }

  drBytes, err := proto.Marshal(&scanRequest);
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  msg, err := natsConn.Request("scan_store", drBytes, 10 * 1000 * time.Millisecond) // 10s timeout
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  res := Response {}
  if err := proto.Unmarshal(msg.Data, &res); err != nil {
    log.Println(err)
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  if res.Code == 200 {
    w.Header().Set("Content-Type", "application/json")
  	w.Write(res.Message)
  } else if len(res.Message) == 0 {
    http.Error(w, http.StatusText(int(res.Code)), int(res.Code))
  } else {
    http.Error(w, string(res.Message), int(res.Code))
  }
}

func GetBite(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  Get("bite", w, r, p)
}

func GetBiteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  Get("user", w, r, p)
}

func Get(t string, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  start, err := ParseStartString(p.ByName("start"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

  dataRequest := DataRequest {
    Key: p.ByName("key"),
    Start: start,
    Type: t,
  }

  drBytes, err := proto.Marshal(&dataRequest);
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  msg, err := natsConn.Request("request_store", drBytes, 10 * 1000 * time.Millisecond) // 10s timeout
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  res := Response {}
  if err := proto.Unmarshal(msg.Data, &res); err != nil {
    log.Println(err)
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  if res.Code == 200 {
    w.Header().Add("Content-Type", "audio/wav")
  	w.Write(res.Message)
  } else if len(res.Message) == 0 {
    http.Error(w, http.StatusText(int(res.Code)), int(res.Code))
  } else {
    http.Error(w, string(res.Message), int(res.Code))
  }
}
