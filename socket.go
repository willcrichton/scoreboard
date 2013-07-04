/**************************************************
 * socket.go
 * Does logic for WebSocket
 **************************************************/

package main

import (
        "easyws"
        "encoding/json"
        "net/http"
        "strconv"
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

                // update chActive global
                chActive = true

                // extract challenge data from packet
                var opts struct {
                        Week string
                        Time string
                }
                err = json.Unmarshal([]byte(result.Value), &opts)
                week, _ := strconv.Atoi(opts.Week)
                end, _ := strconv.Atoi(opts.Time)

                startChallenge(week, end)

        case "approve":
                if !isAdmin(connID[c]) {
                        break
                }

                approveSubmission(result.Value)

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

                rejectSubmission(rejectInfo.Andrew, rejectInfo.Message)
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
