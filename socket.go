package main

import (
	"encoding/json"
	"easyws"
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
		week, _ := strconv.Atoi(result.Value)
		challenges.Find(bson.M{"week": week}).One(&curChallenge)
		curChallenge.Public = true
		challenges.Update(bson.M{"week": week}, curChallenge)
		ws.Broadcast(packet("release", result.Value))
		timer = time.Now().Unix()
	case "approve":
		if !isAdmin(connID[c]) {
			break
		}
		
		id := result.Value
		for conn, andrew := range connID {
			if andrew == id {
				conn.Send(packet("approved", "thumbs up"))
			}
		}
		var cur challenge
		filter := bson.M{"week": curChallenge.Week}
		challenges.Find(filter).One(&cur)
		pts := 10 - len(cur.Scores)
		if pts < 0 {
			pts = 0
		}
		cur.Scores = append(cur.Scores, score{Andrew: id, Score: pts, Time: int(time.Now().Unix() - timer)})
		challenges.Update(filter, cur)

		var u student
		filter = bson.M{"andrew": id}
		students.Find(filter).One(&u)
		u.Points += pts
		students.Update(filter, u)
	case "reject":
		var rejectInfo struct { Andrew, Message string }
		err := json.Unmarshal([]byte(result.Value), &rejectInfo)
		if err != nil {
			break
		}
		if !isAdmin(connID[c]) {
			break
		}
		
		id := rejectInfo.Andrew
		for conn, andrew := range connID {
			if andrew == id {
				conn.Send(packet("rejected", rejectInfo.Message))
			}
		}
			
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