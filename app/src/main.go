package main

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/CrowdSurge/banner"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// AppName app
//var AppName = "name"
var AppName = os.Getenv("APP_NAME")

// Version app
var Version = "version"

// BuildInfo app
var BuildInfo = "commit"

// Revision app
var Revision = fmt.Sprintf("%s version: %s+%s", AppName, Version, BuildInfo)

// AppPort app
var AppPort = os.Getenv("APP_PORT")

// AppDb name
var AppDb = "db/name"

// NewFeature changes mock
var NewFeature = ""

type greetingsText struct {
	Text string `json:"Text"`
}

type greetingsToken struct {
	Hash string `json:"Hash"`
}

type msisdn struct {
	Msisdn string `json:"msisdn"`
}

func main() {
	log.Print(Revision)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/version", versionHandler)
	router.HandleFunc("/healthz", healthzHandler)
	router.HandleFunc("/readinez", readinessHandler)

	switch AppName {
	case "front":
		router.HandleFunc("/", frontHandler)

	case "service":
		router.HandleFunc("/", serviceHandler)

	case "data":
		router.HandleFunc("/", dataHandler)

	case "rate":
		router.HandleFunc("/", rateHandler)

	}
	log.Fatal(http.ListenAndServe(":"+AppPort, router))
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	var b []byte
	b = append([]byte(""), Revision...)
	w.Write(b)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("Healthz: alive!"))
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {

	switch AppName {

	case "front":
		w.Write([]byte("OK"))

	case "service":
		client := redis.NewClient(&redis.Options{
			Addr:     "redis:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		})
		probe, err := client.Ping().Result()
		log.Print(probe, err)
		if err != nil {
			http.Error(w, "Not Ready", http.StatusServiceUnavailable)
		}

	case "data":
		client := redis.NewClient(&redis.Options{
			Addr:     "redis:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		})
		probe, err := client.Set("readiness_probe", 0, 0).Result()
		log.Print(probe)
		if err != nil {
			http.Error(w, "Not Ready", http.StatusServiceUnavailable)
		}

		db, err := sql.Open("mysql", AppDb)
		if err != nil {
			http.Error(w, "Not Ready", http.StatusServiceUnavailable)
		}
		defer db.Close()
		err = db.Ping()

		if err != nil {
			http.Error(w, "Not Ready", http.StatusServiceUnavailable)
		}

		w.Write([]byte("200"))

	default:
		http.Error(w, "Not Ready", http.StatusServiceUnavailable)

	}

}

func frontHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf("%s", rest("http://service", `{"text":"kubernetes bootcamp"}`))))

}

func serviceHandler(w http.ResponseWriter, r *http.Request) {
	var m greetingsText
	switch r.Method {
	case "GET":
		log.Printf("Get GET Request!")
		w.Write([]byte("Please use POST"))

	case "POST":
		b, _ := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err := json.Unmarshal(b, &m); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(w).Encode(err); err != nil {
				panic(err)
			}
		}
		if NewFeature != "" {
			m.Text = NewFeature
		}
		log.Print("Text: ", m.Text)
		hashStr := fmt.Sprintf(`{"hash":"%s"}`, greetingsID(m.Text))
		log.Print("Hash:", hashStr)
		w.Write(rest("http://data", hashStr))

	}
}

func greetingsID(decodedStr string) string {
	client := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	log.Print("DecodedStr: ", decodedStr)
	encodedStr := hex.EncodeToString([]byte(banner.PrintS(decodedStr)))
	log.Print("EncodedStr: ", encodedStr)
	hashStr := fmt.Sprintf("%x", md5.Sum([]byte(encodedStr)))
	client.Set(hashStr, encodedStr, 0)
	return hashStr
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	var m greetingsToken
	switch r.Method {
	case "GET":
		log.Printf("Get GET Request!")
		w.Write([]byte("Please use POST"))

	case "POST":
		b, _ := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err := json.Unmarshal(b, &m); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(w).Encode(err); err != nil {
				panic(err)
			}
		}
		w.Write([]byte(greetingsDB(m.Hash)))
	}
}

