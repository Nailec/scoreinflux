package db

import (
	"context"
	"sort"

	"github.com/cannona/choose"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/uuid"
)

type CombinValue struct {
	// ID generated by mongo
	ID uuid.UUID `bson:"_id"`
	// Number of players in combin
	Size int `bson:"size"`
	// Players part of a combination we store the value for
	Players []string `bson:"players"`
	// Cumulated time played together for this combination
	Minutes int `bson:"minutes,omitempty"`
	// Cumulated score for when this combination played together
	CumulatedScore float64 `bson:"cumulated_score,omitempty"`
	// Averaged score per 90 minutes for this combination
	Score float64 `bson:"score,omitempty"`
	// Number of combinations a player as been part of
	CombinCount int `bson:"combin_count,omitempty"` // Only if len(Players) = 1
	// Actual Shapley Value of a player
	// It is actually only an estimation as not all the combinations are counted
	ShapleyValue float64 `bson:"shapley_value,omitempty"` // Only if len(Players) = 1
}

var ColCombin string = "combinations"

func (c *MongoDBClient) GetSingletonsForPlayers(ids []string) ([]*CombinValue, error) {
	cursor, err := c.Client.Database(c.Database).Collection(ColCombin).Find(
		context.TODO(),
		bson.M{"size": 1, "players": bson.M{"$in": ids}},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var vals []*CombinValue
	err = cursor.All(context.TODO(), &vals)
	return vals, err
}

func (c *MongoDBClient) GetPlayersFromCombins() ([]*CombinValue, error) {
	cursor, err := c.Client.Database(c.Database).Collection(ColCombin).Find(
		context.TODO(),
		bson.M{"size": 1},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var vals []*CombinValue
	err = cursor.All(context.TODO(), &vals)
	return vals, err
}

func (c *MongoDBClient) GetPairsWithPlayer(id string) ([]*CombinValue, error) {
	cursor, err := c.Client.Database(c.Database).Collection(ColCombin).Find(context.TODO(),
		bson.M{"size": 2, "players": id})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get pairs for "+id)
	}

	var all []*CombinValue
	if cursor.All(context.TODO(), &all) != nil {
		return nil, errors.Wrap(err, "cursor error")
	}

	return all, nil
}

func (c *MongoDBClient) ComputePlayerRelativeContrib(player string, combin []*CombinValue) (float64, error) {
	scores := map[string]float64{}
	minutes := map[string]int{}
	totTime := 0

	players := make([]string, 0, len(combin))
	for _, x := range combin {
		comp := x.Players[0]
		if x.Players[0] == player {
			comp = x.Players[1]
		}
		players = append(players, comp)

		scores[comp] = x.Score
		minutes[comp] = x.Minutes
		totTime += x.Minutes
	}

	cursor, err := c.Client.Database(c.Database).Collection(ColCombin).Find(context.TODO(),
		bson.M{"size": 1, "players": bson.M{"$in": players}})
	if err != nil {
		return 0.0, errors.Wrapf(err, "cannot get complements for %s", player)
	}

	var complements []*CombinValue
	err = cursor.All(context.TODO(), &complements)
	if err != nil {
		return 0.0, err
	}

	totScores := 0.0
	for _, complement := range complements {
		totScores += (scores[complement.Players[0]] - complement.Score) * float64(minutes[complement.Players[0]])
	}

	return totScores / float64(totTime), nil
}

func (c *MongoDBClient) UpdateScores(home []string, away []string, min int, score float64) error {
	homePairs := genPairs(home)
	awayPairs := genPairs(away)
	var err error

	for _, h := range home {
		err = c.InsertOrUpdateCombin([]string{h}, min, score, true)
		if err != nil {
			return errors.Wrap(err, "error with home player score update")
		}
	}

	for _, a := range away {
		err = c.InsertOrUpdateCombin([]string{a}, min, score, false)
		if err != nil {
			return errors.Wrap(err, "error with away player score update")
		}
	}

	for _, hp := range homePairs {
		err = c.InsertOrUpdateCombin(hp, min, score, true)
		if err != nil {
			return errors.Wrap(err, "error with home players score update")
		}
	}

	for _, ap := range awayPairs {
		err = c.InsertOrUpdateCombin(ap, min, score, false)
		if err != nil {
			return errors.Wrap(err, "error with away players score update")
		}
	}

	return nil
}

func (c *MongoDBClient) InsertOrUpdateCombin(players []string, min int, score float64, home bool) error {
	if !home {
		score = -score
	}

	id, err := uuid.New()
	if err != nil {
		return err
	}

	_, err = c.Client.Database(c.Database).Collection(ColCombin).UpdateOne(
		context.TODO(),
		bson.M{
			"players": players,
			"size":    len(players),
		},
		[]bson.M{
			{
				"$set": bson.M{
					"_id": bson.M{
						"$cond": []interface{}{bson.M{
							"$eq": []interface{}{bson.M{"$type": "$_id"}, "missing"}},
							id,
							"$_id",
						},
					},
					"players": players,
					"size":    len(players),
					"minutes": bson.M{
						"$add": []interface{}{bson.M{
							"$cond": []interface{}{bson.M{
								"$eq": []interface{}{bson.M{"$type": "$minutes"}, "missing"}},
								0,
								"$minutes",
							}},
							min,
						},
					},
					"cumulated_score": bson.M{
						"$add": []interface{}{bson.M{
							"$cond": []interface{}{bson.M{
								"$eq": []interface{}{bson.M{"$type": "$cumulated_score"}, "missing"}},
								0,
								"$cumulated_score",
							}},
							score,
						},
					},
				},
			},
			{
				"$set": bson.M{
					"score": bson.M{"$divide": []interface{}{"$cumulated_score", "$minutes"}},
				},
			},
		},
		options.Update().SetUpsert(true),
	)

	return err
}

func genPairs(elems []string) [][]string {
	size := choose.Choose(int64(len(elems)), 2)
	res := make([][]string, 0, size)
	sort.Strings(elems)
	for i, elem1 := range elems {
		for _, elem2 := range elems[i+1:] {
			res = append(res, []string{elem1, elem2})
		}
	}

	return res
}
