package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"labix.org/v2/mgo/bson"
)

// right now we have two statically assigned admins
// todo: admin management
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

	// adding a new challenge to the db (i.e. form on the right was submitted)
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
		Page        string
		Submissions []struct {
			Andrew string
			Done   bool
		}
		Challenges  []challenge
		Active      bool
	}
	data.LoggedIn = true
	data.Andrew = session.Values["andrew"].(string)
	data.Root = htmlRoot
	data.Page = "admin"
	data.Active = chActive

	// read current submission list from directory
	// todo: leave this work to mongo, not file reading
	if chActive {
		dir, err := ioutil.ReadDir("submissions")
		if err != nil {
			panic(err)
		}
		rx, _ := regexp.Compile(`[^\.]+`)
		for _, stat := range dir {
			matches := rx.FindStringSubmatch(stat.Name())
			andrew := matches[0]
			var ch challenge
			var sub struct { Andrew string; Done bool }
			sub.Andrew = andrew
			sub.Done = false
			challenges.Find(bson.M{"week": curChallenge.Week}).One(&ch)
			for _, entry := range ch.Scores {
				if entry.Andrew == andrew {
					sub.Done = true
					break
				}
			}
			data.Submissions = append(data.Submissions, sub)
		}

		// get challenge list from mongo
	}
	challenges.Find(nil).Sort("-week").All(&data.Challenges)

	serve("admin.html", w, data)
}

/* when an admin requests a download for a submission, we gotta serve it
 * up a little bit specially because submissions should never be exposed to
 * peering eyes, so we have a submissions directory outside of the html root.
 * granted, we could just protect the html directory, but this is more fun!
 */
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

	// find requested file in directory (gotta search b/c ext unknown)
	dir, err := ioutil.ReadDir("submissions")
	rx, _ := regexp.Compile(`[^\.]+`)
	for _, stat := range dir {
		matches := rx.FindStringSubmatch(stat.Name())
		if matches[0] == andrew {
			// todo: use Content-Disposition or whatever to name file
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
