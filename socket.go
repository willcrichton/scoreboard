package main

import (
	"easyws"
	"encoding/json"
	"labix.org/v2/mgo/bson"
	"net/http"
	"os"
	"strconv"
	"time"
)

var timer int64

// packets are just a basic key/value pair encoded w/ json
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
	// decode json string
	var result struct{ Key, Value string }
	err := json.Unmarshal([]byte(msg), &result)
	if err != nil {
		c.Send("bad message")
		return
	}
	switch result.Key {
	case "release":
		// admin has notified to release another challenge
		if !isAdmin(connID[c]) {
			break
		}
		chActive = true
		var opts struct {
			Week string
			Time string
		}
		err = json.Unmarshal([]byte(result.Value), &opts)
		week, _ := strconv.Atoi(opts.Week)
		end, _ := strconv.Atoi(opts.Time)

		// update the record in Mongo to reflect the release
		challenges.Update(bson.M{"week": week}, bson.M{"$set": bson.M{"public": true}})
		challenges.Find(bson.M{"week": week}).One(&curChallenge)

		// tell everyone which challenged has been released
		ws.Broadcast(packet("release", result.Value))

		// start the round timer
		timer = time.Now().Unix()
		go func() {
			time.Sleep(time.Duration(end) * time.Minute)
			chActive = false
			ws.Broadcast(packet("end", ""))
			os.RemoveAll("submissions")
			os.Mkdir("submissions", os.ModePerm|os.ModeDir)
		}()
	case "approve":
		if !isAdmin(connID[c]) {
			break
		}

		// update record in challenge entry
		id := result.Value
		timeDiff := int(time.Now().Unix() - timer)
		for conn, andrew := range connID {
			if andrew == id {
				conn.Send(packet("approved", "thumbs up"))
			}
		}

		// find current challenge
		var cur challenge
		filter := bson.M{"week": curChallenge.Week}
		challenges.Find(filter).One(&cur)

		// calculate score and update their record in the challenge
		pts := 15 - len(cur.Scores)
		if pts < 5 {
			pts = 5
		}
		cur.Scores = append(cur.Scores, score{Andrew: id, Score: pts, Place: 1 + len(cur.Scores), Time: timeDiff})
		challenges.Update(filter, cur)

		// update student's personal record
		var u student
		filter = bson.M{"andrew": id}
		students.Find(filter).One(&u)
		u.Points += pts
		students.Update(filter, u)

		// broadcast approval to system
		if pts > 5 {
			str, _ := json.Marshal(score{Andrew: id, Place: len(cur.Scores), Time: timeDiff})
			ws.Broadcast(packet("place", string(str)))
		}
	case "reject":
		// Get the rejected student and rejection reason
		var rejectInfo struct{ Andrew, Message string }
		err := json.Unmarshal([]byte(result.Value), &rejectInfo)
		if err != nil {
			break
		}
		if !isAdmin(connID[c]) {
			break
		}

		// Find which connection corresponds to the given andrew
		id := rejectInfo.Andrew
		for conn, andrew := range connID {
			if andrew == id {
				conn.Send(packet("rejected", rejectInfo.Message))
			}
		}

		// delete their submission from the submissions directory
		os.Remove("submissions/" + getSubmission(id))
	}
}

func wsOnJoin(r *http.Request, c *easyws.Connection, h *easyws.Hub) {
	// associate a connection object w/ an andrew id
	session, err := store.Get(r, sessName)
	if err != nil || session.Values["andrew"] == nil {
		return
	}

	connID[c] = session.Values["andrew"].(string)
}

func wsOnLeave(r *http.Request, c *easyws.Connection, h *easyws.Hub) {
	delete(connID, c)
}
