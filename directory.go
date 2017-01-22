package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/garyburd/redigo/redis"
)

const (
	error_resp = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>
`

	pass_resp = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<document type="freeswitch/xml">
 <section name="directory" description="User Directory">
  <domain name="{{.Domain}}">
   <params>
    <param name="dial-string" value="{rtp_secure_media=${regex(${sofia_contact(*/${dialed_user})}|transport=tls)},presence_id=*/${dialed_user}@${dialed_domain}}${sofia_contact(*/${dialed_user})}"/>
   </params>
   <groups>
    <group name="default">
     <users>
      <user id="{{.Userid}}">
       <params>
        <param name="password" value="{{.Password}}"/>
       </params>
       <variables>
        <variable name="user_context" value="internal"/>
        <variable name="effective_caller_id_name" value="{{.Name}}"/>
        <variable name="effective_caller_id_number" value="{{.Number}}"/>
       </variables>
      </user>
     </users>
    </group>
   </groups>
  </domain>
 </section>
</document>
`
)

type (
	okResp struct {
		Domain   string
		Userid   string
		Password string
		Name     string
		Number   string
	}

	TAuthData struct {
		Name     string `json:"name"`
		Number   string `json:"number"`
		Password string `json:"pass"`
	}

	TConfig struct {
		Listen struct {
			Host string `json:"host"`
			Port string `json:"port"`
		} `json:"listen"`
		RedisLocal struct {
			Host      string `json:"host"`
			Port      string `json:"port"`
			Auth      string `json:"auth"`
			KeyPrefix string `json:"key_prefix"`
		} `json:"redis_local"`
		Domans []string `json:"domains"`
	}
)

var (
	err    error
	pool   *redis.Pool
	config *TConfig
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: directory <config_file>")
	}

	confFile := os.Args[1]
	getConfig(confFile)

	redisServer := config.RedisLocal.Host + ":" + config.RedisLocal.Port
	redisPassword := config.RedisLocal.Auth
	pool = newPool(redisServer, redisPassword)
	// Check redis connect
	if _, err = pool.Dial(); err != nil {
		log.Fatalln("Redis Driver Error", err)
	}

	http.HandleFunc("/directory", directory)
	err := http.ListenAndServe(config.Listen.Host+":"+config.Listen.Port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func directory(w http.ResponseWriter, r *http.Request) {
	redispool := pool.Get()
	defer redispool.Close()

	w.Header().Set("Content-type", "text/xml")
	authError := func() {
		w.Write([]byte(error_resp))
	}

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		eventName := r.Form.Get("Event-Name")
		validate("Event-Name", &eventName)

		domain := r.Form.Get("domain")
		validate("domain", &domain)

		user := r.Form.Get("user")

		redisresp, err := redis.Bytes(redispool.Do("GET", config.RedisLocal.KeyPrefix+user))
		if err != nil {
			authError()
			return
		}

		var rrr TAuthData
		err = json.Unmarshal(redisresp, &rrr)
		if err != nil {
			authError()
			return
		}

		if eventName != "" && domain != "" && user != "" && rrr.Password != "" {
			tpass, _ := template.New("pass_resp").Parse(pass_resp)
			tpass.Execute(w, okResp{domain, user, rrr.Password, rrr.Name, rrr.Number})
			return
		}
		authError()
	}
}

func validate(param string, value *string) {
	switch param {
	case "Event-Name":
		if *value != "REQUEST_PARAMS" {
			*value = ""
		}
	case "domain":
		domain := config.Domans
		for _, v := range domain {
			if v == *value {
				return
			}
		}
		*value = ""
	}
}

func newPool(server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		MaxActive:   0,
		IdleTimeout: 180 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func getConfig(file_path string) {
	f, err := os.Open(file_path)
	if err != nil {
		log.Fatal("error:", err)
		return
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal("error:", err)
	}
}
