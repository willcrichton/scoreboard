package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/sessions"
	"github.com/willcrichton/easyws"
	"html/template"
	"io"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type student struct {
	Andrew string
	Points int
}

type score struct {
	Andrew string
	Score  int
	Place  int
}

var (
	store        = sessions.NewCookieStore([]byte(SESSIONKEY))
	students     *mgo.Collection
	challenges   *mgo.Collection
	connID       = make(map[*easyws.Connection]string)
	ws           *easyws.Hub
	tmplPath     = "www"
	sessName     = "_98232session"
	htmlRoot     = ""
	fileserver   = http.FileServer(http.Dir(tmplPath))
	curChallenge challenge
	chActive     = false
)

func packet(key, value string) string {
	var p struct{ Key, Value string }
	p.Key = key
	p.Value = value
	str, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(str)
}

func wsOnMessage(msg string, c *easyws.Connection, h *easyws.Hub) {
	var result struct{ Key, Value string }
	err := json.Unmarshal([]byte(msg), &result)
	if err != nil {
		c.Send("bad message")
		return
	}
	switch result.Key {
	case "release":
		if !isAdmin(connID[c]){
			break
		}
		chActive = true
		week, _ := strconv.Atoi(result.Value)
		challenges.Find(bson.M{"week": week}).One(&curChallenge)
		curChallenge.Public = true
		challenges.Update(bson.M{"week": week}, curChallenge)
		ws.Broadcast(packet("release", result.Value))
	}
}

func wsOnJoin(c *easyws.Connection, h *easyws.Hub) {
	connID[c] = "wcrichto"
}

func router(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/", "/index.html":
		homePage(w, r)
	case "/login":
		loginHandler(w, r)
	case "/logout":
		logoutHandler(w, r)
	case "/admin":
		adminPage(w, r)
	case "/submit":
		submitHandler(w, r)
	case "/download":
		downloadHandler(w, r)
	case "/challenge":
		challengePage(w, r)
	default:
		fileserver.ServeHTTP(w, r)
	}
}

func serve(file string, w http.ResponseWriter, data interface{}) {
	t := template.New(file)
	t = t.Funcs(template.FuncMap{"eq": func(a, b string) bool { return a == b }})
	templ, err := t.ParseFiles(tmplPath+"/"+file, tmplPath+"/_header.html", tmplPath+"/_footer.html")
	if err != nil {
		panic(err)
	}
	err = templ.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}

	var data struct {
		Andrew   string
		LoggedIn bool
		Root     string
		Page     string
		Scores   []score
	}

	if session.Values["andrew"] != nil {
		data.Andrew = session.Values["andrew"].(string)
	}
	data.LoggedIn = session.Values["logged_in"] == "yes"
	data.Root = htmlRoot
	data.Page = "home"
	data.Scores = make([]score, 10)

	var result []student
	err = students.Find(nil).Sort("-points").Limit(10).All(&result)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 10; i++ {
		if i < len(result) {
			data.Scores[i] = score{
				Andrew: result[i].Andrew,
				Score:  result[i].Points,
				Place:  i + 1,
			}
		}
	}
	serve("index.html", w, data)
}

func challengePage(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}

	if session.Values["logged_in"] != "yes" {
		http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
		return
	}

	var data struct {
		Andrew   string
		LoggedIn bool
		Root     string
		Page     string
		Week     int
		Name     string
		List     bool
		Past     []challenge
		Active   bool
	}
	data.LoggedIn = session.Values["logged_in"] == "yes"
	data.Root = htmlRoot
	data.Andrew = session.Values["andrew"].(string)
	data.Page = "challenge"

	weekStr := r.URL.Query().Get("week")
	if chActive || weekStr != "" {
		var ch challenge
		if chActive {
			ch = curChallenge
		} else {
			week, err := strconv.Atoi(weekStr)
			if err != nil {
				http.Redirect(w, r, htmlRoot+"/challenge", http.StatusFound)
				return
			}
			challenges.Find(bson.M{"week": week}).One(&ch)
		}
		data.Week = ch.Week
		data.Name = ch.Name
		data.List = false
		data.Active = ch.Week == curChallenge.Week && chActive
	} else {
		data.Week = -1
		data.Name = ""
		data.List = true
		data.Active = false
		challenges.Find(nil).Sort("-week").All(&data.Past)
	}

	serve("challenge.html", w, data)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}

	if session.Values["logged_in"] != "yes" {
		http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
		return
	}

	// todo: check in mongo to see if they've already submitted

	submission, header, err := r.FormFile("submission")
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/?bad", http.StatusFound)
		return
	}
	defer submission.Close()
	ext := filepath.Ext(header.Filename)
	file, err := os.Create("submissions/" + session.Values["andrew"].(string) + ext)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	io.Copy(file, submission)

	http.Redirect(w, r, htmlRoot+"/?success", http.StatusFound)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.PostFormValue("post") != "login" {
		return
	}
	andrew := r.PostFormValue("andrew")
	pass := r.PostFormValue("password")
	hasher := sha1.New()
	hasher.Write([]byte(pass))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	var result struct{ Andrew string }
	err := students.Find(bson.M{"andrew": andrew, "password": sha}).One(&result)
	fmt.Printf("Attempted login from %s (%s)\n", andrew, sha)
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/?fail", http.StatusFound)
		return
	}

	session, err := store.Get(r, sessName)
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/?fail", http.StatusFound)
		return
	}
	session.Values["logged_in"] = "yes"
	session.Values["andrew"] = result.Andrew
	sessions.Save(r, w)

	http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/?fail", http.StatusFound)
		return
	}
	session.Values["logged_in"] = "no"
	sessions.Save(r, w)
	http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
}

func main() {
	fmt.Println("Starting server")
	// connect to mongo
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()
	students = session.DB("98232").C("students")
	challenges = session.DB("98232").C("challenges")

	// start websocket and listen on 8000
	ws = easyws.Socket(htmlRoot+"/ws", wsOnMessage, wsOnJoin)
	http.Handle(htmlRoot+"/", http.HandlerFunc(router))
	log.Fatal(http.ListenAndServe(":8000", nil))
}
