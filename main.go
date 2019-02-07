package main

import (
  "bytes"
  "encoding/binary"
  "encoding/json"
  "errors"
  "flag"
  "net/http"
  "log"
  "regexp"
  "strconv"

  "github.com/dgraph-io/badger"
	"github.com/julienschmidt/httprouter"
  "github.com/nats-io/go-nats"
  "github.com/golang/protobuf/proto"
)

var listen string
var dbPath string
var natsHost string

var db *badger.DB

func main() {
  // Parse flags
  flag.StringVar(&listen, "listen", ":8080", "host and port to listen on")
  flag.StringVar(&dbPath, "dbpath", "/tmp/badger", "path to store data")
  flag.StringVar(&natsHost, "nats", "nats://localhost:4222", "host and port of NATS")
	flag.Parse()

  // Open badger
	log.Printf("starting badger at %s", dbPath)
	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath
	var err error
	db, err = badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

  // NATS client
  nc, _ := nats.Connect(natsHost);
  nc.Subscribe("new_bite", NewBite);
  nc.Subscribe("new_bite_user", NewBiteUser);

  // Routes
	router := httprouter.New()
	router.GET("/conversation/:key/scan", ScanBites) // Scanning
  router.GET("/conversation/:key/start/:start", GetBite) // GET bites
  router.GET("/conversation/:key/start/:start/user", GetBiteUser) // GET bite_users

  // Start server
  log.Printf("starting server on %s", listen)
	log.Fatal(http.ListenAndServe(listen, router))
}

// Marshal keys
func validObj(obj string) bool {
	return obj == "bite" || obj == "user"
}

// TODO: ensure security of regexp
var validConversationRegexp = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

func validConversation(conversation string) bool {
	return validConversationRegexp.MatchString(conversation)
}

const conversationSeprator = '@'
const objSeprator = '+'

func MarshalKey(obj, conversation string, start uint64) ([]byte, error) {
	prefixBytes, err := MarshalKeyPrefix(obj, conversation)
	if err != nil {
		return nil, err
	}

	startBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(startBytes, start)

	return append(prefixBytes, startBytes...), nil
}

func MarshalKeyPrefix(obj, conversation string) ([]byte, error) {
	if !validObj(obj) || !validConversation(conversation) {
		return nil, errors.New("main: FormatKey: bad obj or conversation")
	}
	return []byte(obj + string(objSeprator) + conversation + string(conversationSeprator)), nil
}

func ExtractKey(b []byte) (string, string, uint64, error) {
	startStart := bytes.LastIndexByte(b, conversationSeprator) + 1
	if startStart < 0 {
		return "", "", 0, ExtractKeyParseError
	}
	startBytes := b[startStart:]

	convStart := bytes.LastIndexByte(b[:startStart-1], objSeprator) + 1
	if convStart < 0 {
		return "", "", 0, ExtractKeyParseError
	}
	convBytes := b[convStart : startStart-1]

	objStart := 0
	if objStart < 0 {
		return "", "", 0, ExtractKeyParseError
	}
	objBytes := b[objStart : convStart-1]

	obj := string(objBytes)
	conv := string(convBytes)
	start := binary.BigEndian.Uint64(startBytes)

	return obj, conv, start, nil
}

var ExtractKeyParseError = errors.New("ExtractKey: parse error, possibly because seprator was not found")

func ParseStartString(start string) (uint64, error) {
	return strconv.ParseUint(start, 10, 64)
}

// Sub handlers
// m.data = Bite protobuf
func NewBite(m *nats.Msg) {
  bite := Bite{}
  if err := proto.Unmarshal(m.Data, &bite); err != nil {
    log.Println(err)
    return
  }

  key, err := MarshalKey("bite", bite.Key, bite.Start)
  if err != nil {
    log.Println(err)
    return
  }

  err = db.Update(func(txn *badger.Txn) error {
		// TODO: prevent overwriting existing
		err := txn.Set(key, bite.Data)
		return err
	})

  if err != nil {
    log.Println(err)
    return
  }
}

func NewBiteUser(m *nats.Msg) {
  bite := Bite{}
  if err := proto.Unmarshal(m.Data, &bite); err != nil {
    log.Println(err)
    return
  }

  key, err := MarshalKey("user", bite.Key, bite.Start)
  if err != nil {
    log.Println(err)
    return
  }

  err = db.Update(func(txn *badger.Txn) error {
    // TODO: prevent overwriting existing
    err := txn.Set(key, bite.Data)
    return err
  })

  if err != nil {
    log.Println(err)
    return
  }
}

// rOUTE HANDLERS

type BitesList struct {
	Previous uint64   `json:"previous"` // One bite before starts. Hint for how many steps the client can skip
	Starts   []uint64 `json:"starts"`
	Next     uint64   `json:"next"` // One bite after starts. Hint for how many steps the client can skip
}

func ScanBites(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  prefix, err := MarshalKeyPrefix("bite", p.ByName("key"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

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

	fromKey, err := MarshalKey("bite", p.ByName("key"), from)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	bitesList := BitesList{}

	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		// Fetch previous key
		it.Seek(fromKey)
		if it.ValidForPrefix(fromKey) {
			// Lazy check to compare key == seeked key
			it.Next()
		}
		if !it.ValidForPrefix(prefix) {
			return nil
		}
		item := it.Item()
		key := item.Key()

		_, _, start, err := ExtractKey(key)
		if err != nil {
			return nil
		}
		bitesList.Previous = start

		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(fromKey); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			_, _, start, err := ExtractKey(key)
			if err != nil {
				continue
			}
			if start > to {
				// A key was found that is greater than to
				// Save that as next
				bitesList.Next = start
				break
			}

			bitesList.Starts = append(bitesList.Starts, start)
		}

		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bitesList)
}

func GetBite(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	w.Header().Add("Content-Type", "audio/wav")
	start, err := ParseStartString(p.ByName("start"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	key, err := MarshalKey("bite", p.ByName("key"), start)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
    err = item.Value(func(value []byte) error {
      w.Write(value)
      return nil
    })
    if err != nil {
      return err
    }
    return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func GetBiteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	w.Header().Add("Content-Type", "text/plain")
	start, err := ParseStartString(p.ByName("start"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	key, err := MarshalKey("user", p.ByName("key"), start)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
    err = item.Value(func(value []byte) error {
      w.Write(value)
      return nil
    })
    if err != nil {
      return err
    }
    return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}
