package main

import (
	"io/ioutil"
	"net/http"
	"regexp"
)

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

	var data struct {
		LoggedIn    bool
		Andrew      string
		Root        string
		Submissions []string
	}
	data.LoggedIn = true
	data.Andrew = session.Values["andrew"].(string)
	data.Root = htmlRoot

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

	serve("admin.html", w, data)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {

}
