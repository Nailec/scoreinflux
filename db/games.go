package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Game struct {
	ID            string              `bson:"_id"`
	Competition   string              `bson:"competition"`
	Home          []string            `bson:"home"`
	Away          []string            `bson:"away"`
	Goals         []Goal              `bson:"goals"`
	Substitutions []Substitution      `bson:"subs"`
	Date          primitive.Timestamp `bson:"date"`
}

type Goal struct {
	Home bool
	Min  int
}

type Substitution struct {
	Home bool
	Min  int
	In   string
	Out  string
}

var ColGames string = "games"

func (c *MongoDBClient) InsertGameInfo(competition, url string, home, away []string,
	goals []Goal, subs []Substitution) error {

	_, err := c.Client.Database(c.Database).Collection(ColGames).InsertOne(
		context.TODO(),
		&Game{
			ID:            url,
			Competition:   competition,
			Home:          home,
			Away:          away,
			Goals:         goals,
			Substitutions: subs,
		},
	)

	return err
}

// returns true if already exists
func (c *MongoDBClient) GameExists(id string) (bool, error) {
	//  ctx, cancel := context.WithTimeout(context.Background(), ctxDeadline)
	//  defer cancel()

	err := c.Client.Database(c.Database).Collection(ColGames).FindOne(
		context.TODO(),
		bson.M{"_id": id},
	).Err()

	return err != mongo.ErrNoDocuments, err
}
