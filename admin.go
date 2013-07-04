/**************************************************
 * admin.go
 * Displays the administration page
 * Does submission validation and challenge editing
 **************************************************/

package main

import (
        "io/ioutil"
        "labix.org/v2/mgo/bson"
        "net/http"
        "os"
        "regexp"
        "strconv"
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

        // check user authentication
        if session.Values["logged_in"] != "yes" || !isAdmin(session.Values["andrew"].(string)) {
                http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
                return
        }

        // if they post to the page, do operations on challenge collection
        if r.FormValue("post") == "challenge" {
                week, _ := strconv.Atoi(r.FormValue("week"))

                if r.FormValue("edit") != "" {
                        // update challenge in db if they're editing
                        edit, _ := strconv.Atoi(r.FormValue("edit"))
                        var ch challenge
                        challenges.Find(bson.M{"week": edit}).One(&ch)
                        ch.Name = r.FormValue("name")
                        ch.Description = r.FormValue("description")
                        ch.Week = week
                        challenges.Update(bson.M{"week": edit}, ch)
                } else {
                        // otherwise insert a new entry into collection
                        challenges.Insert(challenge{
                                Week:        week,
                                Name:        r.FormValue("name"),
                                Public:      false,
                                Description: r.FormValue("description"),
                        })
                }

                http.Redirect(w, r, htmlRoot+"/admin?success", http.StatusFound)
                return
        }

        // if they're not submitting as a form, show regular admin page
        var data struct {
                Admin       bool
                LoggedIn    bool
                Andrew      string
                Root        string
                Page        string
                Submissions []struct {
                        Andrew string
                        Done   bool
                }
                Challenges []challenge
                IsEditing  bool
                Edit       challenge
                Active     bool
        }
        data.Admin = true
        data.LoggedIn = true
        data.Andrew = session.Values["andrew"].(string)
        data.Root = htmlRoot
        data.Page = "admin"
        data.Active = chActive
        data.IsEditing = false

        // if we're running a challenge, populate submissions list
        if chActive {
                dir, err := ioutil.ReadDir("submissions")
                if err != nil {
                        panic(err)
                }

                rx, _ := regexp.Compile(`[^\.]+`)

                // scan through submissions directory
                for _, stat := range dir {

                        // extract andrew ID from submission name (i.e. ignore extension)
                        matches := rx.FindStringSubmatch(stat.Name())
                        andrew := matches[0]

                        var ch challenge
                        var sub struct {
                                Andrew string
                                Done   bool
                        }

                        sub.Andrew = andrew
                        sub.Done = false
                        challenges.Find(bson.M{"week": curChallenge.Week}).One(&ch)

                        // determine from db entry for challenge if they've submitted already
                        for _, entry := range ch.Scores {
                                if entry.Andrew == andrew {
                                        sub.Done = true
                                        break
                                }
                        }

                        data.Submissions = append(data.Submissions, sub)
                }
        }

        challenges.Find(nil).Sort("-week").All(&data.Challenges)

        // if they're in the process of editing a challenge, mark form as such
        weekStr := r.URL.Query().Get("edit")
        if weekStr != "" {
                week, _ := strconv.Atoi(weekStr)
                challenges.Find(bson.M{"week": week}).One(&data.Edit)
                data.IsEditing = true
        }

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

        // authenticate user
        if session.Values["logged_in"] != "yes" || !isAdmin(session.Values["andrew"].(string)) {
                http.Redirect(w, r, htmlRoot+"/", http.StatusFound)
                return
        }

        // get andrew ID from GET
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
