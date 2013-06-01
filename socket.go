package main

import (
	"encoding/json"
	"github.com/willcrichton/easyws"
	"labix.org/v2/mgo/bson"
	"strconv"
)

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
	}
}

func wsOnJoin(c *easyws.Connection, h *easyws.Hub) {
	// associate a connection object w/ an andrew id
	session, err := store.Get(r, sessName)
	if err != nil {
		w.Write([]byte("bad cookies"))
		return
	}

	connID[c] = session.Values["andrew"]
}
