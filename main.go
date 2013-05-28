package main

import (
	"easyws"
	"fmt"
	"net/http"
	"html/template"
	"labix.org/v2/mgo"
    "labix.org/v2/mgo/bson"
	"github.com/gorilla/sessions"
	"crypto/sha1"
	"encoding/base64"
	"log"
)

type tmpl_data struct {
	Andrew   string
	LoggedIn bool
}

type user struct {
	Andrew   string
}

var (
	store = sessions.NewCookieStore([]byte("sekrut")) 
	students *mgo.Collection
	tmplPath = "www"
	sessName = "_98232session"
	fileserver = http.FileServer(http.Dir(tmplPath))
)
	

func wsOnMessage(msg string, c *easyws.Connection, h *easyws.Hub){
	fmt.Println(msg)
}

func wsOnJoin(c *easyws.Connection, h *easyws.Hub){

}

func router(w http.ResponseWriter, r *http.Request){
	switch r.URL.Path {
	case "/", "/index.html" :
		homePage(w, r)
	case "/login":
		loginHandler(w, r)
	default:
		fileserver.ServeHTTP(w, r)
	}
}

func homePage(w http.ResponseWriter, r *http.Request){
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}
	
	data := tmpl_data{}
	if session.Values["andrew"] != nil {
		data.Andrew = session.Values["andrew"].(string)
	}
	data.LoggedIn = session.Values["logged_in"] == "yes"

	t := template.New("index.html")
	templ, err := t.ParseFiles(tmplPath + "/index.html")
	if err != nil {
		panic(err)
	}
	err = templ.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request){
	if r.PostFormValue("post") != "login" {
		return;
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
		http.Redirect(w, r, htmlRoot + "/?fail", http.StatusFound)
			return
	}

	session, err := store.Get(r, sessName)
	if err != nil {
		http.Redirect(w, r, htmlRoot + "/?fail", http.StatusFound)
		return
	}
	session.Values["logged_in"] = "yes"
	session.Values["andrew"] = result.Andrew
	sessions.Save(r, w)

	http.Redirect(w, r, htmlRoot + "/", http.StatusFound)
}

func main(){
	fmt.Println("Starting server")
	// connect to mongo
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()
	students = session.DB("98232").C("students");
	
	// start websocket and listen on 8000
	easyws.Socket(htmlRoot + "/ws", wsOnMessage, wsOnJoin)
	http.Handle(htmlRoot + "/", http.HandlerFunc(router))
	log.Fatal(http.ListenAndServe(":8000", nil))
}