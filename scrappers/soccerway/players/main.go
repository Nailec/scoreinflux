package main

import (
	"context"
	"fmt"
	"scoreinflux/db"
	"time"

	"github.com/go-rod/rod"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var urlPrefix string = "https://uk.soccerway.com"
var mdb string = "xstats"

func main() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println("cannot connect to mongodb", err)
		return
	}

	c := db.MongoDBClient{
		Client:   client,
		Database: mdb,
	}

	browser := rod.New().MustConnect()
	defer browser.MustClose()

	ids, err := c.GetMissingPlayerInfos()
	if err != nil {
		fmt.Println(err)
		return
	}

	maxGoroutines := 6
	limit := make(chan struct{}, maxGoroutines)

	for _, id := range ids {
		limit <- struct{}{}
		go func(id string) {
			fmt.Println(urlPrefix + id)
			pi := &db.PlayerInfo{ID: id}

			page := browser.MustPage(urlPrefix + id)
			content := page.MustElement(".block_player_passport")

			if !content.MustHas(`dd[data-date_of_birth="date_of_birth"]`) {
				<-limit
				return
			}

			pi.FirstName = content.MustElement(`dd[data-first_name="first_name"]`).MustText()
			pi.LastName = content.MustElement(`dd[data-last_name="last_name"]`).MustText()
			pi.Nationality = content.MustElement(`dd[data-nationality="nationality"]`).MustText()
			dob := content.MustElement(`dd[data-date_of_birth="date_of_birth"]`).MustText()
			page.Close()

			t, err := time.Parse("2 January 2006", dob)
			if err != nil {
				fmt.Println(err)
				<-limit
				return
			}

			pi.DateOfBirth = t.Format("2006-01-02")

			err = c.InsertPlayerInfo(pi)
			if err != nil {
				fmt.Println(err)
				<-limit
				return
			}

			<-limit
		}(id)
	}
}
