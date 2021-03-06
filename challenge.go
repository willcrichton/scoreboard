/**************************************************
 * challenge.go
 * Displays the challenge page and handles challenge
 * submissions.
 **************************************************/

package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"labix.org/v2/mgo/bson"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type score struct {
	Andrew string
	Score  int
	Time   int
	Place  int
}

type challenge struct {
	Week        int
	Name        string
	Public      bool
	Scores      []score
	Description string
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
		Admin       bool
		Andrew      string
		LoggedIn    bool
		Root        string
		Page        string
		Week        int
		Description template.HTML
		Name        string
		List        bool
		Past        []challenge
		Active      bool
		Scores      []score
	}
	data.LoggedIn = session.Values["logged_in"] == "yes"
	data.Root = htmlRoot
	data.Andrew = session.Values["andrew"].(string)
	data.Page = "challenge"
	data.Admin = isAdmin(session.Values["andrew"].(string))

	weekStr := r.URL.Query().Get("week")
	if chActive || weekStr != "" {
		// if a challenge is specified, show that challenge's full description
		var ch challenge
		var week int
		if weekStr == "" {
			week = curChallenge.Week
		} else {
			week, err = strconv.Atoi(weekStr)
			if err != nil {
				http.Redirect(w, r, htmlRoot+"/challenge", http.StatusFound)
				return
			}
		}
		challenges.Find(bson.M{"week": week}).One(&ch)
		data.Week = ch.Week
		data.Name = ch.Name
		data.List = false
		data.Active = ch.Week == curChallenge.Week && chActive
		data.Description = template.HTML(ch.Description)
		n := len(ch.Scores)
		if n > 10 {
			n = 10
		}
		data.Scores = ch.Scores[:n]
	} else {
		// otherwise, output a list of previous challenges
		data.Week = -1
		data.Name = ""
		data.List = true
		data.Active = false
		challenges.Find(nil).Sort("-week").All(&data.Past)
	}

	serve("challenge.html", w, data)
}

func getSubmission(andrew string) string {
	dir, _ := ioutil.ReadDir("submission")
	rx, _ := regexp.Compile(`[^\.]+`)
	for _, stat := range dir {
		matches := rx.FindStringSubmatch(stat.Name())
		if matches[0] == andrew {
			return stat.Name()
		}
	}
	return ""
}

// when students submit responses to the challenge, they're handled here
func submitHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}

	if session.Values["logged_in"] != "yes" || !chActive {
		http.Redirect(w, r, htmlRoot+"/challenge", http.StatusFound)
		return
	}

	// todo: check in mongo to see if they've already submitted
	var cur challenge
	challenges.Find(bson.M{"week": curChallenge.Week}).One(&cur)
	for _, entry := range cur.Scores {
		if entry.Andrew == session.Values["andrew"] {
			http.Redirect(w, r, htmlRoot+"/challenge?fail=oldsubmit", http.StatusFound)
			return
		}
	}

	if getSubmission(session.Values["andrew"].(string)) != "" {
		http.Redirect(w, r, htmlRoot+"/challenge?fail=oldsubmit", http.StatusFound)
		return
	}

	submission, header, err := r.FormFile("submission")
	if err != nil {
		http.Redirect(w, r, htmlRoot+"/challenge?fail=submitfile", http.StatusFound)
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

	for conn, andrew := range connID {
		if isAdmin(andrew) {
			conn.Send(packet("received", session.Values["andrew"].(string)))
		}
	}

	http.Redirect(w, r, htmlRoot+"/challenge?success", http.StatusFound)
}
