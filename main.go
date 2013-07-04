/***************************************************
 * Scoreboard, a Go programming competition hoster *
 * Created by Will Crichton, 2013                  *
 * In order to run the executable, you will need:  *
 * 1. MongoDB running locally                      *
 * 2. The following directory structure:           *
 *    - scoreboard                                 *
 *    - hidden/                                    *
 *    - submissions/                               *
 *    - www/                                       *
 ***************************************************/

package main

import (
	"crypto/sha1"
	"easyws"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/gorilla/sessions"
	"html/template"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"net/http"
	"strconv"
)

type student struct {
	Andrew   string
	Points   int
	Password string
}

type leaderboard struct {
	Andrew string
	Score  int
	Place  int
}

var (
	store        = sessions.NewCookieStore([]byte(SESSIONKEY)) // Andrew stored in cookie
	students     *mgo.Collection                               // mongo set of students in db
	challenges   *mgo.Collection                               // mongo set of all challenges
	connID       = make(map[*easyws.Connection]string)         // map from conn to andrew id
	ws           *easyws.Hub                                   // websocket server
	tmplPath     = "www"                                       // path to templates (rel. to executable)
	sessName     = "_98232session"                             // name of cookie
	htmlRoot     = ""                                          // root of fileserver
	fileserver   = http.FileServer(http.Dir(tmplPath))         // fs object for serving static stuff
	curChallenge challenge                                     // holds challenge obj if active
	chActive     = false                                       // if challenge is happening now
	port         = flag.Int("port", 8000, "Application port")  // port that the app runs on (default 8000)
)

// sends page requests to the appropriate handlers
func router(w http.ResponseWriter, r *http.Request) {
	// giant switch statements wheeeeeeeee
	// todo: make this less switchy
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
		// by default, assume they're asking for a static file
		fileserver.ServeHTTP(w, r)
	}
}

// write template to client w/ appropriate data and header/footer
func serve(file string, w http.ResponseWriter, data interface{}) {

	// load template
	t := template.New(file)

	// give template an equality function for strings
	t = t.Funcs(template.FuncMap{"eq": func(a, b string) bool { return a == b }})

	// parse template, making sure to load header and footer files
	templ, err := t.ParseFiles(tmplPath+"/"+file, tmplPath+"/_header.html", tmplPath+"/_footer.html")
	if err != nil {
		panic(err)
	}

	// output generated html to REsponseWriter
	err = templ.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	// get logged-in status
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}

	// we pass this data to the template
	var data struct {
		Admin    bool
		Andrew   string
		LoggedIn bool
		Root     string
		Page     string
		Scores   []leaderboard
	}

	if session.Values["andrew"] != nil {
		data.Andrew = session.Values["andrew"].(string)
	}
	data.LoggedIn = session.Values["logged_in"] == "yes"
	data.Root = htmlRoot
	data.Page = "home"
	data.Scores = make([]leaderboard, 10)
	data.Admin = isAdmin(session.Values["andrew"].(string))

	// get the leaderboard from students collection
	var result []student
	err = students.Find(nil).Sort("-points").Limit(10).All(&result)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 10; i++ {
		if i < len(result) {
			data.Scores[i] = leaderboard{
				Andrew: result[i].Andrew,
				Score:  result[i].Points,
				Place:  i + 1,
			}
		}
	}
	serve("index.html", w, data)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.PostFormValue("post") != "login" {
		return
	}
	// get password and sha1 hash it
	// todo: make more secure passwording
	andrew := r.PostFormValue("andrew")
	pass := r.PostFormValue("password")
	hasher := sha1.New()
	hasher.Write([]byte(pass))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	// check user/pass combo in db
	var result struct{ Andrew string }
	err := students.Find(bson.M{"andrew": andrew, "password": sha}).One(&result)
	fmt.Printf("Attempted login from %s (%s)\n", andrew, sha)
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/?fail=login", http.StatusFound)
		return
	}

	// passed the test, log 'em in
	session, err := store.Get(r, sessName)
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/?fail=cookie", http.StatusFound)
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
		http.Redirect(w, r, htmlRoot+"/?fail=cookie", http.StatusFound)
		return
	}

	// set cookie to indicate they're logged out
	session.Values["logged_in"] = "no"
	sessions.Save(r, w)
	http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
}

func main() {
	flag.Parse()
	portStr := strconv.Itoa(*port)

	// note: in /usr/local/etc/mongod.conf, set bind_ip = 127.0.0.1
	//       to prevent tricksy remote connections
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}

	// close mongo session once main exits
	defer session.Close()

	// get the relevant collections into our globals
	students = session.DB("98232").C("students")
	challenges = session.DB("98232").C("challenges")

	// start websocket and listen on 8000
	ws = easyws.Socket(htmlRoot+"/ws", wsOnMessage, wsOnJoin, wsOnLeave)

	// add router to our http server
	http.Handle(htmlRoot+"/", http.HandlerFunc(router))

	// start server
	log.Fatal(http.ListenAndServe(":"+portStr, nil))
	fmt.Println("Starting server on :" + portStr)
}
