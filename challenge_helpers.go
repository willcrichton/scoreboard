/**************************************************
 * challenge_helpers.go
 * Helper functions to run the challenges
 **************************************************/

package main

import (
	"encoding/json"
	"io/ioutil"
	"labix.org/v2/mgo/bson"
	"os"
	"time"
)

func startChallenge(week, end int) {

	// update the record in Mongo to reflect the release
	challenges.Update(bson.M{"week": week}, bson.M{"$set": bson.M{"public": true}})
	challenges.Find(bson.M{"week": week}).One(&curChallenge)

	// move hidden assets to public directory
	dir, err := ioutil.ReadDir("hidden")
	if err != nil {
		panic(err)
	}

	for _, file := range dir {
		os.Rename("hidden/"+file.Name(), tmplPath+"/assets/"+file.Name())
	}

	// tell everyone which challenged has been released
	ws.Broadcast(packet("release", string(week)))

	// start the round timer
	timer = time.Now().Unix()
	go func() {
		time.Sleep(time.Duration(end) * time.Minute)
		chActive = false
		ws.Broadcast(packet("end", ""))
		os.RemoveAll("submissions")
		os.Mkdir("submissions", 0755)
	}()
}

func approveSubmission(id string) {

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
}

func rejectSubmission(id, reason string) {

	// Find which connection corresponds to the given andrew
	for conn, andrew := range connID {
		if andrew == id {
			conn.Send(packet("rejected", reason))
		}
	}

	// delete their submission from the submissions directory
	os.Remove("submissions/" + getSubmission(id))
}
