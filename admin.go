package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
)

type challenge struct {
	Week   int
	Name   string
	Public bool
	Scores []struct {
		Andrew string
		Score  int
	}
}

func isAdmin(user string) bool {
	return user == ROOT1 || user == ROOT2
}

func adminPage(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookie"))
		return
	}

	if session.Values["logged_in"] != "yes" || !isAdmin(session.Values["andrew"].(string)) {
		http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
		return
	}

	if r.FormValue("post") == "challenge" {
		week, _ := strconv.Atoi(r.FormValue("week"))
		ch := challenge{
			Week:   week,
			Name:   r.FormValue("name"),
			Public: false,
		}
		challenges.Insert(ch)
		http.Redirect(w, r, htmlRoot+"/admin?success", http.StatusFound)
		return
	}

	var data struct {
		LoggedIn    bool
		Andrew      string
		Root        string
		Submissions []string
		Challenges  []challenge
	}
	data.LoggedIn = true
	data.Andrew = session.Values["andrew"].(string)
	data.Root = htmlRoot

	// read current submission from directory (switch to mongo?)
	dir, err := ioutil.ReadDir("submissions")
	if err != nil {
		panic(err)
	}
	data.Submissions = make([]string, len(dir))
	i := 0
	rx, _ := regexp.Compile(`[^\.]+`)
	for _, stat := range dir {
		matches := rx.FindStringSubmatch(stat.Name())
		data.Submissions[i] = matches[0]
		i++
	}

	// get challenge list from mongo
	challenges.Find(nil).Sort("-week").All(&data.Challenges)

	serve("admin.html", w, data)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookie"))
		return
	}

	if session.Values["logged_in"] != "yes" || !isAdmin(session.Values["andrew"].(string)) {
		http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
		return
	}

	andrew := r.URL.Query().Get("user")
	if andrew == "" {
		http.Redirect(w, r, htmlRoot+"/admin", http.StatusFound)
	}

	dir, err := ioutil.ReadDir("submissions")
	rx, _ := regexp.Compile(`[^\.]+`)
	for _, stat := range dir {
		matches := rx.FindStringSubmatch(stat.Name())
		if matches[0] == andrew {
			file, err := os.Open("submissions/" + stat.Name())
			if err != nil {
				panic(err)
			}
			buffer := make([]byte, stat.Size())
			_, err = file.Read(buffer)
			if err != nil {
				panic(err)
			}
			w.Write(buffer)
			return
		}
	}
	http.Redirect(w, r, htmlRoot+"/admin", http.StatusFound)
}