func greetingsDB(hash string) string {
	var Payload string

	db, err := sql.Open("mysql", AppDb)
	if err != nil {
		log.Print("Open db err: ")
		panic(err)
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Print("Ping db err: ")
		panic(err.Error()) // proper error handling instead of panic in your app
	}

	client := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	hexStr, err := client.Get(hash).Result()
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS demoTable (id INT NOT NULL AUTO_INCREMENT, token VARCHAR(100), text TEXT, PRIMARY KEY(id))")
	_, err = db.Exec("insert into demoTable values(null,?,?)", hash, hexStr)

	err = db.QueryRow("SELECT text FROM demoTable WHERE token = ?", hash).Scan(&Payload) // WHERE number = 13
	if err != nil {
		panic(err.Error()) // proper error handling instead of panic in your app
	}
	decoded, err := hex.DecodeString(Payload)
	return string(decoded)
}

func rest(url string, jsonStr string) []byte {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonStr)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Second * 5}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	log.Print("response Status:", resp.Status)
	log.Print("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	return body
}

func readiness(url string) string {
	c := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := c.Get(url)
	if err != nil {
		log.Print(err)
		return resp.Status
	}
	defer resp.Body.Close()

	log.Print("response Status:", resp.Status)
	return resp.Status
}

func rateHandler(w http.ResponseWriter, r *http.Request) {
	var m msisdn
	switch r.Method {
	case "GET":
		log.Printf("Get GET Request!")
		w.Write([]byte("Please use POST"))

	case "POST":
		b, _ := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err := json.Unmarshal(b, &m); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(422) // unprocessable entity
			if err := json.NewEncoder(w).Encode(err); err != nil {
				panic(err)
			}
		}
		w.Write([]byte(getRate(m.Msisdn)))
	}
}

/**
func getRate(msisdn string) string {
	client := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	log.Print("MSISDN: ", msisdn)
	client.HGetAll('RATE_CACHE:'+substr(msisdn,0,$CONF{'rate_cache_len'}))
	%{ SQL(TEMPLATE('sql:get_rate'),'hash')
	if (!scalar keys %{$Q{CACHE}}){
		$R->HSET('RATE_CACHE:'.substr($Q{MSISDN},0,$CONF{'rate_cache_len'}),$_,$HASH{$_}, sub{}) for keys %HASH;
		$R->wait_one_response;
		$R->EXPIRE('RATE_CACHE:'.substr($Q{MSISDN},0,$CONF{'rate_cache_len'}),defined $CONF{'rate_cache_ttl'} ? $CONF{'rate_cache_ttl'} :86400 );
		}#if cache

		map {$Q{$_}=$HASH{$_}} keys %HASH; #map for DID call
		$Q{'CALL_RATE'}=($HASH{'RATE'}*$CONF{'markup_rate'}+$Q{MTC})/100; #notation 1.2
		$Q{'CALL_LIMIT'}=($Q{'CALL_LIMIT'}=floor(($Q{'CARD_BALANCE_AVAIL'})/$Q{'CALL_RATE'})*60)>$CONF{'max-call-time'} ? $CONF{'max-call-time'} : $Q{'CALL_LIMIT'}; #notation seconds

		logger('LOG','CALL_RATE-[rate|limit|mtc]:',"$Q{'CALL_RATE'}:$Q{'CALL_LIMIT'}:$Q{MTC}") if $CONF{debug}>2;


	return hashStr
}
**/

func getRate(msisdn string) string {
	var Rate, Trunk, Prefix, RateId string
	db, err := sql.Open("sqlite3", "./msrn.db")
	if err != nil {
		log.Print("Open db err: ")
		panic(err)
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Print("Ping db err: ")
		panic(err.Error()) // proper error handling instead of panic in your app
	}

	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379", //Addr:     "redis:6379",
		Password: "",               // no password set
		DB:       0,                // use default DB
	})

	Rate, err = client.HGet("RATE_CACHE:"+msisdn[0:5], "RATE").Result()
	if err != nil {
		log.Print(err)
		err = db.QueryRow("select rate*100 RATE, trunk TRUNK, dial PREFIX, id RATEID FROM RATES WHERE (instr(?,prefix)=1) ORDER BY len DESC,rate LIMIT 1", msisdn).Scan(&Rate, &Trunk, &Prefix, &RateId)

		if err != nil {
			log.Print(err.Error()) // proper error handling instead of panic in your app
		}
		client.HSet("RATE_CACHE:"+msisdn[0:5], "RATE", Rate)
		client.Expire("RATE_CACHE:"+msisdn[0:5], 10*time.Second)
	}

	return Rate
}