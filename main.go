package main

import (
	"context"
	"fmt"
	"scoreinflux/db"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mdb string = "xstats"

func main() {
	//	ctx, cancel := context.WithTimeout(context.Background(), ctxDeadline)
	//	defer cancel()
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println("cannot connect to mongodb", err)
		return
	}

	c := &db.MongoDBClient{
		Client:   client,
		Database: mdb,
	}

	players, err := c.GetPlayersFromCombins()
	if err != nil {
		fmt.Println("cannot get players", err)
		return
	}

	var all []*db.CombinValue
	var contrib float64
	for _, player := range players {
		all, err = c.GetPairsWithPlayer(player.Players[0])
		if err != nil {
			fmt.Println("cannot get pairs for "+player.Players[0], err)
			return
		}

		contrib, err = c.ComputePlayerRelativeContrib(player.Players[0], all)
		if err != nil {
			fmt.Println(errors.Wrap(err, "cannot compute contrib"))
			return
		}

		shap := (player.Score + contrib) * 20000.0

		//_, err = client.Database("xstats").Collection("ratings").UpdateByID(
		err = c.UpsertRating(player.Players[0], shap, primitive.Timestamp{T: uint32(time.Now().Unix())})
		if err != nil {
			fmt.Println("update error", err)
			return
		}
	}
}
