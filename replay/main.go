package main

import (
	"context"
	"fmt"
	"scoreinflux/db"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mdb string = "xstats3"

func main() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println("cannot connect to mongodb", err)
		return
	}

	c := &db.MongoDBClient{
		Client:   client,
		Database: mdb,
	}

	sort := options.Find().SetSort(bson.M{"date": 1})
	cursor, err := client.Database(mdb).Collection("games").Find(context.TODO(), bson.M{},
		sort,
	)
	if err != nil {
		fmt.Println("cannot get games", err)
		return
	}
	defer cursor.Close(context.TODO())

	var games []*db.Game
	err = cursor.All(context.TODO(), &games)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, game := range games {
		fmt.Println(game.ID)
		allPlayerIDs := map[string]interface{}{}
		for _, id := range game.Home {
			allPlayerIDs[id] = nil
		}
		for _, id := range game.Away {
			allPlayerIDs[id] = nil
		}

		i := 0
		lastSub := 0
		totTime := 91
		for _, sub := range game.Substitutions {
			allPlayerIDs[sub.In] = nil
			min := sub.Min - lastSub
			predScore, err := getPredicted(c, game.Home, game.Away, min)
			if err != nil {
				fmt.Println(err)
				return
			}

			score := -predScore

			for i < len(game.Goals) && game.Goals[i].Min <= sub.Min { // debate if <
				if game.Goals[i].Home {
					score++
				} else {
					score--
				}
				i++
			}

			if min > 0 {
				err := c.UpdateScores(game.Home, game.Away, min, float64(score))
				if err != nil {
					fmt.Println(errors.Wrap(err, "cannot update score"))
					return
				}
				lastSub = sub.Min
				if sub.Min >= totTime {
					totTime = sub.Min + 1
				}
			}
			if sub.Home {
				for j, p := range game.Home {
					if p == sub.Out {
						game.Home[j] = sub.In
					}
				}
			} else {
				for j, p := range game.Away {
					if p == sub.Out {
						game.Away[j] = sub.In
					}
				}
			}
		}

		score := 0.0
		for i < len(game.Goals) { // debate if <
			if game.Goals[i].Min >= totTime {
				totTime = game.Goals[i].Min + 1
			}

			if game.Goals[i].Home {
				score++
			} else {
				score--
			}
			i++
		}

		min := totTime - lastSub
		predScore, err := getPredicted(c, game.Home, game.Away, min)
		if err != nil {
			fmt.Println(err)
			return
		}

		score -= predScore

		err = c.UpdateScores(game.Home, game.Away, min, float64(score))
		if err != nil {
			fmt.Println(errors.Wrap(err, "cannot update score end"))
			return
		}

		ids := make([]string, 0, len(allPlayerIDs))
		for id := range allPlayerIDs {
			ids = append(ids, id)
		}

		// update players current ratings
		playerCombins, err := c.GetSingletonsForPlayers(ids)
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, combin := range playerCombins {
			pairs, err := c.GetPairsWithPlayer(combin.Players[0])
			if err != nil {
				fmt.Println(err)
				return
			}

			contrib, err := c.ComputePlayerRelativeContrib(combin.Players[0], pairs)
			if err != nil {
				fmt.Println(err)
				return
			}

			rating := (combin.Score + contrib) * 90.0 * 100.0 / 2.0

			err = c.UpsertRating(combin.Players[0], rating, game.Date)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}

}

func getPredicted(c *db.MongoDBClient, home, away []string, min int) (float64, error) {
	if min == 0 {
		return 0, nil
	}

	homeRating, err := c.GetRatingsForPlayers(home)
	if err != nil {
		return 0.0, err
	}

	awayRating, err := c.GetRatingsForPlayers(away)
	if err != nil {
		return 0.0, err
	}

	predict := 0.0
	for _, combin := range homeRating {
		predict += combin.Rating
	}

	for _, combin := range awayRating {
		predict -= combin.Rating
	}

	return predict * float64(min) / (float64(12 * 100 * 90)), nil
}
